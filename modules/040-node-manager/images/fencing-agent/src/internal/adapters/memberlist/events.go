/*
Copyright 2026 Flant JSC

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

package memberlist

import (
	"sync/atomic"

	hcml "github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// eventBuffer absorbs the per-member event burst of a push/pull with a large group.
const eventBuffer = 1024

type nodeEvent struct {
	kind string
	name string
	addr string
}

// eventDelegate offloads notifications to its own goroutine: memberlist invokes
// them under its state lock, so the delegate must not block. A full buffer drops
// and counts events.
type eventDelegate struct {
	logger  *log.Logger
	events  chan nodeEvent
	dropped atomic.Int64
}

func newEventDelegate(logger *log.Logger) *eventDelegate {
	return &eventDelegate{
		logger: logger,
		events: make(chan nodeEvent, eventBuffer),
	}
}

func (d *eventDelegate) NotifyJoin(node *hcml.Node) {
	d.enqueue("joined", node)
}

func (d *eventDelegate) NotifyLeave(node *hcml.Node) {
	d.enqueue("left_or_failed", node)
}

func (d *eventDelegate) NotifyUpdate(node *hcml.Node) {
	d.enqueue("updated", node)
}

func (d *eventDelegate) enqueue(kind string, node *hcml.Node) {
	select {
	case d.events <- nodeEvent{kind: kind, name: node.Name, addr: node.Address()}:
	default:
		d.dropped.Add(1)
	}
}

func (d *eventDelegate) run(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case event := <-d.events:
			if event.kind == "updated" {
				d.logger.Debug("membership changed", "event", event.kind, "member", event.name, "address", event.addr)
			} else {
				d.logger.Info("membership changed", "event", event.kind, "member", event.name, "address", event.addr)
			}

			if dropped := d.dropped.Swap(0); dropped > 0 {
				d.logger.Warn("membership events were dropped, the log is incomplete", "count", dropped)
			}
		}
	}
}
