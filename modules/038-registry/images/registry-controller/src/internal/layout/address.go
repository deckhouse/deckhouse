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

package layout

import (
	"net"
	"strconv"
	"strings"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// StorageAddresses is where each replica serves, as "host:port", given the node addresses the
// replicas run on.
//
// The addresses are discovered as node addresses, without the port. The port is the one the replica
// on that host reported in its own status; without a usable report it is the one the address
// already carries, and otherwise constant.StoragePort — not the port in constant.Host, which is part
// of a name and on the node belongs to the agent. Matched by host, so an IPv6 address matches
// whether or not it was bracketed.
//
// The report decides because of the update to the store's current ports. The StatefulSet replaces
// its pods only when storageupdate deletes them, so until then a replica runs the previous pod spec
// and serves on constant.LegacyStoragePort — and the images its replacement needs are pulled through
// the agents from those same replicas. Pointing the agents at constant.StoragePort first leaves
// nothing to pull them from. A replaced replica reports its new port as soon as its registry answers,
// and storageupdate waits for that report before it replaces the next one. Once no replica on the
// previous pod spec can exist, the constant alone will do.
func StorageAddresses(addresses []string, replicas []registryv1alpha1.StorageReplicaStatus) []string {
	reported := reportedPorts(replicas)

	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		host, port := splitAddress(address)
		if reportedPort, ok := reported[hostKey(host)]; ok {
			port = reportedPort
		}
		if port == "" {
			port = strconv.Itoa(constant.StoragePort)
		}
		out = append(out, net.JoinHostPort(host, port))
	}
	return out
}

// reportedPorts is the port each replica reported serving on, by host.
//
// A report that is not "host:port" with a valid port is skipped, and the replica gets the
// default: a malformed report must not take a replica out of the layout.
func reportedPorts(replicas []registryv1alpha1.StorageReplicaStatus) map[string]string {
	ports := make(map[string]string, len(replicas))
	for i := range replicas {
		host, port, err := net.SplitHostPort(replicas[i].Address)
		if err != nil || host == "" {
			continue
		}
		number, err := strconv.Atoi(port)
		if err != nil || number <= 0 || number > 65535 {
			continue
		}
		ports[hostKey(host)] = strconv.Itoa(number)
	}
	return ports
}

// sameHost reports whether two addresses, with or without a port, name the same host.
func sameHost(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	hostA, _ := splitAddress(a)
	hostB, _ := splitAddress(b)
	return hostKey(hostA) == hostKey(hostB)
}

// splitAddress splits "host:port", "[v6]:port", "[v6]", a bare IPv6 address or a bare name into the
// host, without brackets, and the port, empty when there is none.
func splitAddress(address string) (string, string) {
	if h, p, err := net.SplitHostPort(address); err == nil {
		return h, p
	}
	return strings.TrimSuffix(strings.TrimPrefix(address, "["), "]"), ""
}

// hostKey is the form two spellings of one host compare equal in: the canonical text of an IP
// address, a name in lower case.
func hostKey(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return strings.ToLower(host)
}
