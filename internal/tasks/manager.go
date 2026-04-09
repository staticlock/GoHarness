package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager stores task records in memory.
type Manager struct {
	mu    sync.RWMutex
	tasks map[string]Record
}

// NewManager creates an empty task manager.
func NewManager() *Manager {
	return &Manager{tasks: map[string]Record{}}
}

// CreateShellTask registers a new local shell task record.
func (m *Manager) CreateShellTask(command, description, cwd string) Record {
	record := Record{
		ID:          taskID(TaskTypeLocalBash),
		Type:        TaskTypeLocalBash,
		Status:      TaskStatusRunning,
		Description: description,
		CWD:         cwd,
		Command:     command,
		CreatedAt:   time.Now(),
		Metadata:    map[string]string{},
	}
	m.mu.Lock()
	m.tasks[record.ID] = record
	m.mu.Unlock()
	return record
}

// CreateAgentTask registers a local-agent task record.
func (m *Manager) CreateAgentTask(prompt, description, cwd, model string) Record {
	record := Record{
		ID:          taskID(TaskTypeLocalAgent),
		Type:        TaskTypeLocalAgent,
		Status:      TaskStatusRunning,
		Description: description,
		CWD:         cwd,
		Prompt:      prompt,
		CreatedAt:   time.Now(),
		Metadata:    map[string]string{},
	}
	if strings.TrimSpace(model) != "" {
		record.Metadata["model"] = model
	}
	m.mu.Lock()
	m.tasks[record.ID] = record
	m.mu.Unlock()
	return record
}

// UpdateTaskRecord updates an entire task record.
func (m *Manager) UpdateTaskRecord(record Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks[record.ID] = record
}

// UpdateTaskStatus updates a task's status.
func (m *Manager) UpdateTaskStatus(taskID string, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("No task found with ID: %s", taskID)
	}
	record.Status = TaskStatus(status)
	m.tasks[taskID] = record
	return nil
}

// UpdateTaskOutput updates a task's output.
func (m *Manager) UpdateTaskOutput(taskID string, output string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("No task found with ID: %s", taskID)
	}
	record.Metadata["output"] = output
	m.tasks[taskID] = record
	return nil
}

// UpdateTaskMetadata sets a specific metadata key-value pair.
func (m *Manager) UpdateTaskMetadata(taskID string, key string, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("No task found with ID: %s", taskID)
	}
	record.Metadata[key] = value
	m.tasks[taskID] = record
	return nil
}

// UpdateTask updates task metadata fields used by command/status flows.
func (m *Manager) UpdateTask(taskID string, description *string, progress *int, statusNote *string) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.tasks[taskID]
	if !ok {
		return Record{}, fmt.Errorf("No task found with ID: %s", taskID)
	}
	if description != nil && *description != "" {
		record.Description = *description
	}
	if progress != nil {
		record.Metadata["progress"] = fmt.Sprintf("%d", *progress)
	}
	if statusNote != nil {
		if *statusNote == "" {
			delete(record.Metadata, "status_note")
		} else {
			record.Metadata["status_note"] = *statusNote
		}
	}
	m.tasks[taskID] = record
	return record, nil
}

// StopTask marks a running task as killed.
func (m *Manager) StopTask(taskID string) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.tasks[taskID]
	if !ok {
		return Record{}, fmt.Errorf("No task found with ID: %s", taskID)
	}
	if record.Status == TaskStatusCompleted || record.Status == TaskStatusFailed || record.Status == TaskStatusKilled {
		return record, nil
	}
	record.Status = TaskStatusKilled
	m.tasks[taskID] = record
	return record, nil
}

// ReadTaskOutput returns captured output text if present.
func (m *Manager) ReadTaskOutput(taskID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.tasks[taskID]
	if !ok {
		return "", fmt.Errorf("No task found with ID: %s", taskID)
	}
	if output, ok := record.Metadata["output"]; ok {
		return output, nil
	}
	return "", nil
}

// GetTask returns one task record by ID.
func (m *Manager) GetTask(taskID string) (Record, bool) {
	m.mu.RLock()
	record, ok := m.tasks[taskID]
	m.mu.RUnlock()
	return record, ok
}

// ListTasks returns all tasks sorted by creation time descending.
func (m *Manager) ListTasks(status string) []Record {
	m.mu.RLock()
	out := make([]Record, 0, len(m.tasks))
	for _, record := range m.tasks {
		if status != "" && string(record.Status) != status {
			continue
		}
		out = append(out, record)
	}
	m.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

var (
	defaultManager     *Manager
	defaultManagerOnce sync.Once
)

// DefaultManager returns the process-wide singleton manager.
func DefaultManager() *Manager {
	defaultManagerOnce.Do(func() {
		defaultManager = NewManager()
	})
	return defaultManager
}

// WriteToTask writes a message to a running task's stdin.
// Note: Currently returns error as stdin support requires more complex process management.
func (m *Manager) WriteToTask(taskID string, message string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("No task found with ID: %s", taskID)
	}
	if record.Status != "running" {
		return fmt.Errorf("Task %s is not running", taskID)
	}
	if record.Type != "local_agent" {
		return fmt.Errorf("Task %s does not accept input", taskID)
	}
	return fmt.Errorf("stdin writing not implemented - task runs in detached mode")
}

func taskID(taskType TaskType) string {
	prefix := "t"
	switch taskType {
	case TaskTypeLocalBash:
		prefix = "b"
	case TaskTypeLocalAgent:
		prefix = "a"
	}
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return prefix + hex.EncodeToString(buf)
}
