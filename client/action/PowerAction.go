package action

import (
	"errors"
	"github.com/docker/docker/client"
	"supervisor/containers"
)

const Start = "start"
const Stop = "stop"
const Restart = "restart"
const Pause = "pause"
const Unpause = "unpause"
const Kill = "kill"

type PowerAction struct {
	Id        string               `json:"id"`
	Type      string               `json:"type"`
	Container containers.Container `json:"container"`
	Power     string               `json:"power"`
}

func (a *PowerAction) Process(cli *client.Client) error {
	switch a.Type {
	case Start:
		{

		}
	case Stop:
		{

		}
	case Restart:
		{

		}
	case Pause:
		{

		}
	case Unpause:
		{

		}
	case Kill:
		{

		}
	default:
		{
			return errors.New("unknown power action type")
		}
	}
	return nil
}
