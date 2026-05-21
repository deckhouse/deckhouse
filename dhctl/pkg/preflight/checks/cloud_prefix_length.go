// Copyright 2026 Flant JSC
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

package checks

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

const CloudDiskNameLengthCheckName preflight.CheckName = "cloud-disk-name-length"

const maxDiskNameLength = 63

var providerDiskNameOverhead = map[string]int{
	"dvp":         37,
	"zvirt":       26,
	"dynamix":     26,
	"vcd":         21,
	"aws":         19,
	"azure":       19,
	"gcp":         19,
	"yandex":      19,
	"openstack":   19,
	"huaweicloud": 19,
	"vsphere":     9,
}

type CloudDiskNameLengthCheck struct {
	MetaConfig *config.MetaConfig
}

func (CloudDiskNameLengthCheck) Description() string {
	return "validate that cluster prefix does not cause disk names to exceed the length limit"
}

func (CloudDiskNameLengthCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (CloudDiskNameLengthCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.RetryPolicy{Attempts: 1}
}

func (c CloudDiskNameLengthCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil {
		return fmt.Errorf("meta config is nil")
	}

	prefix := c.MetaConfig.ClusterPrefix
	provider := c.MetaConfig.ProviderName

	overhead := providerDiskNameOverhead[provider]

	maxPrefixLen := maxDiskNameLength - overhead
	if len(prefix) > maxPrefixLen {
		return fmt.Errorf(
			"cluster prefix %q is too long for provider %q: "+
				"length %d exceeds maximum %d "+
				"(disk names must be no more than %d characters; "+
				"longest disk name suffix for this provider is %d characters)",
			prefix, provider,
			len(prefix), maxPrefixLen,
			maxDiskNameLength, overhead,
		)
	}

	return nil
}

func CloudDiskNameLength(metaConfig *config.MetaConfig) preflight.Check {
	check := CloudDiskNameLengthCheck{MetaConfig: metaConfig}
	return preflight.Check{
		Name:        CloudDiskNameLengthCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
