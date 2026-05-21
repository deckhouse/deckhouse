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
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

const CloudDiskNameLengthCheckName preflight.CheckName = "cloud-disk-name-length"

const maxDiskNameLength = 63

// providerDiskSuffixParts contains the fixed parts of the longest disk name suffix
// for each provider, excluding prefix and node_group.
// The parts are joined with "-" to form the suffix.
//
// Providers where node_group is part of the disk name (variable):
//   DVP: {prefix}-{node_group}-additional-disk-{disk_index}-{node_index}-{hash}
//
// Providers where node_group is hardcoded as "master":
//   Zvirt:   {prefix}-master-{nodeIndex}-kubernetes-data
//   Dynamix: {prefix}-master-{nodeIndex}-kubernetes-data
//   VCD:     {prefix}-master-{nodeIndex}-etcd-disk
//
// Providers without node_group in disk name:
//   AWS, Azure, GCP, Yandex, OpenStack, Huaweicloud: {prefix}-kubernetes-data-{index}
//   vSphere: uses clusterUUID, not prefix
var providerDiskSuffixParts = map[string][]string{
	"dvp":         {"additional-disk", "0", "0", "abcdef"},
	"zvirt":       {"master", "0", "kubernetes-data"},
	"dynamix":     {"master", "0", "kubernetes-data"},
	"vcd":         {"master", "0", "etcd-disk"},
	"aws":         {"kubernetes-data", "0"},
	"azure":       {"kubernetes-data", "0"},
	"gcp":         {"kubernetes-data", "0"},
	"yandex":      {"kubernetes-data", "0"},
	"openstack":   {"kubernetes-data", "0"},
	"huaweicloud": {"kubernetes-data", "0"},
	"vsphere":     {"master", "0"},
}

var providersWithNodeGroupInDiskName = map[string]bool{
	"dvp": true,
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

	suffixParts, ok := providerDiskSuffixParts[provider]
	if !ok {
		return nil
	}

	if providersWithNodeGroupInDiskName[provider] {
		nodeGroups := c.collectNodeGroupNames()
		for _, ng := range nodeGroups {
			parts := append([]string{ng}, suffixParts...)
			diskName := prefix + "-" + strings.Join(parts, "-")
			if len(diskName) > maxDiskNameLength {
				return fmt.Errorf(
					"disk name %q for node group %q exceeds %d characters (got %d); "+
						"use a shorter cluster prefix or node group name",
					diskName, ng, maxDiskNameLength, len(diskName),
				)
			}
		}
	} else {
		diskName := prefix + "-" + strings.Join(suffixParts, "-")
		if len(diskName) > maxDiskNameLength {
			return fmt.Errorf(
				"disk name %q exceeds %d characters (got %d); "+
					"use a shorter cluster prefix",
				diskName, maxDiskNameLength, len(diskName),
			)
		}
	}

	return nil
}

func (c CloudDiskNameLengthCheck) collectNodeGroupNames() []string {
	names := []string{"master"}
	for _, ng := range c.MetaConfig.TerraNodeGroupSpecs {
		if ng.Name != "" {
			names = append(names, ng.Name)
		}
	}
	return names
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
