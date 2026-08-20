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

package immutable

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	libdhctlyaml "github.com/deckhouse/lib-dhctl/pkg/yaml"
	yamlvalidation "github.com/deckhouse/lib-dhctl/pkg/yaml/validation"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

// IsImmutableMaster reports whether the master NodeGroup asks for an immutable
// system. A cloud bootstrap has the group parsed into CloudProviderVars; a
// static one has no cloud filler at all, so the documents are read here.
func IsImmutableMaster(_ context.Context, metaConfig *config.MetaConfig) bool {
	if metaConfig == nil {
		return false
	}

	if metaConfig.CloudProviderVars != nil {
		master := metaConfig.CloudProviderVars.NodeGroups[global.MasterNodeGroupName]
		if systemType, _, _ := unstructured.NestedString(master, "spec", "systemType"); systemType == systemTypeImmutable {
			return true
		}
	}

	return masterSystemTypeFromResources(metaConfig.ResourcesYAML) == systemTypeImmutable
}

// The group the master NodeGroup must belong to. Matched the way the other
// walks over this same stream do (pkg/config/cloud_provider_resources.go):
// without it a foreign NodeGroup named master masks the real one.
const (
	nodeGroupKind     = "NodeGroup"
	nodeGroupAPIGroup = "deckhouse.io"
)

// masterSystemTypeFromResources reads spec.systemType of the master NodeGroup
// straight from the documents. ParseResourcesYAML cannot answer this: it keeps
// CloudPermanent groups only, and a static master group is not one.
func masterSystemTypeFromResources(resourcesYAML string) string {
	for _, document := range libdhctlyaml.SplitYAML(resourcesYAML) {
		index, err := yamlvalidation.ParseIndex(strings.NewReader(document))
		// A document this fails on is not ours to judge: it stays in the
		// resources, where the existing validation reports it.
		if err != nil {
			continue
		}
		if index.Kind != nodeGroupKind || index.Group() != nodeGroupAPIGroup {
			continue
		}

		var obj map[string]any
		if err := yaml.Unmarshal([]byte(document), &obj); err != nil {
			continue
		}
		if name, _, _ := unstructured.NestedString(obj, "metadata", "name"); name != global.MasterNodeGroupName {
			continue
		}

		systemType, _, _ := unstructured.NestedString(obj, "spec", "systemType")
		return systemType
	}
	return ""
}

// ValidateInputs refuses the combinations that leave the bootstrap with a node
// it cannot talk to: the address comes from the BaseInfra phase, which reports
// nothing outside a cloud, and naming the machines closes exactly that hole.
func ValidateInputs(_ context.Context, metaConfig *config.MetaConfig, hosts map[string]string) error {
	if metaConfig.ClusterType == config.CloudClusterType {
		if len(hosts) > 0 {
			return errors.New(
				"--master-host names the machines of a static cluster, and here the cloud infrastructure reports the addresses itself: " +
					"drop the flag, or set clusterType to Static")
		}
		return nil
	}

	if len(hosts) == 0 {
		return fmt.Errorf(
			"the master NodeGroup asks for systemType %q and this is a %q cluster, so nothing reports where the machines are: "+
				"name each of them with --master-host <node-name>=<address>",
			systemTypeImmutable, metaConfig.ClusterType)
	}

	return nil
}

// ParseHosts turns the repeated --master-host values into node name to address.
// Both halves are required: the name is what the node registers as, and a
// static cluster has no prefix to derive it from.
func ParseHosts(raw []string) (map[string]string, error) {
	hosts := make(map[string]string, len(raw))

	for _, pair := range raw {
		name, address := ParseHost(pair)
		if name == "" || address == "" {
			return nil, fmt.Errorf("--master-host %q is not <node-name>=<address>", pair)
		}
		if previous, taken := hosts[name]; taken {
			return nil, fmt.Errorf("--master-host names %s twice: %s and %s", name, previous, address)
		}
		hosts[name] = address
	}

	return hosts, nil
}

// ParseHost splits one --master-host value into node name and address. The only
// place the flag is normalised: a name read anywhere else has to be the key
// ParseHosts stored the address under. Empty halves mean the value is not a pair.
func ParseHost(pair string) (string, string) {
	// Both halves are trimmed: kingpin splits an envar on newlines and trims only
	// the trailing one, and the spaces an operator puts around the "=" belong to
	// neither the node name nor the address.
	name, address, _ := strings.Cut(pair, "=")
	return strings.TrimSpace(name), strings.TrimSpace(address)
}
