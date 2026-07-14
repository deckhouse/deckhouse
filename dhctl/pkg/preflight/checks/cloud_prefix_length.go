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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	dhctljson "github.com/deckhouse/deckhouse/dhctl/pkg/util/json"
)

const CloudDiskNameLengthCheckName preflight.CheckName = "cloud-disk-name-length"

const maxDiskNameLength = 63

func maxNodeIndex(replicas int) string {
	if replicas <= 1 {
		return "0"
	}
	return strconv.Itoa(replicas - 1)
}

func providerKubernetesDataDiskNames(prefix, nodeIndex string) []string {
	return []string{
		fmt.Sprintf("%s-kubernetes-data-%s", prefix, nodeIndex),
	}
}

func openstackDiskNames(prefix, nodeIndex string) []string {
	return []string{
		fmt.Sprintf("%s-kubernetes-data-%s", prefix, nodeIndex),
		fmt.Sprintf("%s-master-root-volume-%s", prefix, nodeIndex),
	}
}

func vcdDiskNames(prefix, nodeIndex string) []string {
	return []string{
		fmt.Sprintf("%s-master-%s-etcd-disk", prefix, nodeIndex),
	}
}

func providerMasterNodeDiskNames(prefix, nodeIndex string) []string {
	return []string{
		fmt.Sprintf("%s-master-%s-kubernetes-data", prefix, nodeIndex),
	}
}

func dvpMasterDiskNames(prefix, nodeIndex, hash string, withAdditional bool) []string {
	names := []string{
		fmt.Sprintf("%s-master-%s-%s", prefix, nodeIndex, hash),
		fmt.Sprintf("%s-master-kubernetes-data-%s-%s", prefix, nodeIndex, hash),
	}
	if withAdditional {
		names = append(names, fmt.Sprintf("%s-master-additional-disk-0-%s-%s", prefix, nodeIndex, hash))
	}
	return names
}

func dvpNodeGroupDiskNames(prefix, nodeGroup, nodeIndex, hash string, withAdditional bool) []string {
	names := []string{
		fmt.Sprintf("%s-%s-%s-%s", prefix, nodeGroup, nodeIndex, hash),
	}
	if withAdditional {
		names = append(names, fmt.Sprintf("%s-%s-additional-disk-0-%s-%s", prefix, nodeGroup, nodeIndex, hash))
	}
	return names
}

type dvpInstanceClass struct {
	AdditionalDisks []json.RawMessage `json:"additionalDisks,omitempty"`
}

type dvpMasterNodeGroup struct {
	InstanceClass dvpInstanceClass `json:"instanceClass"`
}

type dvpNodeGroup struct {
	Name          string           `json:"name"`
	Replicas      int              `json:"replicas"`
	InstanceClass dvpInstanceClass `json:"instanceClass"`
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

	if provider == "dvp" {
		return c.runDVP(prefix)
	}

	nodeIndex := maxNodeIndex(c.MetaConfig.MasterNodeGroupSpec.Replicas)

	var diskNames []string
	switch provider {
	case "aws", "azure", "gcp", "yandex", "huaweicloud", "vsphere":
		diskNames = providerKubernetesDataDiskNames(prefix, nodeIndex)
	case "openstack":
		diskNames = openstackDiskNames(prefix, nodeIndex)
	case "vcd":
		diskNames = vcdDiskNames(prefix, nodeIndex)
	case "zvirt", "dynamix":
		diskNames = providerMasterNodeDiskNames(prefix, nodeIndex)
	default:
		return nil
	}

	for _, diskName := range diskNames {
		if len(diskName) > maxDiskNameLength {
			return fmt.Errorf(
				"disk name %q exceeds %d characters (got %d); use a shorter cluster prefix",
				diskName, maxDiskNameLength, len(diskName),
			)
		}
	}

	return nil
}

func (c CloudDiskNameLengthCheck) runDVP(prefix string) error {
	masterIndex := maxNodeIndex(c.MetaConfig.MasterNodeGroupSpec.Replicas)
	for _, diskName := range dvpMasterDiskNames(prefix, masterIndex, "abcdef", c.dvpMasterHasAdditionalDisks()) {
		if len(diskName) > maxDiskNameLength {
			return fmt.Errorf(
				"disk name %q for node group %q exceeds %d characters (got %d); "+
					"use a shorter cluster prefix or node group name",
				diskName, "master", maxDiskNameLength, len(diskName),
			)
		}
	}

	for _, ng := range c.dvpNodeGroups() {
		hasAdditional := len(ng.InstanceClass.AdditionalDisks) > 0
		ngIndex := maxNodeIndex(ng.Replicas)
		for _, diskName := range dvpNodeGroupDiskNames(prefix, ng.Name, ngIndex, "abcdef", hasAdditional) {
			if len(diskName) > maxDiskNameLength {
				return fmt.Errorf(
					"disk name %q for node group %q exceeds %d characters (got %d); "+
						"use a shorter cluster prefix or node group name",
					diskName, ng.Name, maxDiskNameLength, len(diskName),
				)
			}
		}
	}

	return nil
}

func (c CloudDiskNameLengthCheck) dvpMasterHasAdditionalDisks() bool {
	master, err := dhctljson.UnmarshalToFromMessageMap[dvpMasterNodeGroup](
		c.MetaConfig.ProviderClusterConfig, "masterNodeGroup",
	)
	if err != nil {
		return false
	}
	return len(master.InstanceClass.AdditionalDisks) > 0
}

func (c CloudDiskNameLengthCheck) dvpNodeGroups() []dvpNodeGroup {
	groups, err := dhctljson.UnmarshalToFromMessageMap[[]dvpNodeGroup](
		c.MetaConfig.ProviderClusterConfig, "nodeGroups",
	)
	if err != nil {
		return nil
	}
	return *groups
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
