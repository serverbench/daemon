package machine

type Session struct {
	Id      string  `json:"id"`
	Machine Machine `json:"machine"`
}
