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
	"errors"
	"fmt"
	"regexp"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

var prefixRegex = regexp.MustCompile("^([a-z]([-a-z0-9]{0,61}[a-z0-9])?)$")

type MetaConfigPreparator struct {
	validatePrefix bool
	logger         log.Logger
}

func NewMetaConfigPreparator(validatePrefix bool, logger log.Logger) *MetaConfigPreparator {
	if govalue.IsNil(logger) {
		logger = log.NewSilentLogger()
	}
	return &MetaConfigPreparator{
		validatePrefix: validatePrefix,
		logger:         logger,
	}
}

func (p *MetaConfigPreparator) Validate(_ context.Context, input config.ProviderInput) error {
	if p.validatePrefix && !prefixRegex.MatchString(input.ClusterPrefix) {
		return fmt.Errorf("invalid prefix '%v' for provider '%v', prefix must match the pattern: %v", input.ClusterPrefix, ProviderName, prefixRegex.String())
	}

	if err := p.validateMasterNodeGroup(input); err != nil {
		return err
	}

	if err := p.validateNodeGroups(input); err != nil {
		return err
	}

	return p.validateWithNATInstanceLayout(input)
}

func (p *MetaConfigPreparator) Prepare(_ context.Context, _ config.ProviderInput) (proto.PrepareResult, error) {
	return proto.PrepareResult{}, nil
}

func (p *MetaConfigPreparator) validateMasterNodeGroup(input config.ProviderInput) error {
	raw, ok := input.ProviderClusterConfig["masterNodeGroup"]
	if !ok {
		return fmt.Errorf("Unable to unmarshal master node group from provider cluster configuration: key not found")
	}
	var masterNodeGroup masterNodeGroupSpec
	if err := json.Unmarshal(raw, &masterNodeGroup); err != nil {
		return fmt.Errorf("Unable to unmarshal master node group from provider cluster configuration: %v", err)
	}

	if masterNodeGroup.Replicas > 0 &&
		len(masterNodeGroup.InstanceClass.ExternalIPAddresses) > 0 &&
		masterNodeGroup.Replicas > len(masterNodeGroup.InstanceClass.ExternalIPAddresses) {
		return fmt.Errorf("Number of masterNodeGroup.replicas should be equal to the length of masterNodeGroup.instanceClass.externalIPAddresses")
	}

	return nil
}

func (p *MetaConfigPreparator) validateNodeGroups(input config.ProviderInput) error {
	raw, ok := input.ProviderClusterConfig["nodeGroups"]
	if !ok {
		p.logger.LogDebugLn("nodeGroups not found in provider cluster configuration. Skip validation.")
		return nil
	}
	var yandexNodeGroups []nodeGroupSpec
	if err := json.Unmarshal(raw, &yandexNodeGroups); err != nil {
		return fmt.Errorf("Unable to unmarshal node groups from provider cluster configuration: %v", err)
	}

	for _, nodeGroup := range yandexNodeGroups {
		if nodeGroup.Replicas > 0 &&
			len(nodeGroup.InstanceClass.ExternalIPAddresses) > 0 &&
			nodeGroup.Replicas > len(nodeGroup.InstanceClass.ExternalIPAddresses) {
			return fmt.Errorf(`number of nodeGroups["%s"].replicas should be equal to the length of nodeGroups["%s"].instanceClass.externalIPAddresses`, nodeGroup.Name, nodeGroup.Name)
		}
	}

	return nil
}

func (p *MetaConfigPreparator) validateWithNATInstanceLayout(input config.ProviderInput) error {
	if input.Layout != "with-nat-instance" {
		p.logger.LogDebugF("Skip validate WithNATInstance layout. Got layout %v\n", input.Layout)
		return nil
	}

	if input.Operation != proto.OperationBootstrap {
		p.logger.LogDebugLn("Skip validate WithNATInstance layout. Validation disabled")
		return nil
	}

	raw, ok := input.ProviderClusterConfig["withNATInstance"]
	if !ok {
		return fmt.Errorf("Unable to unmarshal withNATInstance from provider cluster configuration: key not found")
	}
	var spec withNatInstanceSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return fmt.Errorf("Unable to unmarshal withNATInstance from provider cluster configuration: %v", err)
	}

	if spec.InternalSubnetCIDR == "" && spec.InternalSubnetID == "" {
		return errors.New("You should provide internalSubnetCIDR or internalSubnetID for withNATInstance")
	}

	return nil
}
