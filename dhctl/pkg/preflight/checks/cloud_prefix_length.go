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
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

var yamlDocSeparator = regexp.MustCompile(`(?m)^---\s*$`)

const CloudDiskNameLengthCheckName preflight.CheckName = "cloud-disk-name-length"

const maxDiskNameLength = 63

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
	seen := map[string]struct{}{"master": {}}
	names := []string{"master"}

	for _, ng := range c.MetaConfig.TerraNodeGroupSpecs {
		if ng.Name != "" {
			if _, ok := seen[ng.Name]; !ok {
				seen[ng.Name] = struct{}{}
				names = append(names, ng.Name)
			}
		}
	}

	for _, name := range parseNodeGroupNamesFromResources(c.MetaConfig.ResourcesYAML) {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}

	return names
}

func parseNodeGroupNamesFromResources(resourcesYAML string) []string {
	if resourcesYAML == "" {
		return nil
	}

	type resource struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
	}

	var names []string
	docs := yamlDocSeparator.Split(resourcesYAML, -1)
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		var r resource
		if err := yaml.Unmarshal([]byte(doc), &r); err != nil {
			continue
		}
		if r.Kind == "NodeGroup" && r.Metadata.Name != "" {
			names = append(names, r.Metadata.Name)
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
