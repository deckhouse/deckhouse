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

package provider

import (
	"encoding/json"
	"fmt"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"k8s.io/utils/pointer"
)

type SettingsProviderConfig struct {
	// empty
	// reserve for future usage
}

type GetSettingsFunc func(config *config.MetaConfig, additionalProviderConfig SettingsProviderConfig) Settings

type Settings interface {
	Namespace() string
	CloudName() string
	Versions() []string
	DestinationBinary() string
	VmResourceType() string
	UseOpenTofu() bool
	InfrastructureVersion() string
}

type settingsSimple struct {
	NamespaceVal             *string   `json:"namespace,omitempty"`
	CloudNameVal             *string   `json:"cloudName,omitempty"`
	VersionVal               *string   `json:"version,omitempty"`
	VersionsVal              *[]string `json:"versions,omitempty"`
	DestinationBinaryVal     *string   `json:"destinationBinary,omitempty"`
	VmResourceTypeVal        *string   `json:"vmResourceType,omitempty"`
	UseOpenTofuVal           *bool     `json:"useOpentofu,omitempty"`
	InfrastructureVersionVal *string   `json:"infrastructureVersion,omitempty"`
}

func settingsSimpleFromMap(s any, terraformVersion string, openTofuVersion string) (*settingsSimple, error) {
	sJSON, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	settings := settingsSimple{}

	if err := json.Unmarshal(sJSON, &settings); err != nil {
		return nil, err
	}

	if err := settings.validate(false); err != nil {
		return nil, err
	}

	if settings.UseOpenTofu() {
		settings.InfrastructureVersionVal = pointer.StringPtr(openTofuVersion)
	} else {
		settings.InfrastructureVersionVal = pointer.StringPtr(terraformVersion)
	}

	return &settings, nil
}

func (s *settingsSimple) validate(strictInfraVersion bool) error {
	if s.NamespaceVal == nil {
		return fmt.Errorf("namespace is required")
	}

	if s.CloudNameVal == nil {
		return fmt.Errorf("cloudName is required")
	}

	if s.VersionVal == nil && s.VersionsVal == nil {
		return fmt.Errorf("version or versions is required")
	}

	if s.DestinationBinaryVal == nil {
		return fmt.Errorf("destinationBinary is required")
	}

	if s.VmResourceTypeVal == nil {
		return fmt.Errorf("vmResourceType is required")
	}

	if s.UseOpenTofuVal == nil {
		return fmt.Errorf("useOpentoufly is required")
	}

	if strictInfraVersion && s.InfrastructureVersionVal == nil {
		return fmt.Errorf("infrastructureVersion is required")
	}

	return nil
}

func (s *settingsSimple) Namespace() string {
	if s.NamespaceVal == nil {
		panic("namespace is required")
	}

	return *s.NamespaceVal
}

func (s *settingsSimple) CloudName() string {
	if s.CloudNameVal == nil {
		panic("cloudName is required")
	}

	return *s.CloudNameVal
}
func (s *settingsSimple) Versions() []string {
	var versions []string
	if s.VersionVal != nil {
		versions = []string{*s.VersionVal}
	} else if s.VersionsVal != nil {
		versions = *s.VersionsVal
	}

	if len(versions) == 0 {
		panic("versions is required")
	}

	return versions
}

func (s *settingsSimple) DestinationBinary() string {
	if s.DestinationBinaryVal == nil {
		panic("destinationBinary is required")
	}

	return *s.DestinationBinaryVal
}

func (s *settingsSimple) VmResourceType() string {
	if s.VmResourceTypeVal == nil {
		panic("vmResourceType is required")
	}

	return *s.VmResourceTypeVal
}

func (s *settingsSimple) UseOpenTofu() bool {
	if s.UseOpenTofuVal == nil {
		panic("useOpentoufly is required")
	}

	return *s.UseOpenTofuVal
}

func (s *settingsSimple) InfrastructureVersion() string {
	if s.InfrastructureVersionVal == nil {
		panic("infrastructureVersion is required")
	}

	return *s.InfrastructureVersionVal
}
