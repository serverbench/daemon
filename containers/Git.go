package containers

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/thanhpk/randstr"
)

func (c *Container) GetCommit() (commit *string, err error) {
	if c.Branch == nil {
		return nil, nil
	}
	isRepo, err := c.isGitRepository()
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if !isRepo {
		return nil, nil
	}
	err = c.Whitelist()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "-C", c.Dir(), "rev-parse", "HEAD")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	commitHash := strings.TrimSpace(out.String())
	return &commitHash, nil
}

func (c *Container) isGitRepository() (isRepo bool, err error) {
	dataPath := c.Dir()
	gitDir := path.Join(dataPath, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

func (c *Container) Whitelist() (err error) {
	dataPath := c.Dir()
	log.Info("whitelisting repo")
	err = exec.Command("git", "config", "--global", "--add", "safe.directory", dataPath).Run()
	if err != nil {
		log.Error("error while whitelisting repo")
		return err
	}
	return nil
}

func (c *Container) Pull(cli *client.Client, token string, uri string, branch string, domain string) (err error) {
	log.Info("pulling repository")
	if c.ExpectingFirstCommit {
		log.Info("pulling first commit")
	} else {
		log.Info("pulling subsequent commit")
	}
	// check state is valid for pull
	shouldRestart := false
	status, err := c.getStatus(cli, nil, nil)
	if err != nil {
		return err
	}
	if status == "paused" {
		log.Error("unable tu pull while frozen")
		err = errors.New("unable to perform pull while the container is frozen")
		return err
	} else if status == "running" || status == "restarting" {
		log.Info("stopping container in preparation for pull - container will be restarted when finished")
		err = c.Stop(cli)
		if err != nil {
			return err
		}
		shouldRestart = true
	}
	gitUrl := "https://x-access-token:" + token + "@" + domain + "/" + uri
	isUpdated := false
	dataPath := c.Dir()
	if c.ExpectingFirstCommit {
		shouldRestart = true
	}
	// check if git repo is initialized
	isRepo, err := c.isGitRepository()
	if err != nil {
		log.Error("error while checking if project is within a git repository: ", err)
		return err
	}
	// if the repo is not initialized, we will first pull aside the data, perform a clone, and move data back
	if !isRepo {
		log.Info("the container is not on a github repository, initializing")
		temporaryId, err := c.pullAside()
		if err != nil {
			return err
		}
		log.Info("initializing container")
		out, err := exec.Command(
			"git", "-C", dataPath, "clone", "--depth", "1", "-b", branch, gitUrl, ".",
		).CombinedOutput()
		log.Info(string(out))
		if err != nil {
			log.Error("error while initializing: ", err)
			_ = c.bringTogether(temporaryId)
			return err
		}
		err = c.bringTogether(temporaryId)
		if err != nil {
			return err
		}
		isUpdated = true
	}
	// clean repo
	err = c.Whitelist()
	if err != nil {
		return err
	}
	log.Info("resetting repo")
	err = exec.Command("git", "-C", dataPath, "reset", "--hard").Run()
	if err != nil {
		log.Error("error resetting repo: ", err)
		return err
	}
	log.Info("cleaning up repo")
	err = exec.Command("git", "-C", dataPath, "clean", "-dff").Run()
	if err != nil {
		log.Error("error cleaning up repo: ", err)
		return err
	}
	if !isUpdated {
		// update remote url (token)
		log.Info("updating remote")
		err = exec.Command("git", "-C", dataPath, "remote", "set-url", "origin", gitUrl).Run()
		if err != nil {
			log.Error("error while updating remote")
			return err
		}
		// ensure correct branch
		log.Info("checking out branch")
		err = exec.Command("git", "-C", dataPath, "checkout", branch).Run()
		if err != nil {
			log.Error("error while checking out branch: ", err)
			return err
		}
		// pull changes
		log.Info("pulling changes")
		err = exec.Command("git", "-C", dataPath, "pull", "--progress", "--rebase").Run()
		if err != nil {
			log.Info("error while pulling changes: ", err)
			return err
		}
	}
	// delete container to re-crease, just in case the .env file changed
	err = c.deleteContainer(cli)
	if err != nil {
		return err
	}
	err = c.createContainer(cli)
	if err != nil {
		return err
	}
	err = c.ReadyFs()
	if err != nil {
		return err
	}
	if shouldRestart {
		log.Info("restarting the container to match the initial state before pull")
		err = c.Start(cli)
		if err != nil {
			log.Error("error while restarting container: ", err)
			return err
		}
	}
	log.Info("finished pulling")
	return err
}

func (c *Container) getTemporaryFolder(temporaryId string) string {
	return filepath.Join(c.homeDir(), "tmp-"+temporaryId)
}

func (c *Container) pullAside() (temporaryId string, err error) {
	log.Info("pulling aside")
	temporaryId = randstr.Hex(8)
	targetPath := c.getTemporaryFolder(temporaryId)
	err = os.MkdirAll(targetPath, os.ModePerm)
	if err != nil {
		return "", err
	}
	originPath := c.Dir()
	r, err := exec.Command("rsync", "-a", "--remove-source-files", c.appendSlash(originPath), targetPath).Output()
	if err != nil {
		log.Error("error while pulling aside, trying to bring together: ", string(r), ", ", err)
		_ = c.bringTogether(temporaryId)
		return "", err
	}
	err = c.Clear()
	return temporaryId, err
}

func (c *Container) bringTogether(temporaryId string) (err error) {
	log.Info("bringing together aside")
	temporaryDirectory := c.getTemporaryFolder(temporaryId)
	originPath := c.Dir()
	r, err := exec.Command("rsync", "-a", "--remove-source-files", "--ignore-existing", c.appendSlash(temporaryDirectory), originPath).Output()
	if err != nil {
		log.Error("error while bringing together: ", string(r), ", ", err)
		return err
	}
	// cleanup
	err = os.RemoveAll(temporaryDirectory)
	if err != nil {
		log.Error("error while cleaning after bringing together")
	}
	return err
}

func (c *Container) appendSlash(str string) string {
	if len(str) == 0 || str[len(str)-1] != os.PathSeparator {
		return str + string(os.PathSeparator)
	}
	return str
}
