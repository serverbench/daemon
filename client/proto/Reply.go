package proto

type Reply struct {
	Result *interface{} `json:"result"`
	Rid    string       `json:"rid"`
}
