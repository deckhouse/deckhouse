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

package fastping

import (
	"net"
	"slices"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Pinger is the main pinging struct.
type Pinger struct {
	OnRecv func(PacketResult) // Called when packet is received

	interval time.Duration
	count    int
	timeout  time.Duration

	hosts []string

	sentCount map[string]int
	recvCount map[string]int
	hostMap   map[string]string

	mu sync.Mutex

	// internal fields
	// socket, etc.
}

// NewPinger creates a new Pinger instance.
func NewPinger(hosts []string, count int, interval, timeout time.Duration) *Pinger {
	ipHosts := []string{}
	hostMap := make(map[string]string)

	for _, host := range hosts {
		ips, err := net.LookupIP(host)
		if err != nil {
			log.Warn("failed to resolve host: %s, err: %v", host, err)
			continue
		}

		for _, ip := range ips {
			if ip.To4() == nil {
				continue // Skip non-IPv4 addresses
			}
			ipStr := ip.String()
			if existingHost, exists := hostMap[ipStr]; exists && existingHost != host {
				log.Warn("IP %s already mapped to %s, adding for %s", ipStr, existingHost, host)
			}
			hostMap[ipStr] = host
			if !slices.Contains(ipHosts, ipStr) {
				ipHosts = append(ipHosts, ipStr)
			}
			// log.Info(fmt.Sprintf("mapped host %s to IP %s", host, ipStr))
		}
	}

	return &Pinger{
		hosts:     ipHosts,
		hostMap:   hostMap,
		count:     count,
		interval:  interval,
		timeout:   timeout,
		sentCount: make(map[string]int),
		recvCount: make(map[string]int),
	}
}
