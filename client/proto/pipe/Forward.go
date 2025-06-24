package pipe

type Forward struct {
	Lid   string      `json:"lid"`
	Event Event       `json:"event"`
	Data  interface{} `json:"data"`
	End   bool        `json:"end"`
}
