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
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

func TestOwnIP(t *testing.T) {
	tests := []struct {
		name string
		node domain.NodeSignals
		want string
	}{
		{"first InternalIP", domain.NodeSignals{InternalIPs: []string{"10.0.0.5", "10.0.0.6"}, ExternalIPs: []string{"203.0.113.10"}}, "10.0.0.5"},
		{"IPv6 InternalIP is skipped", domain.NodeSignals{InternalIPs: []string{"fd00::5", "10.0.0.5"}}, "10.0.0.5"},
		// A cloud node without InternalIP.
		{"ExternalIP without InternalIP", domain.NodeSignals{ExternalIPs: []string{"203.0.113.10"}}, "203.0.113.10"},
		{"ExternalIP when InternalIP is IPv6 only", domain.NodeSignals{InternalIPs: []string{"fd00::5"}, ExternalIPs: []string{"203.0.113.10"}}, "203.0.113.10"},
		{"no IPv4", domain.NodeSignals{InternalIPs: []string{"fd00::5"}, ExternalIPs: []string{"2001:db8::10"}}, ""},
		{"no addresses", domain.NodeSignals{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := OwnIP(tt.node)
			if tt.want == "" {
				if ok {
					t.Errorf("OwnIP() = %s, want none", got)
				}
				return
			}
			if !ok || got != netip.MustParseAddr(tt.want) {
				t.Errorf("OwnIP() = (%s, %t), want %s", got, ok, tt.want)
			}
		})
	}
}

type fakeStatus struct {
	mu      sync.Mutex
	reasons []domain.Reason
}

func (s *fakeStatus) SetCondition(reason domain.Reason, _ string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reasons = append(s.reasons, reason)
}

func (s *fakeStatus) SetCPSource(domain.CPSource) {}

func (s *fakeStatus) SetProfile(string) {}

func (s *fakeStatus) got() []domain.Reason {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.reasons)
}

// fakeNode is the own Node and the node interfaces. Its fields change while WaitOwnIP runs.
type fakeNode struct {
	mu       sync.Mutex
	observed bool
	signals  domain.NodeSignals
	ifaces   []net.Addr
}

func (n *fakeNode) set(f func(*fakeNode)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	f(n)
}

func (n *fakeNode) snapshot() (domain.NodeSignals, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.signals, n.observed
}

func (n *fakeNode) interfaceAddrs() ([]net.Addr, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.ifaces), nil
}

func ipNet(s string) net.Addr {
	ip, ipnet, _ := net.ParseCIDR(s)
	return &net.IPNet{IP: ip, Mask: ipnet.Mask}
}

type waitResult struct {
	addr netip.Addr
	err  error
}

func startWait(t *testing.T, node *fakeNode, status *fakeStatus) <-chan waitResult {
	t.Helper()

	done := make(chan waitResult, 1)
	go func() {
		addr, err := WaitOwnIP(t.Context(), node.snapshot, node.interfaceAddrs, status, log.NewNop())
		done <- waitResult{addr, err}
	}()

	return done
}

// No IPv4 at start.
func TestWaitOwnIPWaitsWithNodeIPMismatchUntilTheNodeHasIPv4(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		node := &fakeNode{observed: true, signals: domain.NodeSignals{InternalIPs: []string{"fd00::5"}}}
		status := &fakeStatus{}

		done := startWait(t, node, status)

		time.Sleep(time.Minute)
		synctest.Wait()
		select {
		case r := <-done:
			t.Fatalf("WaitOwnIP returned %v without an IPv4 address, want it to wait", r)
		default:
		}
		if got := status.got(); !slices.Equal(got, []domain.Reason{domain.ReasonNodeIPMismatch}) {
			t.Fatalf("condition reasons = %v, want one NodeIPMismatch", got)
		}

		node.set(func(n *fakeNode) {
			n.signals.InternalIPs = []string{"10.0.0.5"}
			n.ifaces = []net.Addr{ipNet("127.0.0.1/8"), ipNet("10.0.0.5/24")}
		})

		r := <-done
		if r.err != nil || r.addr != netip.MustParseAddr("10.0.0.5") {
			t.Errorf("WaitOwnIP() = (%s, %v), want 10.0.0.5", r.addr, r.err)
		}
	})
}

// The check before the creation of memberlist.
func TestWaitOwnIPRequiresTheAddressOnAnInterface(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		node := &fakeNode{
			observed: true,
			signals:  domain.NodeSignals{ExternalIPs: []string{"203.0.113.10"}},
			ifaces:   []net.Addr{ipNet("10.0.0.5/24")},
		}
		status := &fakeStatus{}

		done := startWait(t, node, status)

		time.Sleep(time.Minute)
		synctest.Wait()
		if len(done) != 0 {
			t.Fatal("WaitOwnIP returned while the address is not on an interface")
		}
		if got := status.got(); !slices.Equal(got, []domain.Reason{domain.ReasonNodeIPMismatch}) {
			t.Fatalf("condition reasons = %v, want one NodeIPMismatch", got)
		}

		node.set(func(n *fakeNode) { n.ifaces = append(n.ifaces, &net.IPAddr{IP: net.ParseIP("203.0.113.10")}) })

		if r := <-done; r.addr != netip.MustParseAddr("203.0.113.10") {
			t.Errorf("WaitOwnIP() = %s, want 203.0.113.10", r.addr)
		}
	})
}

func TestWaitOwnIPDoesNotReportBeforeTheNodeIsObserved(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		node := &fakeNode{}
		status := &fakeStatus{}

		done := startWait(t, node, status)

		time.Sleep(time.Minute)
		synctest.Wait()
		if got := status.got(); len(got) != 0 {
			t.Errorf("condition reasons = %v before the Node is observed, want none", got)
		}

		node.set(func(n *fakeNode) {
			n.observed = true
			n.signals.InternalIPs = []string{"10.0.0.5"}
			n.ifaces = []net.Addr{ipNet("10.0.0.5/24")}
		})
		<-done
	})
}

func TestWaitOwnIPStopsWithTheContext(t *testing.T) {
	node := &fakeNode{observed: true}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := WaitOwnIP(ctx, node.snapshot, node.interfaceAddrs, &fakeStatus{}, log.NewNop()); !errors.Is(err, context.Canceled) {
		t.Errorf("WaitOwnIP() error = %v, want context.Canceled", err)
	}
}
