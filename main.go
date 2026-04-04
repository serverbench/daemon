package main

import (
	"supervisor/client"
	"time"

	docker "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

func main() {
	cli, err := docker.NewClientWithOpts(
		docker.FromEnv,
		docker.WithVersion("1.47"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	const (
		initialBackoff = 5 * time.Second
		maxBackoff     = 60 * time.Second
	)
	backoff := initialBackoff

	for {
		serverbench := client.Client{}
		err = serverbench.Start(cli)
		if err != nil {
			log.Errorf("daemon error: %v", err)
			log.Infof("reconnecting in %v...", backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			// Clean exit – reset backoff and reconnect immediately.
			backoff = initialBackoff
		}
	}
}
