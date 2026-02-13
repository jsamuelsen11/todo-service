package model

import "time"

// Status represents the state of a TODO item.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

// ValidStatuses contains all valid status values.
var ValidStatuses = map[Status]bool{
	StatusPending:    true,
	StatusInProgress: true,
	StatusDone:       true,
}

// Category represents the category of a TODO item.
type Category string

const (
	CategoryPersonal Category = "personal"
	CategoryWork     Category = "work"
	CategoryOther    Category = "other"
)

// ValidCategories contains all valid category values.
var ValidCategories = map[Category]bool{
	CategoryPersonal: true,
	CategoryWork:     true,
	CategoryOther:    true,
}

// Todo represents a TODO item with progress tracking.
type Todo struct {
	ID              int64     `json:"id" example:"1"`
	Title           string    `json:"title" example:"Buy groceries"`
	Description     string    `json:"description" example:"Milk, eggs, bread"`
	Status          Status    `json:"status" example:"pending" enums:"pending,in_progress,done"`
	Category        Category  `json:"category" example:"personal" enums:"personal,work,other"`
	ProgressPercent int       `json:"progress_percent" example:"0" minimum:"0" maximum:"100"`
	CreatedAt       time.Time `json:"created_at" example:"2026-02-12T15:04:05Z"`
	UpdatedAt       time.Time `json:"updated_at" example:"2026-02-12T15:04:05Z"`
}

// CreateTodoRequest is the payload for creating a new TODO.
type CreateTodoRequest struct {
	Title           string `json:"title" example:"Buy groceries"`
	Description     string `json:"description" example:"Milk, eggs, bread"`
	Status          Status   `json:"status,omitempty" example:"pending" enums:"pending,in_progress,done"`
	Category        Category `json:"category,omitempty" example:"personal" enums:"personal,work,other"`
	ProgressPercent *int     `json:"progress_percent,omitempty" example:"0" minimum:"0" maximum:"100"`
}

// UpdateTodoRequest is the payload for updating a TODO. All fields are optional.
type UpdateTodoRequest struct {
	Title           *string `json:"title,omitempty" example:"Buy groceries"`
	Description     *string `json:"description,omitempty" example:"Milk, eggs, bread, butter"`
	Status          *Status   `json:"status,omitempty" example:"in_progress" enums:"pending,in_progress,done"`
	Category        *Category `json:"category,omitempty" example:"work" enums:"personal,work,other"`
	ProgressPercent *int      `json:"progress_percent,omitempty" example:"50" minimum:"0" maximum:"100"`
}

// TodoListResponse wraps a list of todos.
type TodoListResponse struct {
	Todos []Todo `json:"todos"`
	Count int    `json:"count" example:"5"`
}

// ErrorResponse represents an API error.
type ErrorResponse struct {
	Error   string `json:"error" example:"not found"`
	Message string `json:"message,omitempty" example:"todo with id 42 not found"`
}
