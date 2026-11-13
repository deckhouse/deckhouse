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
	"bytes"
	"net/netip"
	"sync/atomic"

	hcml "github.com/hashicorp/memberlist"
)

// eventBuffer holds the burst of member events of one push/pull with a large cluster.
const eventBuffer = 1024

type eventKind string

const (
	eventJoin   eventKind = "joined"
	eventLeave  eventKind = "left_or_failed"
	eventUpdate eventKind = "updated"
)

type event struct {
	kind eventKind
	name string
	addr netip.Addr
	// meta is a copy of the NodeMeta of the member.
	meta []byte
}

// delegate is the EventDelegate of all instances of one Network. memberlist
// calls it with its node lock held. Thus the methods only copy the data to a
// channel. They do not block and do not call memberlist. When the channel is
// full, they drop the event and count it.
type delegate struct {
	events        chan event
	droppedEvents atomic.Int64
}

func newDelegate() *delegate {
	return &delegate{events: make(chan event, eventBuffer)}
}

func (d *delegate) NotifyJoin(node *hcml.Node) { d.enqueue(eventJoin, node) }

func (d *delegate) NotifyLeave(node *hcml.Node) { d.enqueue(eventLeave, node) }

func (d *delegate) NotifyUpdate(node *hcml.Node) { d.enqueue(eventUpdate, node) }

func (d *delegate) enqueue(kind eventKind, node *hcml.Node) {
	addr, _ := netip.AddrFromSlice(node.Addr)

	select {
	case d.events <- event{kind: kind, name: node.Name, addr: addr.Unmap(), meta: bytes.Clone(node.Meta)}:
	default:
		d.droppedEvents.Add(1)
	}
}
