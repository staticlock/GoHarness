package services

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/user/goharness/internal/config"
	"github.com/user/goharness/internal/engine"
)

// SessionSnapshot is the persisted session payload.
type SessionSnapshot struct {
	SessionID    string                       `json:"session_id"`
	CWD          string                       `json:"cwd"`
	Model        string                       `json:"model"`
	SystemPrompt string                       `json:"system_prompt"`
	Messages     []engine.ConversationMessage `json:"messages"`
	Usage        engine.UsageSnapshot         `json:"usage"`
	CreatedAt    float64                      `json:"created_at"`
	Summary      string                       `json:"summary"`
	MessageCount int                          `json:"message_count"`
}

// SessionInfo is the list-friendly summary shape.
type SessionInfo struct {
	SessionID    string  `json:"session_id"`
	Summary      string  `json:"summary"`
	MessageCount int     `json:"message_count"`
	Model        string  `json:"model"`
	CreatedAt    float64 `json:"created_at"`
}

// ProjectSessionDir returns project-specific session directory.
func ProjectSessionDir(cwd string) (string, error) {
	sessionsDir, err := config.SessionsDir()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(cwd)
	if err != nil {
		resolved = cwd
	}
	digestRaw := sha1.Sum([]byte(resolved))
	digest := hex.EncodeToString(digestRaw[:])[:12]
	dir := filepath.Join(sessionsDir, filepath.Base(resolved)+"-"+digest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// SaveSessionSnapshot saves latest + session-id snapshot.
func SaveSessionSnapshot(cwd, model, systemPrompt, sessionID string, messages []engine.ConversationMessage, usage engine.UsageSnapshot) (string, error) {
	sessionDir, err := ProjectSessionDir(cwd)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = randomID(12)
	}
	now := float64(time.Now().UnixNano()) / float64(time.Second)
	summary := firstUserSummary(messages)
	snap := SessionSnapshot{
		SessionID:    sessionID,
		CWD:          absOrOriginal(cwd),
		Model:        model,
		SystemPrompt: systemPrompt,
		Messages:     messages,
		Usage:        usage,
		CreatedAt:    now,
		Summary:      summary,
		MessageCount: len(messages),
	}
	payload, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')
	latestPath := filepath.Join(sessionDir, "latest.json")
	if err := os.WriteFile(latestPath, payload, 0o644); err != nil {
		return "", err
	}
	sessionPath := filepath.Join(sessionDir, "session-"+sessionID+".json")
	if err := os.WriteFile(sessionPath, payload, 0o644); err != nil {
		return "", err
	}
	return latestPath, nil
}

// LoadLatestSession loads latest session snapshot for a project.
func LoadLatestSession(cwd string) (*SessionSnapshot, error) {
	sessionDir, err := ProjectSessionDir(cwd)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(sessionDir, "latest.json")
	return loadSnapshot(path)
}

// LoadSessionByID loads session by id.
func LoadSessionByID(cwd, sessionID string) (*SessionSnapshot, error) {
	sessionDir, err := ProjectSessionDir(cwd)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(sessionDir, "session-"+sessionID+".json")
	snap, err := loadSnapshot(path)
	if err == nil && snap != nil {
		return snap, nil
	}
	latestPath := filepath.Join(sessionDir, "latest.json")
	latest, latestErr := loadSnapshot(latestPath)
	if latestErr == nil && latest != nil && (latest.SessionID == sessionID || sessionID == "latest") {
		return latest, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, latestErr
}

// ListSessions returns most recent sessions for a project.
func ListSessions(cwd string, limit int) ([]SessionInfo, error) {
	sessionDir, err := ProjectSessionDir(cwd)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, err
	}
	infos := make([]SessionInfo, 0)
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "session-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		snap, err := loadSnapshot(filepath.Join(sessionDir, name))
		if err != nil || snap == nil {
			continue
		}
		seen[snap.SessionID] = true
		infos = append(infos, SessionInfo{SessionID: snap.SessionID, Summary: snap.Summary, MessageCount: snap.MessageCount, Model: snap.Model, CreatedAt: snap.CreatedAt})
	}
	latest, err := loadSnapshot(filepath.Join(sessionDir, "latest.json"))
	if err == nil && latest != nil && !seen[latest.SessionID] {
		s := latest.Summary
		if s == "" {
			s = "(latest session)"
		}
		infos = append(infos, SessionInfo{SessionID: latest.SessionID, Summary: s, MessageCount: latest.MessageCount, Model: latest.Model, CreatedAt: latest.CreatedAt})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].CreatedAt > infos[j].CreatedAt })
	if len(infos) > limit {
		infos = infos[:limit]
	}
	return infos, nil
}

func loadSnapshot(path string) (*SessionSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap SessionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func firstUserSummary(messages []engine.ConversationMessage) string {
	for _, msg := range messages {
		if msg.Role == "user" {
			trim := strings.TrimSpace(msg.Text)
			if trim != "" {
				if len(trim) > 80 {
					return trim[:80]
				}
				return trim
			}
		}
	}
	return ""
}

func randomID(n int) string {
	if n <= 0 {
		n = 12
	}
	now := time.Now().UnixNano()
	raw := sha1.Sum([]byte(strconv.FormatInt(now, 10)))
	s := hex.EncodeToString(raw[:])
	if len(s) < n {
		return s
	}
	return s[:n]
}

func absOrOriginal(path string) string {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return resolved
}
