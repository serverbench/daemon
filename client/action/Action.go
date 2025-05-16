package action

import (
	"encoding/json"
	"errors"
	"github.com/docker/docker/client"
	"supervisor/containers"
)

const Management = "management"
const Password = "password"
const Power = "power"

type Action struct {
	Id        string               `json:"id"`
	Type      string               `json:"type"`
	Container containers.Container `json:"container"`
	Ref       json.RawMessage
}

func (a *Action) Process(cli *client.Client) (err error) {
	switch a.Type {
	case Management:
		{
			management := ManagementAction{}
			err = json.Unmarshal(a.Ref, &management)
			if err != nil {
				return err
			}
			return management.Process(cli)
		}
	case Power:
		{
			power := PowerAction{}
			err = json.Unmarshal(a.Ref, &power)
			if err != nil {
				return err
			}
			return power.Process(cli)
		}
	case Password:
		{
			password := PasswordAction{}
			err = json.Unmarshal(a.Ref, &password)
			if err != nil {
				return err
			}
			return password.Process()
		}
	default:
		return errors.New("invalid action type")
	}
}
