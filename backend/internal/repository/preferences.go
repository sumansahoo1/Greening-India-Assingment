package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PreferencesRepo struct {
	db *pgxpool.Pool
}

func NewPreferencesRepo(db *pgxpool.Pool) *PreferencesRepo {
	return &PreferencesRepo{db: db}
}

type Preferences struct {
	ProjectsPageSize int
	TasksPageSize    int
}

func (r *PreferencesRepo) Get(ctx context.Context, userID string) (*Preferences, error) {
	var prefs Preferences
	_, _ = r.db.Exec(ctx,
		`INSERT INTO user_preferences (user_id)
		 VALUES ($1)
		 ON CONFLICT (user_id) DO NOTHING`,
		userID,
	)

	if err := r.db.QueryRow(
		ctx,
		`SELECT projects_page_size, tasks_page_size
		 FROM user_preferences
		 WHERE user_id = $1`,
		userID,
	).Scan(&prefs.ProjectsPageSize, &prefs.TasksPageSize); err != nil {
		return nil, err
	}
	return &prefs, nil
}

func (r *PreferencesRepo) Set(ctx context.Context, userID string, projectsPageSize, tasksPageSize *int) (*Preferences, error) {
	existing, err := r.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	nextProjects := existing.ProjectsPageSize
	nextTasks := existing.TasksPageSize
	if projectsPageSize != nil {
		nextProjects = *projectsPageSize
	}
	if tasksPageSize != nil {
		nextTasks = *tasksPageSize
	}

	var saved Preferences
	err = r.db.QueryRow(ctx,
		`INSERT INTO user_preferences (user_id, projects_page_size, tasks_page_size, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (user_id) DO UPDATE
		   SET projects_page_size = EXCLUDED.projects_page_size,
		       tasks_page_size = EXCLUDED.tasks_page_size,
		       updated_at = now()
		 RETURNING projects_page_size, tasks_page_size`,
		userID, nextProjects, nextTasks,
	).Scan(&saved.ProjectsPageSize, &saved.TasksPageSize)
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

