package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"simple-securities/domain/model"
	"simple-securities/domain/repo"

	"github.com/jmoiron/sqlx"
)

type ExampleRepo struct {
	db *sqlx.DB
}

func NewExampleRepo(db *sqlx.DB) repo.IExampleRepo {
	return &ExampleRepo{db: db}
}

func (r *ExampleRepo) Create(ctx context.Context, example *model.Example) (*model.Example, error) {
	now := time.Now()
	example.CreatedAt = now
	example.UpdatedAt = now

	// For MySQL 8.x with RETURNING
	query := `
		INSERT INTO examples (name, alias, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query, example.Name, example.Alias, example.CreatedAt, example.UpdatedAt)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	example.Id = int(id)

	return example, nil
}

func (r *ExampleRepo) Update(ctx context.Context, entity *model.Example) error {
	entity.UpdatedAt = time.Now()

	query := `
		UPDATE examples
		SET name = ?, alias = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := r.db.ExecContext(ctx, query, entity.Name, entity.Alias, entity.UpdatedAt, entity.Id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *ExampleRepo) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM examples WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *ExampleRepo) GetByID(ctx context.Context, id int) (*model.Example, error) {
	var example model.Example
	query := `SELECT id, name, alias, created_at, updated_at FROM examples WHERE id = ?`

	err := r.db.GetContext(ctx, &example, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	return &example, nil
}

func (r *ExampleRepo) FindByName(ctx context.Context, name string) (*model.Example, error) {
	var example model.Example
	query := `SELECT id, name, alias, created_at, updated_at FROM examples WHERE name = ?`

	err := r.db.GetContext(ctx, &example, query, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	return &example, nil
}
