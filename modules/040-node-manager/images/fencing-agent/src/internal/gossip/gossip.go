package gossip

type Gossip interface {
	Start(nodeIps []string) error
	PrintNodes()
	IsAlone() bool
	SetAlone(status bool)
	NumMembers() int
}
