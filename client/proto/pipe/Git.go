package pipe

type Git struct {
	Deployed bool `json:"deployed"`
}

type GitFilter struct {
	Container string `json:"container"`
	Uri       string `json:"uri"`
	Token     string `json:"token"`
	Branch    string `json:"branch"`
	Domain    string `json:"domain"`
	ResetData bool   `json:"resetData"`
}
