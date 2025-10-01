package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"simple-securities/internal/notification/domain/model"
	"simple-securities/internal/notification/domain/repo"

	"github.com/go-redis/redis/v8"
)

type NotificationCacheRepo struct {
	client *redis.Client
}

func NewNotificationCacheRepo(client *redis.Client) repo.INotificationCacheRepo {
	return &NotificationCacheRepo{client: client}
}

func notificationKey(id uint64) string {
	return fmt.Sprintf("notification:%d", id)
}

func userKey(userId uint64) string {
	return fmt.Sprintf("user_notifications:%d", userId)
}

// Set stores a notification in Redis
func (r *NotificationCacheRepo) Set(ctx context.Context, key uint64, notification *model.Notification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	pipe := r.client.TxPipeline()
	pipe.Set(ctx, notificationKey(key), data, 0)
	pipe.SAdd(ctx, userKey(notification.UserID), key)
	_, err = pipe.Exec(ctx)
	return err
}

// Get fetches a notification by ID
func (r *NotificationCacheRepo) Get(ctx context.Context, id uint64) (*model.Notification, error) {
	data, err := r.client.Get(ctx, notificationKey(id)).Bytes()
	if err == redis.Nil {
		return nil, nil // not found
	} else if err != nil {
		return nil, err
	}

	var n model.Notification
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

// Invalidate removes a notification by ID
func (r *NotificationCacheRepo) Invalidate(ctx context.Context, id uint64) error {
	// get notification first to know userID
	n, err := r.Get(ctx, id)
	if err != nil {
		return err
	}
	if n == nil {
		return nil
	}

	pipe := r.client.TxPipeline()
	pipe.Del(ctx, notificationKey(id))
	pipe.SRem(ctx, userKey(n.UserID), id)
	_, err = pipe.Exec(ctx)
	return err
}

// InvalidateByUserId removes all notifications for a given user
func (r *NotificationCacheRepo) InvalidateByUserId(ctx context.Context, userId uint64) error {
	ids, err := r.client.SMembers(ctx, userKey(userId)).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	pipe := r.client.TxPipeline()
	for _, idStr := range ids {
		pipe.Del(ctx, fmt.Sprintf("notification:%s", idStr))
	}
	pipe.Del(ctx, userKey(userId))
	_, err = pipe.Exec(ctx)
	return err
}

// InvalidateAll clears the whole cache
func (r *NotificationCacheRepo) InvalidateAll(ctx context.Context) error {
	// Warning: KEYS is O(N), not recommended for very large datasets
	keys, err := r.client.Keys(ctx, "notification:*").Result()
	if err != nil {
		return err
	}

	userKeys, err := r.client.Keys(ctx, "user_notifications:*").Result()
	if err != nil {
		return err
	}

	allKeys := append(keys, userKeys...)
	if len(allKeys) > 0 {
		if err := r.client.Del(ctx, allKeys...).Err(); err != nil {
			return err
		}
	}
	return nil
}
