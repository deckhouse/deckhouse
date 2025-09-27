// Copyright 2025 Flant JSC
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

package yandex

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

var prefixRegex = regexp.MustCompile("^([a-z]([-a-z0-9]{0,61}[a-z0-9])?)$")

type MetaConfigPreparator struct {
	validatePrefix bool
}

func NewMetaConfigPreparator(validatePrefix bool) *MetaConfigPreparator {
	return &MetaConfigPreparator{
		validatePrefix: validatePrefix,
	}
}

func (p MetaConfigPreparator) Validate(_ context.Context, metaConfig *config.MetaConfig) error {
	if p.validatePrefix {
		prefix := metaConfig.ClusterPrefix
		if !prefixRegex.MatchString(prefix) {
			return fmt.Errorf("invalid prefix '%v' for provider '%v', prefix must match the pattern: %v", prefix, ProviderName, prefixRegex.String())
		}
	}

	var masterNodeGroup masterNodeGroupSpec
	if err := json.Unmarshal(metaConfig.ProviderClusterConfig["masterNodeGroup"], &masterNodeGroup); err != nil {
		return fmt.Errorf("unable to unmarshal master node group from provider cluster configuration: %v", err)
	}

	if masterNodeGroup.Replicas > 0 &&
		len(masterNodeGroup.InstanceClass.ExternalIPAddresses) > 0 &&
		masterNodeGroup.Replicas > len(masterNodeGroup.InstanceClass.ExternalIPAddresses) {
		return fmt.Errorf("number of masterNodeGroup.replicas should be equal to the length of masterNodeGroup.instanceClass.externalIPAddresses")
	}

	nodeGroups, ok := metaConfig.ProviderClusterConfig["nodeGroups"]
	if ok {
		var yandexNodeGroups []nodeGroupSpec
		if err := json.Unmarshal(nodeGroups, &yandexNodeGroups); err != nil {
			return fmt.Errorf("unable to unmarshal node groups from provider cluster configuration: %v", err)
		}

		for _, nodeGroup := range yandexNodeGroups {
			if nodeGroup.Replicas > 0 &&
				len(nodeGroup.InstanceClass.ExternalIPAddresses) > 0 &&
				nodeGroup.Replicas > len(nodeGroup.InstanceClass.ExternalIPAddresses) {
				return fmt.Errorf(`number of nodeGroups["%s"].replicas should be equal to the length of nodeGroups["%s"].instanceClass.externalIPAddresses`, nodeGroup.Name, nodeGroup.Name)
			}
		}
	}

	return nil
}

func (p MetaConfigPreparator) Prepare(_ context.Context, _ *config.MetaConfig) error {
	return nil
}
