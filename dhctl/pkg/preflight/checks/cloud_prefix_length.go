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
	"strconv"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
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

// dvpDiskNames follows DVP's own template: prefix, node group, the role, the index and a hash.
// DVP was missing from the switch entirely, so a prefix too long for it passed here and the
// cluster API rejected the disk during base infrastructure instead.
func dvpDiskNames(prefix, nodeIndex string) []string {
	return []string{
		fmt.Sprintf("%s-master-kubernetes-data-%s-00000000", prefix, nodeIndex),
	}
}

type CloudDiskNameLengthCheck struct {
	MetaConfig *config.MetaConfig
}

func (CloudDiskNameLengthCheck) Description() string {
	return "the cluster prefix keeps generated disk names within the length limit"
}

func (CloudDiskNameLengthCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (CloudDiskNameLengthCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c CloudDiskNameLengthCheck) Run(_ context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", fmt.Errorf("meta config is nil")
	}

	prefix := c.MetaConfig.ClusterPrefix
	provider := c.MetaConfig.ProviderName

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
	case "dvp":
		diskNames = dvpDiskNames(prefix, nodeIndex)
	default:
		return "", preflight.NotApplicable("the disk naming of provider %q is not known to this installer", provider)
	}

	longest := ""
	for _, diskName := range diskNames {
		if len(diskName) > len(longest) {
			longest = diskName
		}
	}

	if len(longest) > maxDiskNameLength {
		// The suffix is what the provider appends; what the operator controls is the prefix, so
		// the message says how many characters they have to give back.
		suffix := len(longest) - len(prefix)
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the disk names %s generates from the cluster prefix %q", provider, prefix),
			Observed: fmt.Sprintf("%q is %d characters, the limit is %d", longest, len(longest), maxDiskNameLength),
			Expected: fmt.Sprintf("every generated disk name within %d characters", maxDiskNameLength),
			Fix: fmt.Sprintf("shorten ClusterConfiguration.cloud.prefix (or the prefix in the %q ModuleConfig) "+
				"to at most %d characters; it is %d now, and the suffix %q takes %d",
				"global", maxDiskNameLength-suffix, len(prefix), longest[len(prefix):], suffix),
		})
	}

	return fmt.Sprintf("the longest %s disk name %q is %d of %d characters", provider, longest, len(longest), maxDiskNameLength), nil
}

func CloudDiskNameLength(metaConfig *config.MetaConfig) preflight.Check {
	check := CloudDiskNameLengthCheck{MetaConfig: metaConfig}
	return preflight.Check{
		Name:        CloudDiskNameLengthCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Cacheable:   true,
		Run:         check.Run,
	}
}
