package gossip

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

type Memberlist struct {
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

	return &Memberlist{
		logger: logger,
		list:   list,
	}, nil
}

func (ml *Memberlist) Start(peers []string) error {
	if len(peers) == 0 {
		ml.logger.Info("No peers found, starting as a single node")
		return nil
	}
	numJoined, err := ml.list.Join(peers)
	if err != nil {
		ml.logger.Error("Unable to join to memberlist cluster", zap.Error(err))
		return err
	}
	ml.logger.Info("Joined to memberlist cluster", zap.Int("numJoined", numJoined), zap.Int("peersAttemted", len(peers)))
	return nil
}

func (ml *Memberlist) PrintNodes() {
	for _, member := range ml.list.Members() {
		fmt.Printf("Member: %s %s\n", member.Name, member.Addr)
	}
}
