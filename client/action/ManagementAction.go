package action

import (
	"errors"
	"github.com/docker/docker/client"
	"supervisor/containers"
)

const Update = "update"

type ManagementAction struct {
	Id        string               `json:"id"`
	Type      string               `json:"type"`
	Container containers.Container `json:"container"`
	Action    string               `json:"action"`
	State     containers.Container `json:"state"`
}

func (a *ManagementAction) Process(cli *client.Client) error {
	switch a.Action {
	case Update:
		{
			return a.State.Update(
				cli,
				false,
			)
		}
	default:
		{
			return errors.New("invalid management action type")
		}
	}
}
