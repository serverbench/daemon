package containers

import (
	"fmt"
	pass "github.com/sethvargo/go-password/password"
	log "github.com/sirupsen/logrus"
	"os/exec"
	"path/filepath"
	"strings"
)

const group = "serverbench"

func (c Container) homeDir() string {
	return "/users/" + c.Id
}

func (c Container) dataDir() string {
	return filepath.Join(c.homeDir(), "data")
}

// creates users, prepares the necessary directories and mounts them
func (c Container) createUser() (err error) {
	log.Info("creating user")
	err = exec.Command("useradd", "-m", "-d", c.homeDir(), "-G", group, "--shell", "/bin/false", c.Id).Run()
	if err != nil {
		return err
	}
	log.Info("resetting password")
	_, err = c.ResetPassword()
	if err != nil {
		return err
	}
	log.Info("jailing user (chown)")
	err = exec.Command("chown", "root:root", c.homeDir()).Run()
	if err != nil {
		return err
	}
	log.Info("jailing user (chmod)")
	err = exec.Command("chmod", "755", c.homeDir()).Run()
	if err != nil {
		return err
	}
	log.Info("adding user data root")
	err = exec.Command("mkdir", "-p", c.dataDir()).Run()
	if err != nil {
		return err
	}
	log.Info("creating container data directory")
	err = exec.Command("mkdir", "-p", c.Dir()).Run()
	if err != nil {
		return err
	}
	log.Info("chown-ing data directory")
	err = exec.Command("chown", "-R", c.Id+":"+group, c.Dir()).Run()
	if err != nil {
		return err
	}
	// TODO implement quotas
	return c.MountDir()
}

func (c Container) MountDir() error {
	log.Info("mounting data dir")
	return exec.Command("mount", "--bind", c.Dir(), c.dataDir()).Run()
}

func (c Container) Unmount() error {
	log.Info("unmounting data dir")
	return exec.Command("umount", c.dataDir()).Run()
}

// deletes the user and their data, the container should be disposed beforehand
func (c Container) deleteUser() (err error) {
	err = c.Unmount()
	if err != nil {
		return err
	}
	log.Info("removing container directory")
	err = exec.Command("rm", "-rf", c.Dir()).Run()
	if err != nil {
		return err
	}
	log.Info("deleting user")
	return exec.Command("deluser", "--remove-home", c.Id).Run()
}

func (c Container) ResetPassword() (string, error) {
	password, err := pass.Generate(32, 10, 0, false, false)
	if err != nil {
		return "", fmt.Errorf("failed to generate password: %w", err)
	}

	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", c.Id, password))

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to reset password: %w", err)
	}

	return password, nil
}
