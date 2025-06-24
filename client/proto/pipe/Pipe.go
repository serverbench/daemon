package pipe

import "context"

type Event string

const (
	EventLog      Event = "log"
	EventStatus   Event = "status"
	EventPassword Event = "password"
)

type GenericFilter struct {
	Container string
}

type Pipe struct {
	Ended   bool
	Delete  chan struct{}
	Cancel  context.CancelFunc
	Context context.Context
	Forward chan Forward
	Lid     string
	Event   Event
	Filter  interface{}
}
type BasicPipe struct {
	Lid    string      `json:"lid"`
	Event  Event       `json:"event"`
	Filter interface{} `json:"filter"`
}

func (p *Pipe) End() {
	if p.Ended {
		return
	}
	p.Ended = true
	p.Cancel()
	p.Forward <- Forward{
		Event: p.Event,
		Lid:   p.Lid,
		Data:  nil,
		End:   true,
	}
	p.Delete <- struct{}{}
}

func (p *Pipe) Package(data interface{}) Forward {
	return Forward{
		Event: p.Event,
		Lid:   p.Lid,
		Data:  data,
		End:   false,
	}
}
