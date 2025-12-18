package swarm

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

type Gossip interface {
	Start(nodeIps []string) error
}

type Swarm struct {
	logger *zap.Logger
	list   *memberlist.Memberlist
}

func NewMemberList(logger *zap.Logger) (Gossip, error) {
	config := memberlist.DefaultLocalConfig()
	if portStr := os.Getenv("MEMBERLIST_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, err
		}
		config.BindPort = port
		config.AdvertisePort = port
	}
	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err
	}

	return &Swarm{
		logger: logger,
		list:   list,
	}, nil
}

func (s *Swarm) Start(nodeIps []string) error {
	_, err := s.list.Join(nodeIps)
	if err != nil {
		return err
	}
	return nil
}

func (s *Swarm) PrintNodes() {
	for _, member := range s.list.Members() {
		fmt.Printf("Member: %s %s\n", member.Name, member.Addr)
	}
}
