/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gossip

import (
	agentconfig "fencing-controller/internal/config"
	"log"

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
	configML.ProbeInterval = config.ProbeInterval
	configML.ProbeTimeout = config.ProbeTimeout
	configML.SuspicionMult = config.SuspicionMult
	configML.IndirectChecks = config.IndirectChecks
	configML.GossipInterval = config.GossipInterval
	configML.RetransmitMult = config.RetransmitMult
	configML.GossipToTheDeadTime = config.GossipToTheDeadTime
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
	eventHandler := NewEventHandler(logger, config.MinEventIntervalJoin, config.MinEventIntervalLeft, nil, memList.SetAlone)
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
