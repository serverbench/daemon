package containers

const Drop = "DROP"
const Accept = "ACCEPT"

type Port struct {
	Port    int      `json:"port"`
	Policy  string   `json:"policy"` // DROP or ACCEPT
	Remotes []string `json:"remotes"`
}
