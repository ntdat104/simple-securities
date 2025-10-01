package dto

type NotificationDto struct {
	ID        uint64 `json:"id"`
	Uuid      string `json:"uuid"`
	UserID    uint64 `json:"user_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Read      bool   `json:"read"`
	ReadAt    uint64 `json:"read_at"`
	Viewed    bool   `json:"viewed"`
	ViewedAt  uint64 `json:"viewed_at"`
	CreatedAt uint64 `json:"created_at"`
	UpdatedAt uint64 `json:"updated_at"`
}

type NotificationCreateReq struct {
	UserID uint64 `json:"user_id" validate:"required"`
	Type   string `json:"type" validate:"required"`
	Title  string `json:"title" validate:"required"`
	Body   string `json:"body" validate:"required"`
}

type NotificationUpdateReq struct {
	ID     uint64 `json:"id" validate:"required"`
	UserID uint64 `json:"user_id" validate:"required"`
	Type   string `json:"type" validate:"required"`
	Title  string `json:"title" validate:"required"`
	Body   string `json:"body" validate:"required"`
	Read   bool   `json:"read"`
	Viewed bool   `json:"viewed"`
}
