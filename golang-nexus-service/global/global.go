/*
 * Global package for shared state across servers
 * Following Nutanix gRPC Gateway pattern
 */

package global

import (
	"sync"

	generated "github.com/nutanix/ntnx-api-golang-nexus-pc/generated-code/dto/models/nexus/v4/config"
)

// Server ports and hosts
const (
	APIServerPort  = "9009" // API Handler Server
	TaskServerPort = "9010" // Task Server (for polling)
	APIServerHost  = "localhost"
	TaskServerHost = "localhost"
)

// Task status constants
const (
	TaskStatusPending    = "PENDING"
	TaskStatusInProgress = "IN_PROGRESS"
	TaskStatusCompleted  = "COMPLETED"
	TaskStatusFailed     = "FAILED"
)

// TaskStore holds all tasks (in-memory for demo)
// In production, this would be a database
var (
	TaskList  = make(map[string]*Task)
	TaskMutex sync.RWMutex
)

// ItemStore holds all items using auto-generated DTOs (in-memory for demo)
var (
	ItemsDTO  = make(map[int]*generated.Item)
	ItemMutex sync.RWMutex
	Tasks    = make(map[string]*Task) // For backwards compatibility
	Mutex    = &sync.Mutex{}          // For backwards compatibility
)

// Task represents an asynchronous operation
type Task struct {
	ObjectType_        *string                `json:"$objectType,omitempty"`
	Reserved_          map[string]interface{} `json:"$reserved,omitempty"`
	UnknownFields_     map[string]interface{} `json:"$unknownFields,omitempty"`
	TaskId             string                 `json:"taskId"`
	PercentageComplete int                    `json:"percentageComplete"`
	Status             string                 `json:"status"`
	Message            string                 `json:"message,omitempty"`
	ObjectReturned     interface{}            `json:"objectReturned,omitempty"`
}

// NewTask creates a new task with default values
func NewTask(taskId string) *Task {
	return &Task{
		ObjectType_: stringPtr("nexus.v4.config.Task"),
		Reserved_: map[string]interface{}{
			"$fv": "v4.r1",
		},
		UnknownFields_:     map[string]interface{}{},
		TaskId:             taskId,
		PercentageComplete: 0,
		Status:             TaskStatusPending,
	}
}

// AddTask adds a task to the global store (thread-safe)
func AddTask(task *Task) {
	TaskMutex.Lock()
	defer TaskMutex.Unlock()
	TaskList[task.TaskId] = task
}

// GetTask retrieves a task from the global store (thread-safe)
func GetTask(taskId string) (*Task, bool) {
	TaskMutex.RLock()
	defer TaskMutex.RUnlock()
	task, exists := TaskList[taskId]
	return task, exists
}

// UpdateTask updates a task in the global store (thread-safe)
func UpdateTask(task *Task) {
	TaskMutex.Lock()
	defer TaskMutex.Unlock()
	TaskList[task.TaskId] = task
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
