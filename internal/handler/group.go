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

// GroupHandler handles HTTP requests for group operations.
type GroupHandler struct {
	repo   *db.Repository
	logger *slog.Logger
}

// NewGroupHandler creates a new GroupHandler.
func NewGroupHandler(repo *db.Repository, logger *slog.Logger) *GroupHandler {
	return &GroupHandler{repo: repo, logger: logger}
}

// --- Input/Output types for huma ---

type ListGroupsInput struct{}

type ListGroupsOutput struct {
	Body model.GroupListResponse
}

type CreateGroupInput struct {
	Body model.CreateGroupRequest
}

type CreateGroupOutput struct {
	Body model.Group
}

type GetGroupInput struct {
	ID int64 `path:"id" doc:"Group ID" example:"1"`
}

type GetGroupOutput struct {
	Body model.Group
}

type UpdateGroupInput struct {
	ID   int64 `path:"id" doc:"Group ID" example:"1"`
	Body model.UpdateGroupRequest
}

type UpdateGroupOutput struct {
	Body model.Group
}

type DeleteGroupInput struct {
	ID int64 `path:"id" doc:"Group ID" example:"1"`
}

type GetGroupTodosInput struct {
	ID       int64  `path:"id" doc:"Group ID" example:"1"`
	Status   string `query:"status" required:"false" enum:"pending,in_progress,done" doc:"Filter by status"`
	Category string `query:"category" required:"false" enum:"personal,work,other" doc:"Filter by category"`
}


type GetGroupTodosOutput struct {
	Body model.TodoListResponse
}

// RegisterRoutes registers all group routes with the huma API.
func (h *GroupHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-groups",
		Method:      http.MethodGet,
		Path:        "/api/v1/groups",
		Summary:     "List all groups",
		Description: "Retrieve all TODO groups.",
		Tags:        []string{"groups"},
	}, h.ListGroups)

	huma.Register(api, huma.Operation{
		OperationID:   "create-group",
		Method:        http.MethodPost,
		Path:          "/api/v1/groups",
		Summary:       "Create a new group",
		Description:   "Create a new group for organizing TODOs.",
		Tags:          []string{"groups"},
		DefaultStatus: http.StatusCreated,
	}, h.CreateGroup)

	huma.Register(api, huma.Operation{
		OperationID: "get-group",
		Method:      http.MethodGet,
		Path:        "/api/v1/groups/{id}",
		Summary:     "Get a group by ID",
		Description: "Retrieve a single group by its ID.",
		Tags:        []string{"groups"},
	}, h.GetGroup)

	huma.Register(api, huma.Operation{
		OperationID: "update-group",
		Method:      http.MethodPut,
		Path:        "/api/v1/groups/{id}",
		Summary:     "Update a group",
		Description: "Update an existing group. Only provided fields are changed.",
		Tags:        []string{"groups"},
	}, h.UpdateGroup)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-group",
		Method:        http.MethodDelete,
		Path:          "/api/v1/groups/{id}",
		Summary:       "Delete a group",
		Description:   "Delete a group by its ID. TODOs in this group will be ungrouped.",
		Tags:          []string{"groups"},
		DefaultStatus: http.StatusNoContent,
	}, h.DeleteGroup)

	huma.Register(api, huma.Operation{
		OperationID: "get-group-todos",
		Method:      http.MethodGet,
		Path:        "/api/v1/groups/{id}/todos",
		Summary:     "Get TODOs in a group",
		Description: "Retrieve all TODO items belonging to a specific group, optionally filtered by status and/or category.",
		Tags:        []string{"groups"},
	}, h.GetGroupTodos)
}

func (h *GroupHandler) ListGroups(ctx context.Context, input *ListGroupsInput) (*ListGroupsOutput, error) {
	groups, err := h.repo.ListGroups()
	if err != nil {
		h.logger.Error("failed to list groups", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to retrieve groups")
	}

	return &ListGroupsOutput{
		Body: model.GroupListResponse{Groups: groups, Count: len(groups)},
	}, nil
}

func (h *GroupHandler) CreateGroup(ctx context.Context, input *CreateGroupInput) (*CreateGroupOutput, error) {
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("name is required")
	}

	group, err := h.repo.CreateGroup(input.Body)
	if err != nil {
		h.logger.Error("failed to create group", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to create group")
	}

	return &CreateGroupOutput{Body: group}, nil
}

func (h *GroupHandler) GetGroup(ctx context.Context, input *GetGroupInput) (*GetGroupOutput, error) {
	group, err := h.repo.GetGroup(input.ID)
	if errors.Is(err, db.ErrNotFound) {
		return nil, huma.Error404NotFound(fmt.Sprintf("group with id %d not found", input.ID))
	}
	if err != nil {
		h.logger.Error("failed to get group", slog.String("error", err.Error()), slog.Int64("id", input.ID))
		return nil, huma.Error500InternalServerError("failed to retrieve group")
	}

	return &GetGroupOutput{Body: group}, nil
}

func (h *GroupHandler) UpdateGroup(ctx context.Context, input *UpdateGroupInput) (*UpdateGroupOutput, error) {
	group, err := h.repo.UpdateGroup(input.ID, input.Body)
	if errors.Is(err, db.ErrNotFound) {
		return nil, huma.Error404NotFound(fmt.Sprintf("group with id %d not found", input.ID))
	}
	if err != nil {
		h.logger.Error("failed to update group", slog.String("error", err.Error()), slog.Int64("id", input.ID))
		return nil, huma.Error500InternalServerError("failed to update group")
	}

	return &UpdateGroupOutput{Body: group}, nil
}

func (h *GroupHandler) DeleteGroup(ctx context.Context, input *DeleteGroupInput) (*struct{}, error) {
	err := h.repo.DeleteGroup(input.ID)
	if errors.Is(err, db.ErrNotFound) {
		return nil, huma.Error404NotFound(fmt.Sprintf("group with id %d not found", input.ID))
	}
	if err != nil {
		h.logger.Error("failed to delete group", slog.String("error", err.Error()), slog.Int64("id", input.ID))
		return nil, huma.Error500InternalServerError("failed to delete group")
	}

	return nil, nil
}

func (h *GroupHandler) GetGroupTodos(ctx context.Context, input *GetGroupTodosInput) (*GetGroupTodosOutput, error) {
	// Verify group exists
	_, err := h.repo.GetGroup(input.ID)
	if errors.Is(err, db.ErrNotFound) {
		return nil, huma.Error404NotFound(fmt.Sprintf("group with id %d not found", input.ID))
	}
	if err != nil {
		h.logger.Error("failed to get group", slog.String("error", err.Error()), slog.Int64("id", input.ID))
		return nil, huma.Error500InternalServerError("failed to retrieve group")
	}

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

	todos, err := h.repo.ListTodos(statusFilter, categoryFilter, &input.ID)
	if err != nil {
		h.logger.Error("failed to list group todos", slog.String("error", err.Error()), slog.Int64("group_id", input.ID))
		return nil, huma.Error500InternalServerError("failed to retrieve todos")
	}

	return &GetGroupTodosOutput{
		Body: model.TodoListResponse{Todos: todos, Count: len(todos)},
	}, nil
}
