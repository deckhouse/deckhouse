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
func IsImmutableMaster(_ context.Context, metaConfig *config.MetaConfig) (bool, error) {
	if metaConfig == nil {
		return false, nil
	}

	if metaConfig.CloudProviderVars != nil {
		master := metaConfig.CloudProviderVars.NodeGroups[global.MasterNodeGroupName]
		systemType, _, _ := unstructured.NestedString(master, "spec", "systemType")
		return systemType == systemTypeImmutable, nil
	}

	systemType, err := masterSystemTypeFromResources(metaConfig.ResourcesYAML)
	if err != nil {
		return false, err
	}
	return systemType == systemTypeImmutable, nil
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
func masterSystemTypeFromResources(resourcesYAML string) (string, error) {
	for i, document := range libdhctlyaml.SplitYAML(resourcesYAML) {
		// Lenient, the way the rest of dhctl reads this stream: this runs on every
		// bootstrap before any gate, and a document it cannot read — a comment, a
		// stray fragment, a foreign resource — says nothing about the master.
		var obj map[string]any
		if err := yaml.Unmarshal([]byte(document), &obj); err != nil {
			continue
		}

		kind, _, _ := unstructured.NestedString(obj, "kind")
		apiVersion, _, _ := unstructured.NestedString(obj, "apiVersion")
		index := yamlvalidation.SchemaIndex{Kind: kind, Version: apiVersion}
		if index.Kind != nodeGroupKind || index.Group() != nodeGroupAPIGroup {
			continue
		}
		if name, _, _ := unstructured.NestedString(obj, "metadata", "name"); name != global.MasterNodeGroupName {
			continue
		}

		// The master group, and only it, is read strictly too: a duplicated key reads
		// differently depending on who parses it, and one of those readings is a master
		// NodeGroup nobody sees — taken for "not immutable" it sends the bootstrap over SSH.
		if err := yaml.UnmarshalStrict([]byte(document), &obj); err != nil {
			return "", fmt.Errorf("read the master NodeGroup in resource document %d: %w", i+1, err)
		}

		systemType, _, _ := unstructured.NestedString(obj, "spec", "systemType")
		return systemType, nil
	}
	return "", nil
}

// NodeGroupIsImmutable reports whether a NodeGroup read from the cluster asks for an
// immutable system. The cluster object is the source of truth: a bashible bootstrap
// Secret exists for an immutable group too, so its presence proves nothing.
func NodeGroupIsImmutable(ng *unstructured.Unstructured) bool {
	if ng == nil {
		return false
	}

	systemType, _, _ := unstructured.NestedString(ng.Object, "spec", "systemType")

	return systemType == systemTypeImmutable
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
