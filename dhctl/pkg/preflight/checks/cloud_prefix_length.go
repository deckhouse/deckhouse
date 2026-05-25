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
	"errors"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	dhctljson "github.com/deckhouse/deckhouse/dhctl/pkg/util/json"
)

const CloudDiskNameLengthCheckName preflight.CheckName = "cloud-disk-name-length"

const maxDiskNameLength = 63

var providerDiskSuffixParts = map[string][][]string{
	"aws":         {{"kubernetes-data", "0"}},
	"azure":       {{"kubernetes-data", "0"}},
	"gcp":         {{"kubernetes-data", "0"}},
	"yandex":      {{"kubernetes-data", "0"}},
	"openstack":   {{"kubernetes-data", "0"}, {"master-root-volume", "0"}},
	"huaweicloud": {{"kubernetes-data", "0"}},
	"vsphere":     {{"kubernetes-data", "0"}},
	"vcd":         {{"master", "0", "etcd-disk"}},
	"zvirt":       {{"master", "0", "kubernetes-data"}},
	"dynamix":     {{"master", "0", "kubernetes-data"}},
}

var (
	dvpMasterDisks             = [][]string{{"0", "abcdef"}, {"kubernetes-data", "0", "abcdef"}}
	dvpMasterAdditionalDisk    = []string{"additional-disk", "0", "0", "abcdef"}
	dvpNodeGroupDisks          = [][]string{{"0", "abcdef"}}
	dvpNodeGroupAdditionalDisk = []string{"additional-disk", "0", "0", "abcdef"}
)

type dvpInstanceClass struct {
	AdditionalDisks []json.RawMessage `json:"additionalDisks,omitempty"`
}

type dvpMasterNodeGroup struct {
	InstanceClass dvpInstanceClass `json:"instanceClass"`
}

type dvpNodeGroup struct {
	Name          string           `json:"name"`
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

	disks, ok := providerDiskSuffixParts[provider]
	if !ok {
		return nil
	}

	for _, suffixParts := range disks {
		diskName := prefix + "-" + strings.Join(suffixParts, "-")
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
	masterDisks := dvpMasterDisks
	if c.masterHasAdditionalDisks() {
		masterDisks = append(masterDisks, dvpMasterAdditionalDisk)
	}
	for _, suffixParts := range masterDisks {
		if err := checkDVPDiskName(prefix, "master", suffixParts); err != nil {
			return err
		}
	}

	for _, ng := range c.dvpNodeGroups() {
		ngDisks := dvpNodeGroupDisks
		if len(ng.InstanceClass.AdditionalDisks) > 0 {
			ngDisks = append(ngDisks, dvpNodeGroupAdditionalDisk)
		}
		for _, suffixParts := range ngDisks {
			if err := checkDVPDiskName(prefix, ng.Name, suffixParts); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkDVPDiskName(prefix, nodeGroup string, suffixParts []string) error {
	diskName := prefix + "-" + nodeGroup + "-" + strings.Join(suffixParts, "-")
	if len(diskName) > maxDiskNameLength {
		return fmt.Errorf(
			"disk name %q for node group %q exceeds %d characters (got %d); "+
				"use a shorter cluster prefix or node group name",
			diskName, nodeGroup, maxDiskNameLength, len(diskName),
		)
	}
	return nil
}

func (c CloudDiskNameLengthCheck) masterHasAdditionalDisks() bool {
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
		if errors.Is(err, dhctljson.ErrNotFound) {
			return nil
		}
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
