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

package preflight

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	dhctljson "github.com/deckhouse/deckhouse/dhctl/pkg/util/json"
)

type ConfigDeps struct {
	MetaConfig            *config.MetaConfig
	ValidatePrefix        bool
	ValidateWithNATLayout bool
}

type configCheck struct {
	deps ConfigDeps
}

type instanceClass struct {
	ExternalIPAddresses []string `json:"externalIPAddresses"`
}

type masterNodeGroupSpec struct {
	Replicas      int           `json:"replicas"`
	InstanceClass instanceClass `json:"instanceClass"`
}

type nodeGroupSpec struct {
	Name          string        `json:"name"`
	Replicas      int           `json:"replicas"`
	InstanceClass instanceClass `json:"instanceClass"`
}

type withNatInstanceSpec struct {
	InternalSubnetCIDR string `json:"internalSubnetCIDR,omitempty"`
	InternalSubnetID   string `json:"internalSubnetID,omitempty"`
}

const YandexConfigCheckName preflightnew.CheckName = "yandex-cloud-config"

var prefixRegex = regexp.MustCompile("^([a-z]([-a-z0-9]{0,61}[a-z0-9])?)$")

func (configCheck) Description() string {
	return "validate Yandex.Cloud provider configuration"
}

func (configCheck) Phase() preflightnew.Phase {
	return preflightnew.PhaseProviderConfigCheck
}

func (configCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.RetryPolicy{Attempts: 1}
}

func (c configCheck) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	if c.deps.MetaConfig == nil {
		return fmt.Errorf("meta config is nil")
	}

	if c.deps.ValidatePrefix {
		if err := c.validatePrefix(); err != nil {
			return err
		}
	}

	if err := c.validateMasterNodeGroup(); err != nil {
		return err
	}

	if err := c.validateNodeGroups(); err != nil {
		return err
	}

	return c.validateWithNATInstanceLayout()
}

func (c configCheck) validatePrefix() error {
	prefix := c.deps.MetaConfig.ClusterPrefix
	if prefixRegex.MatchString(prefix) {
		return nil
	}
	return fmt.Errorf("invalid prefix '%v' for provider 'yandex', prefix must match the pattern: %v", prefix, prefixRegex.String())
}

func (c configCheck) validateMasterNodeGroup() error {
	meta := c.deps.MetaConfig
	masterNodeGroup, err := dhctljson.UnmarshalToFromMessageMap[masterNodeGroupSpec](meta.ProviderClusterConfig, "masterNodeGroup")
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

func (c configCheck) validateNodeGroups() error {
	meta := c.deps.MetaConfig
	yandexNodeGroups, err := dhctljson.UnmarshalToFromMessageMap[[]nodeGroupSpec](meta.ProviderClusterConfig, "nodeGroups")
	if err != nil {
		if errors.Is(err, dhctljson.ErrNotFound) {
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

func (c configCheck) validateWithNATInstanceLayout() error {
	meta := c.deps.MetaConfig

	if meta.Layout != "with-nat-instance" || !c.deps.ValidateWithNATLayout {
		return nil
	}

	spec, err := dhctljson.UnmarshalToFromMessageMap[withNatInstanceSpec](meta.ProviderClusterConfig, "withNATInstance")
	if err != nil {
		return fmt.Errorf("Unable to unmarshal withNATInstance from provider cluster configuration: %v", err)
	}

	if spec.InternalSubnetCIDR == "" && spec.InternalSubnetID == "" {
		return errors.New("You should provide internalSubnetCIDR or internalSubnetID for withNATInstance")
	}

	return nil
}

func ConfigCheck(deps ConfigDeps) preflightnew.Check {
	check := configCheck{deps: deps}
	return preflightnew.Check{
		Name:        YandexConfigCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}

func NewSuite(deps ConfigDeps) preflightnew.Suite {
	return preflightnew.NewSuite(
		ConfigCheck(deps),
	)
}
