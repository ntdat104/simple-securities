package model

import (
	"simple-securities/pkg/uuid"
	"time"
)

type Notification struct {
	ID        uint64     `db:"id"`
	Uuid      string     `db:"uuid"`
	UserID    uint64     `db:"user_id"`
	Type      string     `db:"type"`
	Title     string     `db:"title"`
	Body      string     `db:"body"`
	Read      bool       `db:"read"`
	ReadAt    *time.Time `db:"read_at"`
	Viewed    bool       `db:"viewed"`
	ViewedAt  *time.Time `db:"viewed_at"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	CreatedBy uint64     `db:"created_by"`
	UpdatedBy uint64     `db:"updated_by"`
}

func (n Notification) TableName() string {
	return "notifications"
}

func NewNotification(userID uint64, nType, title, body string) *Notification {
	now := time.Now()
	return &Notification{
		Uuid:      uuid.NewGoogleUUID(),
		UserID:    userID,
		Type:      nType,
		Title:     title,
		Body:      body,
		Read:      false,
		ReadAt:    nil,
		Viewed:    false,
		ViewedAt:  nil,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
}
