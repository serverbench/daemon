package pipe

type Log struct {
	Timestamp int64  `json:"timestamp"`
	Content   string `json:"content"`
	End       bool   `json:"end"`
}

type LogFilter struct {
	Container string `json:"container"`
	Since     int64  `json:"since"`
	Until     int64  `json:"until"`
	Limit     int64  `json:"limit"`
}
