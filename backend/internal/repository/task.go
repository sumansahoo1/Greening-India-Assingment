package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sumansahoo/taskflow/backend/internal/model"
)

// taskSelectCols casts enum and date columns to text so pgx can scan them into Go strings.
const taskSelectCols = `
	t.id, t.title, t.description,
	t.status::text, t.priority::text,
	t.project_id::text, t.assignee_id::text, t.created_by::text,
	to_char(t.due_date, 'YYYY-MM-DD'),
	t.created_at, t.updated_at`

type TaskRepo struct {
	db *pgxpool.Pool
}

func NewTaskRepo(db *pgxpool.Pool) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) ListByProject(ctx context.Context, projectID, status, assigneeID string, page, limit int) ([]model.Task, int, error) {
	where := []string{"t.project_id = $1"}
	args := []any{projectID}
	argIdx := 2

	if status != "" {
		where = append(where, fmt.Sprintf("t.status = $%d::task_status", argIdx))
		args = append(args, status)
		argIdx++
	}
	if assigneeID != "" {
		where = append(where, fmt.Sprintf("t.assignee_id = $%d::uuid", argIdx))
		args = append(args, assigneeID)
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM tasks t WHERE %s", whereClause)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting tasks: %w", err)
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	query := fmt.Sprintf(
		`SELECT %s FROM tasks t WHERE %s ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d`,
		taskSelectCols, whereClause, argIdx, argIdx+1,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, total, rows.Err()
}

func (r *TaskRepo) GetByID(ctx context.Context, id string) (*model.Task, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM tasks t WHERE t.id = $1`,
		taskSelectCols,
	)
	row := r.db.QueryRow(ctx, query, id)
	t, err := scanTask(row)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TaskRepo) Create(ctx context.Context, title, description, status, priority, projectID string, assigneeID *string, createdBy string, dueDate *string) (*model.Task, error) {
	query := fmt.Sprintf(
		`INSERT INTO tasks (title, description, status, priority, project_id, assignee_id, created_by, due_date)
		 VALUES ($1, $2, $3::task_status, $4::task_priority, $5, $6, $7, $8)
		 RETURNING %s`,
		strings.ReplaceAll(taskSelectCols, "t.", ""),
	)
	row := r.db.QueryRow(ctx, query, title, description, status, priority, projectID, assigneeID, createdBy, dueDate)
	t, err := scanTask(row)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TaskRepo) Update(ctx context.Context, id string, fields map[string]any) (*model.Task, error) {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	for col, val := range fields {
		cast := ""
		switch col {
		case "status":
			cast = "::task_status"
		case "priority":
			cast = "::task_priority"
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d%s", col, argIdx, cast))
		args = append(args, val)
		argIdx++
	}
	setClauses = append(setClauses, "updated_at = now()")

	args = append(args, id)
	returningCols := strings.ReplaceAll(taskSelectCols, "t.", "")
	query := fmt.Sprintf(
		`UPDATE tasks SET %s WHERE id = $%d RETURNING %s`,
		strings.Join(setClauses, ", "), argIdx, returningCols,
	)

	row := r.db.QueryRow(ctx, query, args...)
	t, err := scanTask(row)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	return err
}

func (r *TaskRepo) StatsByProject(ctx context.Context, projectID string) (*model.ProjectStats, error) {
	stats := &model.ProjectStats{
		ByStatus:   make(map[string]int),
		ByAssignee: make(map[string]model.AssigneeStats),
	}

	rows, err := r.db.Query(ctx,
		`SELECT status::text, COUNT(*) FROM tasks WHERE project_id = $1 GROUP BY status`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats.ByStatus[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows2, err := r.db.Query(ctx,
		`SELECT COALESCE(u.id::text, 'unassigned'), COALESCE(u.name, 'Unassigned'), COUNT(*)
		 FROM tasks t
		 LEFT JOIN users u ON t.assignee_id = u.id
		 WHERE t.project_id = $1
		 GROUP BY u.id, u.name`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var id, name string
		var count int
		if err := rows2.Scan(&id, &name, &count); err != nil {
			return nil, err
		}
		stats.ByAssignee[id] = model.AssigneeStats{Name: name, Count: count}
	}
	return stats, rows2.Err()
}

// scanner is satisfied by both pgx.Row and pgx.Rows
type scanner interface {
	Scan(dest ...any) error
}

func scanTask(s scanner) (model.Task, error) {
	var t model.Task
	err := s.Scan(
		&t.ID, &t.Title, &t.Description,
		&t.Status, &t.Priority,
		&t.ProjectID, &t.AssigneeID, &t.CreatedBy,
		&t.DueDate,
		&t.CreatedAt, &t.UpdatedAt,
	)
	return t, err
}
