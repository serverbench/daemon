package action

import (
	"errors"
	"github.com/docker/docker/client"
	"supervisor/containers"
)

const Create = "create"
const Update = "update"
const Delete = "delete"

type ManagementAction struct {
	Id        string               `json:"id"`
	Type      string               `json:"type"`
	Container containers.Container `json:"container"`
	Action    string               `json:"action"`
	State     containers.Container `json:"state"`
}

func (a *ManagementAction) Process(cli *client.Client) error {
	ports := []containers.Port{}
	switch a.Action {
	case Create:
		{
			return a.Container.Create(
				cli,
				ports,
			)
		}
	case Update:
		{
			return a.Container.Update(
				cli,
				ports,
			)
		}
	case Delete:
		{
			return a.Container.Destroy(
				cli,
			)
		}
	default:
		{
			return errors.New("invalid management action type")
		}
	}
}
