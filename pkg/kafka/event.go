package kafka

type Event struct {
	Meta Meta `json:"meta"`
	Data any  `json:"data"`
}

type Meta struct {
	RequestID   string   `json:"request_id"`
	Code        int      `json:"code"`
	Message     string   `json:"message"`
	Timestamp   int64    `json:"timestamp"`
	Datetime    string   `json:"datetime"`
	ServiceName string   `json:"service_name"`
	Errors      []*Error `json:"errors,omitempty"`
	PageIndex   *int     `json:"page_index,omitempty"`
	PageSize    *int     `json:"page_size,omitempty"`
	TotalItems  *int     `json:"total_items,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
