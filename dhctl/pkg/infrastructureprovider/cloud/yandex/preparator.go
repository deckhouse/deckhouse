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
	"errors"
	"fmt"
	"regexp"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	dhctljson "github.com/deckhouse/deckhouse/dhctl/pkg/util/json"
)

var prefixRegex = regexp.MustCompile("^([a-z]([-a-z0-9]{0,61}[a-z0-9])?)$")

type MetaConfigPreparator struct {
	validatePrefix bool
	// validateWithNATLayout
	// todo need migration for validate everywhere not only bootstrap
	validateWithNATLayout bool

	logger log.Logger
}

func NewMetaConfigPreparator(validatePrefix bool) *MetaConfigPreparator {
	return &MetaConfigPreparator{
		validatePrefix: validatePrefix,
		logger:         log.NewSilentLogger(),
	}
}

func (p *MetaConfigPreparator) WithLogger(logger log.Logger) *MetaConfigPreparator {
	if !govalue.IsNil(logger) {
		p.logger = logger
	}

	return p
}

func (p *MetaConfigPreparator) EnableValidateWithNATLayout() *MetaConfigPreparator {
	p.validateWithNATLayout = true
	return p
}

func (p *MetaConfigPreparator) Validate(_ context.Context, metaConfig *config.MetaConfig) error {
	if p.validatePrefix {
		prefix := metaConfig.ClusterPrefix
		if !prefixRegex.MatchString(prefix) {
			return fmt.Errorf("invalid prefix '%v' for provider '%v', prefix must match the pattern: %v", prefix, ProviderName, prefixRegex.String())
		}
	}

	if err := p.validateMasterNodeGroup(metaConfig); err != nil {
		return err
	}

	if err := p.validateNodeGroups(metaConfig); err != nil {
		return err
	}

	if err := p.validateWithNATInstanceLayout(metaConfig); err != nil {
		return err
	}

	return nil
}

func (p *MetaConfigPreparator) Prepare(_ context.Context, _ *config.MetaConfig) error {
	return nil
}

func (p *MetaConfigPreparator) validateMasterNodeGroup(metaConfig *config.MetaConfig) error {
	masterNodeGroup, err := dhctljson.UnmarshalToFromMessageMap[masterNodeGroupSpec](metaConfig.ProviderClusterConfig, "masterNodeGroup")
	if err != nil {
		return fmt.Errorf("Unable to unmarshal master node group from provider cluster configuration: %v", err)
	}

	if masterNodeGroup.Replicas > 0 &&
		len(masterNodeGroup.InstanceClass.ExternalIPAddresses) > 0 &&
		masterNodeGroup.Replicas > len(masterNodeGroup.InstanceClass.ExternalIPAddresses) {
		return fmt.Errorf("Number of masterNodeGroup.replicas should be equal to the length of masterNodeGroup.instanceClass.externalIPAddresses")
	}

	return nil
}

func (p *MetaConfigPreparator) validateNodeGroups(metaConfig *config.MetaConfig) error {
	yandexNodeGroups, err := dhctljson.UnmarshalToFromMessageMap[[]nodeGroupSpec](metaConfig.ProviderClusterConfig, "nodeGroups")
	if err != nil {
		if errors.Is(err, dhctljson.ErrNotFound) {
			p.logger.LogDebugLn("nodeGroups not found in provider cluster configuration. Skip validation.")
			return nil
		}

		return fmt.Errorf("Unable to unmarshal node groups from provider cluster configuration: %v", err)
	}

	for _, nodeGroup := range *yandexNodeGroups {
		if nodeGroup.Replicas > 0 &&
			len(nodeGroup.InstanceClass.ExternalIPAddresses) > 0 &&
			nodeGroup.Replicas > len(nodeGroup.InstanceClass.ExternalIPAddresses) {
			return fmt.Errorf(`number of nodeGroups["%s"].replicas should be equal to the length of nodeGroups["%s"].instanceClass.externalIPAddresses`, nodeGroup.Name, nodeGroup.Name)
		}
	}

	return nil
}

func (p *MetaConfigPreparator) validateWithNATInstanceLayout(metaConfig *config.MetaConfig) error {
	// layout was prepared with strcase.ToKebab before calling preparator
	if metaConfig.Layout != "with-nat-instance" {
		p.logger.LogDebugF("Skip validate WithNATInstance layout. Got layout %v\n", metaConfig.Layout)
		return nil
	}

	if !p.validateWithNATLayout {
		p.logger.LogDebugLn("Skip validate WithNATInstance layout. Validation disabled")
		return nil
	}

	spec, err := dhctljson.UnmarshalToFromMessageMap[withNatInstanceSpec](metaConfig.ProviderClusterConfig, "withNATInstance")
	if err != nil {
		return fmt.Errorf("Unable to unmarshal withNATInstance from provider cluster configuration: %v", err)
	}

	if spec.InternalSubnetCIDR == "" && spec.InternalSubnetID == "" {
		return errors.New("You should provide internalSubnetCIDR or internalSubnetID for withNATInstance")
	}

	return nil
}
