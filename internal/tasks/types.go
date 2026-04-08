package tasks

import "time"

// TaskType represents supported task execution modes.
type TaskType string

const (
	TaskTypeLocalBash  TaskType = "local_bash"
	TaskTypeLocalAgent TaskType = "local_agent"
)

// TaskStatus represents current lifecycle state for a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusKilled    TaskStatus = "killed"
)

// Record stores task metadata for listing and lookup.
type Record struct {
	ID          string
	Type        TaskType
	Status      TaskStatus
	Description string
	CWD         string
	Command     string
	Prompt      string
	CreatedAt   time.Time
	Metadata    map[string]string
}
