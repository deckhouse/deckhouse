package gossip

import (
	"log"
	"strconv"
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

func NewMemberList(logger *zap.Logger, memberListPort string, nodeName string, nodeIP string) (Gossip, error) {
	config := memberlist.DefaultLANConfig()
	config.Logger = log.New(NewZapAdapter(logger), "[memberlist] ", 0)

	//config.ProbeInterval = 50 * time.Millisecond
	//config.ProbeTimeout = 25 * time.Millisecond
	//config.SuspicionMult = 1
	//config.IndirectChecks = 2 // TODO discuss with team, experiment
	//config.GossipInterval = 50 * time.Millisecond
	//config.RetransmitMult = 2

	config.ProbeInterval = 200 * time.Millisecond
	config.ProbeTimeout = 100 * time.Millisecond
	config.SuspicionMult = 2
	config.IndirectChecks = 3
	config.GossipInterval = 100 * time.Millisecond
	config.RetransmitMult = 3
	config.GossipToTheDeadTime = 2 * time.Second
	if memberListPort != "" {
		memberListPortInt, err := strconv.Atoi(memberListPort)
		if err != nil {
			return nil, err
		}
		config.BindPort = memberListPortInt
		config.AdvertisePort = memberListPortInt
	}
	if nodeName != "" {
		config.Name = nodeName
	}
	config.AdvertiseAddr = nodeIP
	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err
	}
	memList := Memberlist{
		logger:  logger,
		list:    list,
		isAlone: true,
	}
	eventHandler := NewEventHandler(logger, nil, memList.SetAlone)
	config.Events = eventHandler
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
