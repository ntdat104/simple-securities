package repo

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

	"github.com/jmoiron/sqlx"
)

type BaseRepo[T any] struct {
	db *sqlx.DB
}

func NewBaseRepo[T any](db *sqlx.DB) *BaseRepo[T] {
	return &BaseRepo[T]{db: db}
}

// withTransaction runs fn inside a transaction with proper commit/rollback handling
func (r *BaseRepo[T]) WithTransaction(ctx context.Context, fn func(*sqlx.Tx) error) (err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	// rollback/commit handler
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // rethrow panic after rollback
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// FindOne is now generic. It accepts a specific instance of M to scan into.
func (r *BaseRepo[T]) FindOne(ctx context.Context, query string, args ...any) (*T, error) {
	var model T
	if err := r.db.GetContext(ctx, &model, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Indicate not found
		}
		return nil, fmt.Errorf("findOne failed for model %T: %w", model, err)
	}
	return &model, nil
}

// FindMany is now generic, returning a slice of the model type M.
func (r *BaseRepo[T]) FindMany(ctx context.Context, query string, value ...any) ([]*T, error) {
	if len(value) == 0 {
		return []*T{}, nil
	}

	query, args, err := sqlx.In(query, value)
	if err != nil {
		return nil, fmt.Errorf("FindMany failed: generating IN clause: %w", err)
	}

	query = r.db.Rebind(query)

	// The destination must be a pointer to a slice of pointers to M
	models := make([]*T, 0)
	if err := r.db.SelectContext(ctx, &models, query, args...); err != nil {
		return nil, fmt.Errorf("FindMany failed: executing query: %w", err)
	}

	return models, nil
}

func (r *BaseRepo[T]) Save(ctx context.Context, model *T, insertQuery string, updateQuery string) (*T, error) {
	val := reflect.ValueOf(model).Elem()
	idField := val.FieldByName("ID")

	if !idField.IsValid() {
		return nil, fmt.Errorf("Save failed: model %T must have an 'ID' field", model)
	}

	err := r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		if idField.Uint() == 0 {
			// INSERT
			stmt, err := tx.PrepareNamedContext(ctx, insertQuery)
			if err != nil {
				return err
			}
			defer stmt.Close()
			if err := stmt.GetContext(ctx, &idField, model); err != nil {
				return err
			}
		} else {
			// UPDATE
			res, err := tx.NamedExecContext(ctx, updateQuery, model)
			if err != nil {
				return err
			}
			rows, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if rows == 0 {
				return fmt.Errorf("update failed: model with id %d not found", idField.Uint())
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return model, nil
}

// SaveAll processes a slice of models in a single transaction.
func (r *BaseRepo[T]) SaveAll(ctx context.Context, models []*T, insertQuery string, updateQuery string) ([]*T, error) {
	if len(models) == 0 {
		return []*T{}, nil
	}

	err := r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		for _, model := range models {
			val := reflect.ValueOf(model).Elem()
			idField := val.FieldByName("ID")

			if !idField.IsValid() {
				return fmt.Errorf("SaveAll failed: model %T must have an 'ID' field", model)
			}

			if idField.Uint() == 0 {
				stmt, err := tx.PrepareNamedContext(ctx, insertQuery)
				if err != nil {
					return err
				}
				defer stmt.Close()
				if err := stmt.GetContext(ctx, &idField, model); err != nil {
					return err
				}
			} else {
				res, err := tx.NamedExecContext(ctx, updateQuery, model)
				if err != nil {
					return err
				}
				rows, err := res.RowsAffected()
				if err != nil {
					return err
				}
				if rows == 0 {
					return fmt.Errorf("bulk update failed: id %d not found", idField.Uint())
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return models, nil
}
