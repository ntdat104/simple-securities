package repo

import (
	"context"
	"errors"
	"simple-securities/internal/notification/domain/model"
	"simple-securities/internal/notification/domain/repo"
	"sync"
	"time"
)

type NotificationInmemRepo struct {
	data map[uint64]*model.Notification
	mu   sync.RWMutex
	auto uint64
}

func NewNotificationInmemRepo() repo.INotificationRepo {
	return &NotificationInmemRepo{
		data: make(map[uint64]*model.Notification),
		auto: 1,
	}
}

// Create builds a new notification and saves it in memory
func (r *NotificationInmemRepo) Create(ctx context.Context, noti *model.Notification) (*model.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := model.NewNotification(noti.UserID, noti.Type, noti.Title, noti.Body)
	n.ID = r.auto
	r.auto++
	n.CreatedAt = time.Now()
	n.UpdatedAt = n.CreatedAt

	r.data[n.ID] = n
	return n, nil
}

// Delete removes a notification by ID
func (r *NotificationInmemRepo) Delete(ctx context.Context, id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.data[id]; !ok {
		return errors.New("not found")
	}
	delete(r.data, id)
	return nil
}

// Update modifies an existing notification
func (r *NotificationInmemRepo) Update(ctx context.Context, n *model.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.data[n.ID]; !ok {
		return errors.New("not found")
	}
	n.UpdatedAt = time.Now()
	r.data[n.ID] = n
	return nil
}

// GetByID fetches a notification by ID
func (r *NotificationInmemRepo) GetByID(ctx context.Context, id uint64) (*model.Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if n, ok := r.data[id]; ok {
		return n, nil
	}
	return nil, nil
}

// GetByUserId fetches notifications by User ID with pagination
func (r *NotificationInmemRepo) GetByUserId(ctx context.Context, userId uint64, limit, offset uint32) ([]*model.Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var res []*model.Notification
	for _, n := range r.data {
		if n.UserID == userId {
			res = append(res, n)
		}
	}

	// order by CreatedAt DESC (naive sort)
	for i := 0; i < len(res)-1; i++ {
		for j := i + 1; j < len(res); j++ {
			if res[i].CreatedAt.Before(res[j].CreatedAt) {
				res[i], res[j] = res[j], res[i]
			}
		}
	}

	// pagination
	start := int(offset)
	if start > len(res) {
		return []*model.Notification{}, nil
	}
	end := start + int(limit)
	if end > len(res) {
		end = len(res)
	}

	return res[start:end], nil
}
