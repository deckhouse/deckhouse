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
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

const ownIPRetry = time.Second

// OwnIP returns the address of the main network from the own Node: the first
// IPv4 InternalIP, else the first IPv4 ExternalIP.
func OwnIP(node domain.NodeSignals) (netip.Addr, bool) {
	for _, addrs := range [][]string{node.InternalIPs, node.ExternalIPs} {
		for _, s := range addrs {
			if addr, err := netip.ParseAddr(s); err == nil && addr.Is4() {
				return addr, true
			}
		}
	}

	return netip.Addr{}, false
}

// WaitOwnIP waits until the own Node has an IPv4 address and the address is on
// a node interface. interfaceAddrs is net.InterfaceAddrs: on Linux it reads
// the addresses through netlink. While the address is missing, WaitOwnIP sets
// the condition reason NodeIPMismatch. It returns an error only when ctx ends.
func WaitOwnIP(
	ctx context.Context,
	node func() (domain.NodeSignals, bool),
	interfaceAddrs func() ([]net.Addr, error),
	status domain.Status,
	logger *log.Logger,
) (netip.Addr, error) {
	ticker := time.NewTicker(ownIPRetry)
	defer ticker.Stop()

	reported := ""

	for {
		if signals, observed := node(); observed {
			addr, problem := checkOwnIP(signals, interfaceAddrs)
			if problem == "" {
				logger.Info("own address of the main network", "address", addr)
				return addr, nil
			}

			if problem != reported {
				logger.Warn("own address of the main network is missing, waiting", "problem", problem)
				status.SetCondition(domain.ReasonNodeIPMismatch, problem)
				reported = problem
			}
		}

		select {
		case <-ctx.Done():
			return netip.Addr{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func checkOwnIP(node domain.NodeSignals, interfaceAddrs func() ([]net.Addr, error)) (netip.Addr, string) {
	addr, ok := OwnIP(node)
	if !ok {
		return netip.Addr{}, "the Node has no IPv4 InternalIP or ExternalIP"
	}

	addrs, err := interfaceAddrs()
	if err != nil {
		return netip.Addr{}, fmt.Sprintf("cannot read the node interface addresses: %v", err)
	}

	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}

		if got, ok := netip.AddrFromSlice(ip); ok && got.Unmap() == addr {
			return addr, ""
		}
	}

	return netip.Addr{}, fmt.Sprintf("the Node address %s is not on a node interface", addr)
}
