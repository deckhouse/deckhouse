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

const CloudPrefixLengthCheckName preflight.CheckName = "cloud-prefix-length"

// maxResourceNameLength is the Kubernetes DNS label limit.
const maxResourceNameLength = 63

// providerSuffixOverhead maps each provider (lowercase) to the length
// of the longest resource-name suffix appended after the prefix.
//
// DVP:         {prefix}-master-additional-disk-0-0-abcdef  → 37
// Zvirt:       {prefix}-master-0-kubernetes-data            → 26
// Dynamix:     {prefix}-master-0-kubernetes-data            → 26
// VCD:         {prefix}-master-0-etcd-disk                  → 21
// AWS:         {prefix}-kubernetes-data-0                   → 19
// Azure:       {prefix}-kubernetes-data-0                   → 19
// GCP:         {prefix}-kubernetes-data-0                   → 19
// Yandex:      {prefix}-kubernetes-data-0                   → 19
// OpenStack:   {prefix}-kubernetes-data-0                   → 19
// Huaweicloud: {prefix}-kubernetes-data-0                   → 19
// vSphere:     {prefix}-master-0                            → 9
var providerSuffixOverhead = map[string]int{
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

// defaultSuffixOverhead is used for unknown providers (worst case — DVP).
const defaultSuffixOverhead = 37

type CloudPrefixLengthCheck struct {
	MetaConfig *config.MetaConfig
}

func (CloudPrefixLengthCheck) Description() string {
	return "validate cluster prefix length for cloud provider"
}

func (CloudPrefixLengthCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (CloudPrefixLengthCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.RetryPolicy{Attempts: 1}
}

func (c CloudPrefixLengthCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil {
		return fmt.Errorf("meta config is nil")
	}

	prefix := c.MetaConfig.ClusterPrefix
	provider := c.MetaConfig.ProviderName

	overhead, ok := providerSuffixOverhead[provider]
	if !ok {
		overhead = defaultSuffixOverhead
	}

	maxPrefixLen := maxResourceNameLength - overhead
	if len(prefix) > maxPrefixLen {
		return fmt.Errorf(
			"cluster prefix %q is too long for provider %q: "+
				"length %d exceeds maximum %d "+
				"(resource names must be no more than %d characters; "+
				"longest suffix for this provider is %d characters)",
			prefix, provider,
			len(prefix), maxPrefixLen,
			maxResourceNameLength, overhead,
		)
	}

	return nil
}

func CloudPrefixLength(metaConfig *config.MetaConfig) preflight.Check {
	check := CloudPrefixLengthCheck{MetaConfig: metaConfig}
	return preflight.Check{
		Name:        CloudPrefixLengthCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
