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

package protocol

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	hcml "github.com/hashicorp/memberlist"
)

// testNetwork connects memberlist instances in the process. Unlike
// memberlist.MockNetwork, it does not block a sender: like UDP, it drops a
// packet when the queue of the receiver is full. The blocking sends of
// MockNetwork deadlock two instances that send acks to each other.
type testNetwork struct {
	mu    sync.Mutex
	port  int
	nodes map[string]*testTransport
}

func newTestNetwork() *testNetwork {
	return &testNetwork{port: 7000, nodes: make(map[string]*testTransport)}
}

// transport returns a new transport with a unique address. The network finds
// it by the node name and by the address.
func (n *testNetwork) transport(name string) *testTransport {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.port++
	t := &testTransport{
		net:     n,
		addr:    &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: n.port},
		packets: make(chan *hcml.Packet, 1024),
		streams: make(chan net.Conn, 16),
	}
	n.nodes[name] = t
	n.nodes[t.addr.String()] = t

	return t
}

func (n *testNetwork) find(a hcml.Address) (*testTransport, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	t, ok := n.nodes[a.Name]
	if !ok {
		t, ok = n.nodes[a.Addr]
	}
	if !ok || t.closed {
		return nil, errors.New("no route to " + a.String())
	}

	return t, nil
}

type testTransport struct {
	net     *testNetwork
	addr    *net.UDPAddr
	packets chan *hcml.Packet
	streams chan net.Conn
	// closed is guarded by the network mutex.
	closed bool
}

var _ hcml.NodeAwareTransport = (*testTransport)(nil)

func (t *testTransport) FinalAdvertiseAddr(string, int) (net.IP, int, error) {
	return t.addr.IP, t.addr.Port, nil
}

func (t *testTransport) WriteTo(b []byte, addr string) (time.Time, error) {
	return t.WriteToAddress(b, hcml.Address{Addr: addr})
}

func (t *testTransport) WriteToAddress(b []byte, a hcml.Address) (time.Time, error) {
	dest, err := t.net.find(a)
	if err != nil {
		return time.Time{}, err
	}

	now := time.Now()
	select {
	case dest.packets <- &hcml.Packet{Buf: append([]byte(nil), b...), From: t.addr, Timestamp: now}:
	default:
	}

	return now, nil
}

func (t *testTransport) PacketCh() <-chan *hcml.Packet { return t.packets }

func (t *testTransport) DialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	return t.DialAddressTimeout(hcml.Address{Addr: addr}, timeout)
}

func (t *testTransport) DialAddressTimeout(a hcml.Address, _ time.Duration) (net.Conn, error) {
	dest, err := t.net.find(a)
	if err != nil {
		return nil, err
	}

	t.net.mu.Lock()
	defer t.net.mu.Unlock()

	local, remote := net.Pipe()
	select {
	case dest.streams <- remote:
		return local, nil
	default:
		return nil, errors.New("connection refused by " + strconv.Itoa(dest.addr.Port))
	}
}

func (t *testTransport) StreamCh() <-chan net.Conn { return t.streams }

// Shutdown refuses new packets and streams and closes the streams that
// memberlist did not accept, so that their writers do not wait forever.
func (t *testTransport) Shutdown() error {
	t.net.mu.Lock()
	defer t.net.mu.Unlock()

	t.closed = true
	for {
		select {
		case conn := <-t.streams:
			_ = conn.Close()
		default:
			return nil
		}
	}
}
