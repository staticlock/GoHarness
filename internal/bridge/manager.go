package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/user/goharness/internal/config"
)

// SessionSnapshot is a UI-safe bridge session payload.
type SessionSnapshot struct {
	SessionID  string `json:"session_id"`
	Command    string `json:"command"`
	CWD        string `json:"cwd"`
	PID        int    `json:"pid"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"started_at"`
	OutputPath string `json:"output_path"`
}

// Manager stores bridge sessions in memory.
type Manager struct {
	mu        sync.RWMutex
	sessions  map[string]SessionSnapshot
	processes map[string]*exec.Cmd
}

// NewManager builds an empty bridge session manager.
func NewManager() *Manager {
	return &Manager{sessions: map[string]SessionSnapshot{}, processes: map[string]*exec.Cmd{}}
}

// Spawn starts a bridge shell command session and tracks its status.
func (m *Manager) Spawn(command, cwd string) (SessionSnapshot, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return SessionSnapshot{}, fmt.Errorf("command is required")
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = "."
	}
	outputPath, err := bridgeOutputPath()
	if err != nil {
		return SessionSnapshot{}, err
	}
	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return SessionSnapshot{}, err
	}

	cmd := shellCommand(command)
	cmd.Dir = cwd
	cmd.Stdout = file
	cmd.Stderr = file
	if err := cmd.Start(); err != nil {
		_ = file.Close()
		return SessionSnapshot{}, err
	}

	snapshot := SessionSnapshot{
		SessionID:  randomSessionID(),
		Command:    command,
		CWD:        cwd,
		PID:        cmd.Process.Pid,
		Status:     "running",
		StartedAt:  time.Now().Unix(),
		OutputPath: outputPath,
	}
	m.mu.Lock()
	m.sessions[snapshot.SessionID] = snapshot
	m.processes[snapshot.SessionID] = cmd
	m.mu.Unlock()

	go m.watch(snapshot.SessionID, cmd, file)
	return snapshot, nil
}

// ReadOutput returns the tail of one bridge session output file.
func (m *Manager) ReadOutput(sessionID string) (string, error) {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("No bridge session found with ID: %s", sessionID)
	}
	if strings.TrimSpace(session.OutputPath) == "" {
		return "", nil
	}
	data, err := os.ReadFile(session.OutputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	const maxBytes = 12000
	if len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
	}
	return string(data), nil
}

// Stop stops one running bridge session.
func (m *Manager) Stop(sessionID string) error {
	m.mu.Lock()
	cmd, ok := m.processes[sessionID]
	if !ok {
		if session, exists := m.sessions[sessionID]; exists && session.Status != "running" {
			m.mu.Unlock()
			return nil
		}
		m.mu.Unlock()
		return fmt.Errorf("No bridge session found with ID: %s", sessionID)
	}
	session := m.sessions[sessionID]
	session.Status = "killed"
	m.sessions[sessionID] = session
	delete(m.processes, sessionID)
	m.mu.Unlock()
	if cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Kill()
	return nil
}

// Upsert stores or updates a bridge session snapshot.
func (m *Manager) Upsert(item SessionSnapshot) {
	m.mu.Lock()
	m.sessions[item.SessionID] = item
	m.mu.Unlock()
}

// Remove deletes one bridge session snapshot.
func (m *Manager) Remove(sessionID string) {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}

// ListSnapshots returns snapshots sorted by newest first.
func (m *Manager) ListSnapshots() []SessionSnapshot {
	m.mu.RLock()
	out := make([]SessionSnapshot, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	m.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	return out
}

var (
	defaultManager     *Manager
	defaultManagerOnce sync.Once
)

// DefaultManager returns the process-wide bridge manager singleton.
func DefaultManager() *Manager {
	defaultManagerOnce.Do(func() {
		defaultManager = NewManager()
	})
	return defaultManager
}

func (m *Manager) watch(sessionID string, cmd *exec.Cmd, outputFile *os.File) {
	err := cmd.Wait()
	_ = outputFile.Close()

	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[sessionID]
	if !ok {
		return
	}
	if session.Status == "killed" {
		return
	}
	if err != nil {
		session.Status = "failed"
	} else {
		session.Status = "completed"
	}
	m.sessions[sessionID] = session
	delete(m.processes, sessionID)
}

func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("/bin/bash", "-lc", command)
}

func bridgeOutputPath() (string, error) {
	dataDir, err := config.DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(dataDir, "bridge")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, randomSessionID()+".log"), nil
}

func randomSessionID() string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return "bridge-" + hex.EncodeToString(buf)
}
