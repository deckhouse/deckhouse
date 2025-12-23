package gossip

import (
	agentconfig "fencing-controller/internal/config"
	"log"
	"time"

	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

type Memberlist struct {
	logger      *zap.Logger
	list        *memberlist.Memberlist
	isAlone     bool
	isConnected bool
}

func NewMemberList(logger *zap.Logger, config agentconfig.Config, nodeAddr string) (Gossip, error) {
	configML := memberlist.DefaultLANConfig()
	configML.Logger = log.New(NewZapAdapter(logger), "[memberlist] ", 0)
	configML.ProbeInterval = config.ProbeInterval * time.Millisecond
	configML.ProbeTimeout = config.ProbeTimeout * time.Millisecond
	configML.SuspicionMult = config.SuspicionMult
	configML.IndirectChecks = 3
	configML.GossipInterval = 100 * time.Millisecond
	configML.RetransmitMult = 4
	configML.GossipToTheDeadTime = 2 * time.Second
	configML.BindPort = config.MemberListPort
	configML.AdvertisePort = config.MemberListPort
	configML.Name = config.NodeName
	configML.AdvertiseAddr = nodeAddr
	list, err := memberlist.Create(configML)
	if err != nil {
		return nil, err
	}
	memList := Memberlist{
		logger:  logger,
		list:    list,
		isAlone: true,
	}
	eventHandler := NewEventHandler(logger, config.MinEventInterval, nil, memList.SetAlone)
	configML.Events = eventHandler
	return &memList, nil
}

func (ml *Memberlist) Start(peers []string) error {
	if len(peers) == 0 {
		ml.logger.Info("No peers found, starting as a single node")
		ml.SetAlone(true)
		ml.isConnected = true
		return nil
	}
	numJoined, err := ml.list.Join(peers)
	if err != nil {
		ml.logger.Error("Unable to join to memberlist cluster", zap.Error(err))
		ml.isConnected = false
		return err
	}
	ml.logger.Info("Joined to memberlist cluster", zap.Int("numJoined", numJoined), zap.Int("peersAttemted", len(peers)))
	ml.SetAlone(false)
	ml.isConnected = true
	return nil
}

func (ml *Memberlist) PrintNodes() {
	for _, member := range ml.list.Members() {
		ml.logger.Info("Member info", zap.String("name", member.Name), zap.Any("addr", member.FullAddress()))
	}
}

func (ml *Memberlist) IsAlone() bool {
	return ml.isAlone
}

func (ml *Memberlist) SetAlone(status bool) {
	ml.isAlone = status
}

func (ml *Memberlist) IsConnected() bool {
	return ml.isConnected
}

func (ml *Memberlist) NumMembers() int {
	return ml.list.NumMembers()
}
