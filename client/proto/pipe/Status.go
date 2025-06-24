package pipe

import "github.com/docker/docker/api/types/events"

var NormalizeDockerStatus = map[events.Action]string{
	"create":   "created",
	"start":    "running",
	"restart":  "restarting",
	"pause":    "paused",
	"unpause":  "running",
	"die":      "exited",
	"destroy":  "removing",
	"kill":     "running",
	"oom":      "dead",
	"exec_die": "running", // container stays up after exec ends
}

type Status struct {
	Status string `json:"status"`
}
