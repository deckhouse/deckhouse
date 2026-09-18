// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package state

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

const (
	MasterHostsCacheKey = "cluster-hosts"
)

func SaveMasterHosts(ctx context.Context, cache Cache, hosts map[string]string) error {
	return cache.SaveStruct(ctx, MasterHostsCacheKey, hosts)
}

func SaveMasterHostsToCache(ctx context.Context, cache Cache, hosts map[string]string) {
	if err := SaveMasterHosts(ctx, cache, hosts); err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Cannot save ssh hosts %v", err))
	}
}

// MergeMasterHosts joins host lists by node name, a later list winning over an earlier one:
// the hosts cache is rewritten every time a master is created or recreated, while the
// session and the infrastructure state may still hold the address of a replaced machine.
func MergeMasterHosts(lists ...[]session.Host) []session.Host {
	byName := make(map[string]string)

	// An entry without an address says nothing about where the node is. One writer of the
	// cache stores a master whose SSH address came back empty, and letting that win would
	// hide the address the session still has.
	for _, host := range slices.Concat(lists...) {
		if host.Host == "" {
			continue
		}

		byName[host.Name] = host.Host
	}

	merged := make([]session.Host, 0, len(byName))
	for name, address := range byName {
		merged = append(merged, session.Host{Host: address, Name: name})
	}

	sort.Sort(session.SortByName(merged))

	return merged
}

// masterNodeState reads the one output every master carries: the address it answers SSH on.
type masterNodeState struct {
	Outputs struct {
		MasterIPForSSH struct {
			Value string `json:"value"`
		} `json:"master_ip_address_for_ssh"`
	} `json:"outputs"`
}

// MasterHostsFromState names each master by the address in its own infrastructure state.
// The addresses of --ssh-host carry no node name, so the node-to-host mapping is built from
// what converge itself wrote when it created the machine. A state that parses to no address
// is not an error here: an immutable or half-created master simply has none.
func MasterHostsFromState(nodesState map[string][]byte) []session.Host {
	hosts := make([]session.Host, 0, len(nodesState))

	for nodeName, nodeState := range nodesState {
		parsed := masterNodeState{}
		if err := json.Unmarshal(nodeState, &parsed); err != nil {
			continue
		}

		if parsed.Outputs.MasterIPForSSH.Value == "" {
			continue
		}

		hosts = append(hosts, session.Host{Host: parsed.Outputs.MasterIPForSSH.Value, Name: nodeName})
	}

	sort.Sort(session.SortByName(hosts))

	return hosts
}

func GetMasterHostsIPs(ctx context.Context, cache Cache) ([]session.Host, error) {
	inCache, err := cache.InCache(ctx, MasterHostsCacheKey)
	if err != nil {
		return nil, err
	}

	if !inCache {
		return make([]session.Host, 0), nil
	}
	var hosts map[string]string
	err = cache.LoadStruct(ctx, MasterHostsCacheKey, &hosts)
	if err != nil {
		return nil, err
	}
	mastersIPs := make([]session.Host, 0, len(hosts))
	for name, ip := range hosts {
		mastersIPs = append(mastersIPs, session.Host{Host: ip, Name: name})
	}

	sort.Sort(session.SortByName(mastersIPs))

	return mastersIPs, nil
}

func GetMasterHosts(ctx context.Context, cache Cache) ([]sshconfig.Host, error) {
	hostsToReturn := make([]sshconfig.Host, 0)
	hosts, err := GetMasterHostsIPs(ctx, cache)
	if err != nil {
		return hostsToReturn, err
	}

	for _, h := range hosts {
		hostsToReturn = append(hostsToReturn, sshconfig.Host{Host: h.Host})
	}

	return hostsToReturn, nil
}
