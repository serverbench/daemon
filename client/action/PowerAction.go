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
	switch a.Power {
	case Start:
		{
			return a.Container.Start(cli)
		}
	case Stop:
		{
			return a.Container.Stop(cli)
		}
	case Restart:
		{
			return a.Container.Restart(cli)
		}
	case Pause:
		{
			return a.Container.Pause(cli)
		}
	case Unpause:
		{
			return a.Container.Unpause(cli)
		}
	case Kill:
		{
			return a.Container.Kill(cli)
		}
	default:
		{
			return errors.New("unknown power action type")
		}
	}
	return nil
}
