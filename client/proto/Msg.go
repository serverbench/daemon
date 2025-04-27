package proto

type Msg struct {
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params"`
	Rid    string                 `json:"rid"`
}
