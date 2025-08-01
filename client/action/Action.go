package action

import (
	"encoding/json"
	"errors"
	"github.com/docker/docker/client"
	"supervisor/client/proto"
	"supervisor/containers"
)

const Management = "management"
const Power = "power"

type Action struct {
	Id        string               `json:"id"`
	Type      string               `json:"type"`
	Container containers.Container `json:"container"`
	Ref       json.RawMessage
}

func (a *Action) Process(cli *client.Client) (msg *proto.Msg, err error) {
	switch a.Type {
	case Management:
		{
			management := ManagementAction{}
			err = json.Unmarshal(a.Ref, &management)
			if err != nil {
				return nil, err
			}
			return nil, management.Process(cli)
		}
	case Power:
		{
			power := PowerAction{}
			err = json.Unmarshal(a.Ref, &power)
			if err != nil {
				return nil, err
			}
			return nil, power.Process(cli)
		}
	default:
		return nil, errors.New("invalid action type")
	}
}
