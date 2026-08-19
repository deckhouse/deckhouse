// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modulesync

// This file reads ModuleConfig resources, which the package system replaces:
// in the target model the same fields live in the Module spec itself. It dies
// together with the ModuleConfig deprecation, along with the config block of
// fillModuleV2.

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// liveModuleConfigs maps every module config that is not being deleted onto
// the module it configures.
func (s *Syncer) liveModuleConfigs(ctx context.Context) (map[string]*v1alpha1.ModuleConfig, error) {
	list := new(v1alpha1.ModuleConfigList)
	if err := s.reader.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list module configs: %w", err)
	}

	configs := make(map[string]*v1alpha1.ModuleConfig, len(list.Items))

	for i := range list.Items {
		conf := &list.Items[i]

		// a config on its way out is the config controller's business, not the sync's
		if !conf.DeletionTimestamp.IsZero() {
			continue
		}

		configs[conf.Name] = conf
	}

	return configs, nil
}
