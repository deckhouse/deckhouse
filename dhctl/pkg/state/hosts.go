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
	"sort"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

const (
	MasterHostsCacheKey = "cluster-hosts"
)

func SaveMasterHostsToCache(cache Cache, hosts map[string]string) {
	if err := cache.SaveStruct(MasterHostsCacheKey, hosts); err != nil {
		log.WarnF("Cannot save ssh hosts %v\n", err)
	}
}

func GetMasterHostsIPs(cache Cache) ([]session.Host, error) {
	inCache, err := cache.InCache(MasterHostsCacheKey)
	if err != nil {
		return nil, err
	}

	if !inCache {
		return make([]session.Host, 0), nil
	}
	var hosts map[string]string
	err = cache.LoadStruct(MasterHostsCacheKey, &hosts)
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
