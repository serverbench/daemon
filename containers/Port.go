package containers

const Drop = "DROP"
const Accept = "ACCEPT"

type Port struct {
	Port    int      `json:"port"`
	Policy  string   `json:"policy"` // drop or accept
	Remotes []string `json:"remotes"`
}
