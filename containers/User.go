package containers

import (
	"bufio"
	"errors"
	"fmt"
	pass "github.com/sethvargo/go-password/password"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

const group = "serverbench"

func (c *Container) homeDir() string {
	return "/users/" + c.Id
}

func (c *Container) dataDir() string {
	return filepath.Join(c.homeDir(), "data")
}

// creates users, prepares the necessary directories and mounts them
func (c *Container) createUser() (err error) {
	exists, err := c.userExists()
	if err != nil {
		return err
	}
	if !exists {
		log.Info("creating user")
		out, err := exec.Command("useradd", "-m", "-d", c.homeDir(), "-G", group, "--shell", "/bin/false", c.Id).CombinedOutput()
		if err != nil {
			log.Error("error while creating user: ", err, string(out))
			return err
		}
		log.Info("resetting password")
		_, err = c.ResetPassword()
		if err != nil {
			log.Error("error while resetting password: ", err)
			return err
		}
	}
	err = c.ReadyFs()
	if err != nil {
		log.Error("error while preparing FS: ", err)
		return err
	}
	// TODO implement quotas
	err = c.MountDir()
	if err != nil {
		log.Error("error while mounting dir:", err)
		return err
	}
	return nil
}

func (c *Container) PermSnippet() (err error, snippet string) {
	u, err := user.Lookup(c.Id)
	if err != nil {
		return errors.New("error looking up user: " + err.Error()), ""
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		return errors.New("error looking up group: " + err.Error()), ""
	}

	uid := u.Uid
	gid := g.Gid

	return nil, uid + ":" + gid
}

func (c *Container) ReadyFs() (err error) {
	exists, err := c.userExists()
	if !exists {
		return c.createUser()
	}
	log.Info("ensuring user folder")
	err = exec.Command("mkdir", "-p", c.homeDir()).Run()
	if err != nil {
		log.Error("error while creating folder: ", err)
		return err
	}
	log.Info("jailing user (chown)")
	err = exec.Command("chown", "root:root", c.homeDir()).Run()
	if err != nil {
		log.Error("error while jailing user: ", err)
		return err
	}
	log.Info("jailing user (chmod)")
	err = exec.Command("chmod", "755", c.homeDir()).Run()
	if err != nil {
		log.Error("error while chmod 755: ", err)
		return err
	}
	log.Info("adding user data root")
	err = exec.Command("mkdir", "-p", c.dataDir()).Run()
	if err != nil {
		log.Error("error while creating data folder: ", err)
		return err
	}
	log.Info("creating container data directory")
	err = exec.Command("mkdir", "-p", c.Dir()).Run()
	if err != nil {
		log.Error("error while creating container folder: ", err)
		return err
	}
	log.Info("chown-ing data directory")
	err, perm := c.PermSnippet()
	if err != nil {
		log.Error("error while getting perm snippet: ", err)
		return err
	}
	err = exec.Command("chown", "-R", perm, c.Dir()).Run()
	if err != nil {
		log.Error("error while chowning to user: ", err)
		return err
	}
	log.Info("readied fs")
	return nil
}

func (c *Container) GetKeys() (keys []string, err error) {
	sshDir := filepath.Join(c.homeDir(), ".ssh")
	authKeysPath := filepath.Join(sshDir, "authorized_keys")

	// Ensure .ssh directory exists
	err = exec.Command("mkdir", "-p", sshDir).Run()
	if err != nil {
		return nil, fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Ensure authorized_keys file exists
	if _, err := os.Stat(authKeysPath); os.IsNotExist(err) {
		file, err := os.OpenFile(authKeysPath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to create authorized_keys: %w", err)
		}
		file.Close()
	}

	// Change ownership of .ssh directory and authorized_keys file
	log.Info("chown-ing .ssh directory and authorized_keys")
	err = exec.Command("chown", "-R", c.Id+":"+group, sshDir).Run()
	if err != nil {
		return nil, fmt.Errorf("failed to chown .ssh directory: %w", err)
	}

	// Open and read the authorized_keys file
	file, err := os.Open(authKeysPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open authorized_keys: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			keys = append(keys, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading authorized_keys: %w", err)
	}

	return keys, nil
}

func (c *Container) AddKey(pk string) (err error) {
	// Sanitize and trim input
	pk = strings.TrimSpace(pk)
	if pk == "" {
		return fmt.Errorf("empty public key")
	}

	// Get existing keys (also ensures .ssh and authorized_keys exist)
	existingKeys, err := c.GetKeys()
	if err != nil {
		return fmt.Errorf("failed to get existing keys: %w", err)
	}

	// Check if key is already present
	for _, k := range existingKeys {
		if k == pk {
			log.Info("key already exists, skipping")
			return nil
		}
	}

	// Basic validity check: public key should contain at least type and base64 data
	parts := strings.Fields(pk)
	if len(parts) < 2 {
		return fmt.Errorf("invalid public key format")
	}

	// Append the key to authorized_keys
	authKeysPath := filepath.Join(c.homeDir(), ".ssh", "authorized_keys")
	f, err := os.OpenFile(authKeysPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open authorized_keys for writing: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(pk + "\n")
	if err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	log.Info("added new public key to authorized_keys")
	return nil
}

func (c *Container) userExists() (exists bool, err error) {
	cmd := exec.Command("id", c.Id)
	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// If the exit code is non-zero, the user does not exist
			return false, nil
		}
		// An unexpected error occurred
		return false, err
	}
	// Command succeeded, so the user exists
	return true, nil
}

func (c *Container) MountDir() (err error) {
	log.Info("mounting data dir")
	exists, err := c.userExists()
	if err != nil {
		log.Error("error while checking if user exists: ", err)
		return err
	}
	if !exists {
		log.Info("user doesn't exist, creating one")
		err = c.createUser()
	} else {
		log.Info("user already exists, mounting")
		return exec.Command("mount", "--bind", c.Dir(), c.dataDir()).Run()
	}
	return err
}

func (c *Container) Unmount() error {
	log.Info("unmounting data dir")
	return exec.Command("umount", "-l", c.dataDir()).Run()
}

// deletes the user and their data, the container should be disposed beforehand
func (c *Container) deleteUser() (err error) {
	log.Info("removing container directory")
	err = exec.Command("rm", "-rf", c.Dir()).Run()
	if err != nil {
		return err
	}
	err = c.Unmount()
	if err != nil {
		return err
	}
	log.Info("deleting user")
	return exec.Command("deluser", "--remove-home", c.Id).Run()
}

func (c *Container) ResetPassword() (string, error) {
	password, err := pass.Generate(32, 10, 0, false, false)
	if err != nil {
		return "", fmt.Errorf("failed to generate password: %w", err)
	}

	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", c.Id, password))

	out, err := cmd.CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("failed to reset password: %w %s", err, string(out))
	}

	return password, nil
}
