package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"todo-service/internal/db"
	"todo-service/internal/model"
)

// TodoHandler handles HTTP requests for TODO operations.
type TodoHandler struct {
	repo   *db.Repository
	logger *slog.Logger
}

// NewTodoHandler creates a new TodoHandler.
func NewTodoHandler(repo *db.Repository, logger *slog.Logger) *TodoHandler {
	return &TodoHandler{repo: repo, logger: logger}
}

// --- Input/Output types for huma ---

type ListTodosInput struct {
	Status   string `query:"status" required:"false" enum:"pending,in_progress,done" doc:"Filter by status"`
	Category string `query:"category" required:"false" enum:"personal,work,other" doc:"Filter by category"`
}

type ListTodosOutput struct {
	Body model.TodoListResponse
}

type CreateTodoInput struct {
	Body model.CreateTodoRequest
}

type CreateTodoOutput struct {
	Body model.Todo
}

type GetTodoInput struct {
	ID int64 `path:"id" doc:"TODO ID" example:"1"`
}

type GetTodoOutput struct {
	Body model.Todo
}

type UpdateTodoInput struct {
	ID   int64 `path:"id" doc:"TODO ID" example:"1"`
	Body model.UpdateTodoRequest
}

type UpdateTodoOutput struct {
	Body model.Todo
}

type DeleteTodoInput struct {
	ID int64 `path:"id" doc:"TODO ID" example:"1"`
}

// RegisterRoutes registers all TODO routes with the huma API.
func (h *TodoHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-todos",
		Method:      http.MethodGet,
		Path:        "/api/v1/todos",
		Summary:     "List all TODOs",
		Description: "Retrieve all TODO items, optionally filtered by status and/or category.",
		Tags:        []string{"todos"},
	}, h.ListTodos)

	huma.Register(api, huma.Operation{
		OperationID:   "create-todo",
		Method:        http.MethodPost,
		Path:          "/api/v1/todos",
		Summary:       "Create a new TODO",
		Description:   "Create a new TODO item with optional progress tracking.",
		Tags:          []string{"todos"},
		DefaultStatus: http.StatusCreated,
	}, h.CreateTodo)

	huma.Register(api, huma.Operation{
		OperationID: "get-todo",
		Method:      http.MethodGet,
		Path:        "/api/v1/todos/{id}",
		Summary:     "Get a TODO by ID",
		Description: "Retrieve a single TODO item by its ID.",
		Tags:        []string{"todos"},
	}, h.GetTodo)

	huma.Register(api, huma.Operation{
		OperationID: "update-todo",
		Method:      http.MethodPut,
		Path:        "/api/v1/todos/{id}",
		Summary:     "Update a TODO",
		Description: "Update an existing TODO item. Only provided fields are changed.",
		Tags:        []string{"todos"},
	}, h.UpdateTodo)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-todo",
		Method:        http.MethodDelete,
		Path:          "/api/v1/todos/{id}",
		Summary:       "Delete a TODO",
		Description:   "Delete a TODO item by its ID.",
		Tags:          []string{"todos"},
		DefaultStatus: http.StatusNoContent,
	}, h.DeleteTodo)
}

func (h *TodoHandler) ListTodos(ctx context.Context, input *ListTodosInput) (*ListTodosOutput, error) {
	var statusFilter *model.Status
	if input.Status != "" {
		s := model.Status(input.Status)
		statusFilter = &s
	}

	var categoryFilter *model.Category
	if input.Category != "" {
		c := model.Category(input.Category)
		categoryFilter = &c
	}

	todos, err := h.repo.ListTodos(statusFilter, categoryFilter)
	if err != nil {
		h.logger.Error("failed to list todos", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to retrieve todos")
	}

	return &ListTodosOutput{
		Body: model.TodoListResponse{Todos: todos, Count: len(todos)},
	}, nil
}

func (h *TodoHandler) CreateTodo(ctx context.Context, input *CreateTodoInput) (*CreateTodoOutput, error) {
	if input.Body.Title == "" {
		return nil, huma.Error400BadRequest("title is required")
	}

	if input.Body.Status != "" && !model.ValidStatuses[input.Body.Status] {
		return nil, huma.Error400BadRequest("status must be one of: pending, in_progress, done")
	}

	if input.Body.Category != "" && !model.ValidCategories[input.Body.Category] {
		return nil, huma.Error400BadRequest("category must be one of: personal, work, other")
	}

	if input.Body.ProgressPercent != nil && (*input.Body.ProgressPercent < 0 || *input.Body.ProgressPercent > 100) {
		return nil, huma.Error400BadRequest("progress_percent must be between 0 and 100")
	}

	todo, err := h.repo.CreateTodo(input.Body)
	if err != nil {
		h.logger.Error("failed to create todo", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create todo")
	}

	return &CreateTodoOutput{Body: todo}, nil
}

func (h *TodoHandler) GetTodo(ctx context.Context, input *GetTodoInput) (*GetTodoOutput, error) {
	todo, err := h.repo.GetTodo(input.ID)
	if errors.Is(err, db.ErrNotFound) {
		return nil, huma.Error404NotFound(fmt.Sprintf("todo with id %d not found", input.ID))
	}
	if err != nil {
		h.logger.Error("failed to get todo", slog.String("error", err.Error()), slog.Int64("id", input.ID))
		return nil, huma.Error500InternalServerError("failed to retrieve todo")
	}

	return &GetTodoOutput{Body: todo}, nil
}

func (h *TodoHandler) UpdateTodo(ctx context.Context, input *UpdateTodoInput) (*UpdateTodoOutput, error) {
	if input.Body.Status != nil && !model.ValidStatuses[*input.Body.Status] {
		return nil, huma.Error400BadRequest("status must be one of: pending, in_progress, done")
	}

	if input.Body.Category != nil && !model.ValidCategories[*input.Body.Category] {
		return nil, huma.Error400BadRequest("category must be one of: personal, work, other")
	}

	if input.Body.ProgressPercent != nil && (*input.Body.ProgressPercent < 0 || *input.Body.ProgressPercent > 100) {
		return nil, huma.Error400BadRequest("progress_percent must be between 0 and 100")
	}

	todo, err := h.repo.UpdateTodo(input.ID, input.Body)
	if errors.Is(err, db.ErrNotFound) {
		return nil, huma.Error404NotFound(fmt.Sprintf("todo with id %d not found", input.ID))
	}
	if err != nil {
		h.logger.Error("failed to update todo", slog.String("error", err.Error()), slog.Int64("id", input.ID))
		return nil, huma.Error500InternalServerError("failed to update todo")
	}

	return &UpdateTodoOutput{Body: todo}, nil
}

func (h *TodoHandler) DeleteTodo(ctx context.Context, input *DeleteTodoInput) (*struct{}, error) {
	err := h.repo.DeleteTodo(input.ID)
	if errors.Is(err, db.ErrNotFound) {
		return nil, huma.Error404NotFound(fmt.Sprintf("todo with id %d not found", input.ID))
	}
	if err != nil {
		h.logger.Error("failed to delete todo", slog.String("error", err.Error()), slog.Int64("id", input.ID))
		return nil, huma.Error500InternalServerError("failed to delete todo")
	}

	return nil, nil
}
