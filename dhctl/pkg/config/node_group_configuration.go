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

package config

import (
	"context"
	"fmt"
	"strings"

	dhctlyaml "github.com/deckhouse/lib-dhctl/pkg/yaml"
	yamlvalidation "github.com/deckhouse/lib-dhctl/pkg/yaml/validation"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func ParseNodeGroupConfigurations(ctx context.Context, resourcesYAML string) ([]deckhousev1alpha1.NodeGroupConfiguration, error) {
	docs := dhctlyaml.SplitYAML(resourcesYAML)
	nodeGroupConfigurations := make([]deckhousev1alpha1.NodeGroupConfiguration, 0)

	for _, doc := range docs {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		if strings.TrimSpace(doc) == "" {
			continue
		}

		index, err := yamlvalidation.ParseIndex(strings.NewReader(doc), yamlvalidation.ParseIndexWithoutCheckValid())
		if err != nil {
			log.DebugF("Skip NodeGroupConfiguration probe for doc: %v\n", err)
			continue
		}

		if !isNodeGroupConfigurationIndex(index) {
			continue
		}

		nodeGroupConfiguration, err := dhctlyaml.UnmarshalString[deckhousev1alpha1.NodeGroupConfiguration](doc)
		if err != nil {
			return nil, fmt.Errorf("unmarshal NodeGroupConfiguration: %w", err)
		}

		err = validateNodeGroupConfiguration(nodeGroupConfiguration)
		if err != nil {
			return nil, err
		}

		nodeGroupConfigurations = append(nodeGroupConfigurations, nodeGroupConfiguration)
	}

	return nodeGroupConfigurations, nil
}

func isNodeGroupConfigurationIndex(index *yamlvalidation.SchemaIndex) bool {
	if index == nil {
		return false
	}

	return index.Kind == "NodeGroupConfiguration" && index.Version == "deckhouse.io/v1alpha1"
}

func validateNodeGroupConfiguration(nodeGroupConfiguration deckhousev1alpha1.NodeGroupConfiguration) error {
	if nodeGroupConfiguration.Name == "" {
		return fmt.Errorf("NodeGroupConfiguration metadata.name is required")
	}

	if len(nodeGroupConfiguration.Spec.Bundles) == 0 {
		return fmt.Errorf("NodeGroupConfiguration %q spec.bundles is required", nodeGroupConfiguration.Name)
	}

	return nil
}
