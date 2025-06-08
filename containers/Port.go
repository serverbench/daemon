package containers

const Drop = "drop"
const Accept = "accept"

type Port struct {
	Port    int      `json:"port"`
	Policy  string   `json:"policy"` // drop or accept
	Remotes []string `json:"remotes"`
}
