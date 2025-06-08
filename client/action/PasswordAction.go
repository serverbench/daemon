package action

import (
	"supervisor/client/proto"
	"supervisor/containers"
)

/*
virtually the same as a regular empty action
*/
type PasswordAction struct {
	Id        string               `json:"id"`
	Type      string               `json:"type"`
	Container containers.Container `json:"container"`
}

func (a *PasswordAction) Process() (reply *proto.Msg, err error) {
	password, err := a.Container.ResetPassword()
	if err != nil {
		return nil, err
	}
	msg := proto.Msg{
		Action: Password,
		Params: map[string]interface{}{
			Password: password,
		},
	}
	return &msg, err
}
