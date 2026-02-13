package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"todo-service/internal/model"
)

var ErrNotFound = errors.New("not found")

// Repository provides CRUD operations for TODO items.
type Repository struct {
	db     *sql.DB
	logger *slog.Logger
}

// New opens a SQLite database and runs migrations.
func New(dbPath string, logger *slog.Logger) (*Repository, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite doesn't support concurrent writers

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	repo := &Repository{db: db, logger: logger}

	if err := repo.Migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	logger.Info("database initialized", slog.String("path", dbPath))
	return repo, nil
}

// Close closes the database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}

// Migrate creates the todos table if it doesn't exist.
func (r *Repository) Migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS todos (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		title            TEXT    NOT NULL,
		description      TEXT    NOT NULL DEFAULT '',
		status           TEXT    NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'in_progress', 'done')),
		progress_percent INTEGER NOT NULL DEFAULT 0 CHECK(progress_percent >= 0 AND progress_percent <= 100),
		created_at       DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at       DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status);
	`
	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	if err := r.addCategoryColumn(); err != nil {
		return fmt.Errorf("add category column: %w", err)
	}

	r.logger.Info("database migration complete")
	return nil
}

// addCategoryColumn adds the category column if it doesn't already exist.
func (r *Repository) addCategoryColumn() error {
	rows, err := r.db.Query("PRAGMA table_info(todos)")
	if err != nil {
		return fmt.Errorf("query table info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("scan table info: %w", err)
		}
		if name == "category" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table info: %w", err)
	}

	migration := `
	ALTER TABLE todos ADD COLUMN category TEXT NOT NULL DEFAULT 'personal' CHECK(category IN ('personal', 'work', 'other'));
	`
	if _, err := r.db.Exec(migration); err != nil {
		return fmt.Errorf("execute category migration: %w", err)
	}
	if _, err := r.db.Exec(`CREATE INDEX IF NOT EXISTS idx_todos_category ON todos(category)`); err != nil {
		return fmt.Errorf("create category index: %w", err)
	}

	r.logger.Info("added category column to todos table")
	return nil
}

// CreateTodo inserts a new TODO and returns it.
func (r *Repository) CreateTodo(req model.CreateTodoRequest) (model.Todo, error) {
	status := model.StatusPending
	if req.Status != "" {
		status = req.Status
	}
	category := model.CategoryPersonal
	if req.Category != "" {
		category = req.Category
	}
	progress := 0
	if req.ProgressPercent != nil {
		progress = *req.ProgressPercent
	}

	result, err := r.db.Exec(
		`INSERT INTO todos (title, description, status, category, progress_percent) VALUES (?, ?, ?, ?, ?)`,
		req.Title, req.Description, string(status), string(category), progress,
	)
	if err != nil {
		return model.Todo{}, fmt.Errorf("insert todo: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.Todo{}, fmt.Errorf("get last insert id: %w", err)
	}

	return r.GetTodo(id)
}

// GetTodo retrieves a single TODO by ID.
func (r *Repository) GetTodo(id int64) (model.Todo, error) {
	row := r.db.QueryRow(
		`SELECT id, title, description, status, category, progress_percent,
			strftime('%Y-%m-%dT%H:%M:%SZ', created_at),
			strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)
		FROM todos WHERE id = ?`,
		id,
	)
	return scanTodo(row)
}

// ListTodos retrieves all TODOs, optionally filtered by status and/or category.
func (r *Repository) ListTodos(status *model.Status, category *model.Category) ([]model.Todo, error) {
	query := `SELECT id, title, description, status, category, progress_percent,
		strftime('%Y-%m-%dT%H:%M:%SZ', created_at),
		strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)
	FROM todos`
	var conditions []string
	var args []any

	if status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, string(*status))
	}
	if category != nil {
		conditions = append(conditions, "category = ?")
		args = append(args, string(*category))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += ` ORDER BY id ASC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query todos: %w", err)
	}
	defer rows.Close()

	var todos []model.Todo
	for rows.Next() {
		var t model.Todo
		var statusStr, categoryStr string
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &statusStr, &categoryStr, &t.ProgressPercent, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan todo: %w", err)
		}
		t.Status = model.Status(statusStr)
		t.Category = model.Category(categoryStr)
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		todos = append(todos, t)
	}

	if todos == nil {
		todos = []model.Todo{}
	}

	return todos, rows.Err()
}

// UpdateTodo updates only the provided fields of a TODO.
func (r *Repository) UpdateTodo(id int64, req model.UpdateTodoRequest) (model.Todo, error) {
	var setClauses []string
	var args []any

	if req.Title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *req.Title)
	}
	if req.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *req.Description)
	}
	if req.Status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, string(*req.Status))
	}
	if req.Category != nil {
		setClauses = append(setClauses, "category = ?")
		args = append(args, string(*req.Category))
	}
	if req.ProgressPercent != nil {
		setClauses = append(setClauses, "progress_percent = ?")
		args = append(args, *req.ProgressPercent)
	}

	if len(setClauses) == 0 {
		return r.GetTodo(id)
	}

	setClauses = append(setClauses, "updated_at = datetime('now')")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE todos SET %s WHERE id = ?", strings.Join(setClauses, ", "))

	result, err := r.db.Exec(query, args...)
	if err != nil {
		return model.Todo{}, fmt.Errorf("update todo: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return model.Todo{}, fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return model.Todo{}, ErrNotFound
	}

	return r.GetTodo(id)
}

// DeleteTodo deletes a TODO by ID.
func (r *Repository) DeleteTodo(id int64) error {
	result, err := r.db.Exec(`DELETE FROM todos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

// scanTodo scans a single row into a Todo.
func scanTodo(row *sql.Row) (model.Todo, error) {
	var t model.Todo
	var statusStr, categoryStr string
	var createdAt, updatedAt string

	err := row.Scan(&t.ID, &t.Title, &t.Description, &statusStr, &categoryStr, &t.ProgressPercent, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Todo{}, ErrNotFound
	}
	if err != nil {
		return model.Todo{}, fmt.Errorf("scan todo: %w", err)
	}

	t.Status = model.Status(statusStr)
	t.Category = model.Category(categoryStr)
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return t, nil
}
