package gossip

type Gossip interface {
	Start(nodeIps []string) error
	PrintNodes()
}
