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
	"sort"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	MasterHostsCacheKey = "cluster-hosts"
)

func SaveMasterHostsToCache(ctx context.Context, cache Cache, hosts map[string]string) {
	if err := cache.SaveStruct(ctx, MasterHostsCacheKey, hosts); err != nil {
		log.WarnF("Cannot save ssh hosts %v\n", err)
	}
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
