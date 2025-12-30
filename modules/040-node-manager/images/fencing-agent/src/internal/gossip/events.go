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
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

type EventHandler struct {
	logger               *zap.Logger
	nodesLeft            map[string]time.Time
	nodesJoin            map[string]time.Time
	muJoin               *sync.Mutex
	muLeft               *sync.Mutex
	onNodeFailure        func(nodeName string, nodeAddr string)
	onNodeJoin           func(status bool)
	minEventIntervalJoin time.Duration
	minEventIntervalLeft time.Duration
}

func NewEventHandler(logger *zap.Logger, minEventIntervalJoin, minEventIntervalLeft time.Duration, onNodeFailure func(nodeName string, nodeAddr string), onNodeJoin func(status bool)) *EventHandler {
	return &EventHandler{
		logger:               logger,
		onNodeFailure:        onNodeFailure,
		onNodeJoin:           onNodeJoin,
		nodesLeft:            make(map[string]time.Time),
		nodesJoin:            make(map[string]time.Time),
		muJoin:               &sync.Mutex{},
		muLeft:               &sync.Mutex{},
		minEventIntervalJoin: minEventIntervalJoin,
		minEventIntervalLeft: minEventIntervalLeft,
	}
}

func (d *EventHandler) NotifyJoin(node *memberlist.Node) {
	d.muJoin.Lock()
	defer d.muJoin.Unlock()
	if t, ok := d.nodesLeft[node.Name]; ok {
		if time.Since(t) < d.minEventIntervalJoin {
			d.logger.Debug("False joining", zap.String("node", node.Name))
			return
		}
	}
	d.nodesJoin[node.Name] = time.Now()
	d.logger.Info("Node joined", zap.String("node", node.Name))
	d.onNodeJoin(false)
	// mean that node is not alone in cluster
	//if d.memberlist.NumMembers() > 1 {
	//	d.memberlist.SetAlone(false)
	//}
}

func (d *EventHandler) NotifyLeave(node *memberlist.Node) {
	d.muLeft.Lock()
	defer d.muLeft.Unlock()
	if t, ok := d.nodesJoin[node.Name]; ok {
		if time.Since(t) < d.minEventIntervalLeft {
			d.logger.Debug("False leaving", zap.String("node", node.Name))
			return
		}
	}
	d.nodesLeft[node.Name] = time.Now()
	d.logger.Info("Node left, have to notify cilium", zap.String("node", node.Name))

	//if d.memberlist.NumMembers() == 1 {
	//	d.memberlist.SetAlone(true)
	//}
	if d.onNodeFailure != nil {
		// TODO: send notification to cilium or another network agent
	}
}

func (d *EventHandler) NotifyUpdate(node *memberlist.Node) {
	d.logger.Info("Node updated", zap.String("node", node.Name))
}
