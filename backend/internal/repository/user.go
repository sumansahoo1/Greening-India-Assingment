package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sumansahoo/taskflow/backend/internal/model"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, name, email, passwordHash string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (name, email, password) VALUES ($1, $2, $3)
		 RETURNING id, name, email, password, created_at`,
		name, email, passwordHash,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Password, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, password, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Password, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, password, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Password, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) ListAll(ctx context.Context) ([]model.User, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, email, password, created_at FROM users ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) ListByProject(ctx context.Context, projectID string) ([]model.User, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT u.id, u.name, u.email, u.password, u.created_at
		 FROM users u
		 WHERE u.id IN (
		   SELECT owner_id FROM projects WHERE id = $1
		   UNION
		   SELECT assignee_id FROM tasks WHERE project_id = $1 AND assignee_id IS NOT NULL
		 )
		 ORDER BY u.name`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
