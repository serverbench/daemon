package main

import (
	docker "github.com/docker/docker/client"
	"supervisor/client"
)

func main() {
	cli, err := docker.NewClientWithOpts(
		docker.FromEnv,
		docker.WithVersion("1.47"),
	)
	if err != nil {
		panic(err)
	}
	defer cli.Close()
	serverbench := client.Client{}
	err = serverbench.Start(cli)
	if err != nil {
		panic(err)
	}
}
