package repo

import (
	"context"
	"database/sql"
	"fmt"
	"simple-securities/internal/notification/domain/model"
	"simple-securities/internal/notification/domain/repo"

	"github.com/jmoiron/sqlx"
	"golang.org/x/sync/singleflight"
)

type NotificationRepo struct {
	db *sqlx.DB
	sf singleflight.Group
}

func NewNotificationRepo(db *sqlx.DB) repo.INotificationRepo {
	return &NotificationRepo{db: db}
}

// Create builds a new notification entity and persists it
func (r *NotificationRepo) Create(ctx context.Context, noti *model.Notification) (*model.Notification, error) {
	n := model.NewNotification(noti.UserID, noti.Type, noti.Title, noti.Body)

	query := `
		INSERT INTO notifications (
			uuid, user_id, type, title, body,
			read, read_at, viewed, viewed_at,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			:uuid, :user_id, :type, :title, :body,
			:read, :read_at, :viewed, :viewed_at,
			:created_at, :updated_at, :created_by, :updated_by
		)
		RETURNING id
	`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	if err := stmt.GetContext(ctx, &n.ID, n); err != nil {
		return nil, err
	}

	return n, nil
}

// Delete removes a notification by ID
func (r *NotificationRepo) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM notifications WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// Update modifies an existing notification
func (r *NotificationRepo) Update(ctx context.Context, n *model.Notification) error {
	query := `
		UPDATE notifications
		SET 
			user_id   = :user_id,
			type      = :type,
			title     = :title,
			body      = :body,
			read      = :read,
			read_at   = :read_at,
			viewed    = :viewed,
			viewed_at = :viewed_at,
			updated_at = :updated_at,
			updated_by = :updated_by
		WHERE id = :id
	`
	_, err := r.db.NamedExecContext(ctx, query, n)
	return err
}

// GetByID fetches a notification by ID (with singleflight)
func (r *NotificationRepo) GetByID(ctx context.Context, id uint64) (*model.Notification, error) {
	key := fmt.Sprintf("notification:id:%d", id)

	v, err, _ := r.sf.Do(key, func() (interface{}, error) {
		query := `
			SELECT 
				id, uuid, user_id, type, title, body,
				read, read_at, viewed, viewed_at,
				created_at, updated_at, created_by, updated_by
			FROM notifications
			WHERE id = $1
		`

		var n model.Notification
		err := r.db.GetContext(ctx, &n, query, id)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		return &n, nil
	})

	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}

	return v.(*model.Notification), nil
}

// GetByUserId fetches notifications by User ID with pagination (with singleflight)
func (r *NotificationRepo) GetByUserId(ctx context.Context, userId uint64, limit, offset uint32) ([]*model.Notification, error) {
	key := fmt.Sprintf("notification:user:%d:limit:%d:offset:%d", userId, limit, offset)

	v, err, _ := r.sf.Do(key, func() (interface{}, error) {
		query := `
			SELECT 
				id, uuid, user_id, type, title, body,
				read, read_at, viewed, viewed_at,
				created_at, updated_at, created_by, updated_by
			FROM notifications
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`

		var notifications []*model.Notification
		err := r.db.SelectContext(ctx, &notifications, query, userId, limit, offset)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		return notifications, nil
	})

	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}

	return v.([]*model.Notification), nil
}
