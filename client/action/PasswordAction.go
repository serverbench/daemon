package action

import (
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

func (a *PasswordAction) Process() error {
	_, err := a.Container.ResetPassword()
	if err != nil {
		return err
	}
	return nil
}
