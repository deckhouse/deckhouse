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

package provider

import (
	"fmt"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
)

type isVMChecker func(rc plan.ResourceChange) bool

type cachedSettings struct {
	m               sync.Mutex
	providerUseTofu map[string]struct{}
	// isProviderVm "yandex-cloud/yandex"
	providerVmChecker map[string]isVMChecker
}

func (v *cachedSettings) NeedToUseTofu(metaConfig *config.MetaConfig) bool {
	v.m.Lock()
	defer v.m.Unlock()

	provider := strings.ToLower(metaConfig.ProviderName)

	_, useTofu := v.providerUseTofu[provider]

	return useTofu
}

func (v *cachedSettings) IsVMChange(rc plan.ResourceChange) bool {
	v.m.Lock()
	defer v.m.Unlock()

	check, ok := v.providerVmChecker[rc.Type]

	if !ok {
		panic(fmt.Errorf("vm checker for provider %s not found", rc.Type))
	}

	return check(rc)
}
