package kafka

type SendMessage struct {
	m         *Manager
	topic     string
	key       string
	partition int
	headers   map[string]string
	event     Event
}

type Event struct {
	Meta Meta `json:"meta"`
	Data any  `json:"data"`
}

type Meta struct {
	ServiceName string   `json:"service_name"`
	RequestID   string   `json:"request_id"`
	Code        int      `json:"code"`
	Message     string   `json:"message"`
	Timestamp   int64    `json:"timestamp"`
	Datetime    string   `json:"datetime"`
	Errors      []*Error `json:"errors,omitempty"`
	PageIndex   *int     `json:"page_index,omitempty"`
	PageSize    *int     `json:"page_size,omitempty"`
	TotalItems  *int     `json:"total_items,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
