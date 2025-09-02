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

package infrastructure

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	infra "github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/stretchr/testify/assert/yaml"
)

type Plan map[string]any

type DestructiveChangesReport struct {
	Changes *PlanDestructiveChanges
	hasMasterDestruction bool
}

type PlanDestructiveChanges struct {
	ResourcesDeleted   []ValueChange `json:"resources_deleted,omitempty"`
	ResourcesRecreated []ValueChange `json:"resourced_recreated,omitempty"`
	Provider           string        `json:"provider,omitempty"`
}

type ValueChange struct {
	CurrentValue interface{} `json:"current_value,omitempty"`
	NextValue    interface{} `json:"next_value,omitempty"`
	Type         string      `json:"type,omitempty"`
}

type TfPlan struct {
	ResourceChanges []ResourceChange `json:"resource_changes"`
}

type ResourceChange struct {
	Change       ChangeOp `json:"change"`
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	ProviderName string   `json:"provider_name,omitempty"`
}

type ChangeOp struct {
	Actions []string               `json:"actions"`
	Before  map[string]interface{} `json:"before,omitempty"`
	After   map[string]interface{} `json:"after,omitempty"`
}

// no in init function because config.InfrastructureVersions loaded dynamically
var cloudNameToUseOpenTofu map[string]struct{}

func getCloudNameToUseOpentofuMap(filename string) (map[string]struct{}, error) {
	if cloudNameToUseOpenTofu != nil {
		return cloudNameToUseOpenTofu, nil
	}

	infrastructureProviders := make(map[string]interface{})

	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Cannot read infrastructure versions file %s: %v", filename, err)
	}

	err = yaml.Unmarshal(file, &infrastructureProviders)
	if err != nil {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: %v", filename, err)
	}

	cloudNameToUseOpenTofu = make(map[string]struct{})

	for name, rawSettings := range infrastructureProviders {
		if slices.Contains([]string{"opentofu", "terraform"}, name) {
			log.DebugF("Found not provider name key %s\n", name)
			continue
		}

		settings, ok := rawSettings.(map[string]interface{})
		if !ok {
			log.DebugF("provider %s is not map\n", name)
			continue
		}

		useTofuRaw, ok := settings["useOpentofu"]
		if !ok {
			log.DebugF("Provider %s does not have useOpentofu. Skip provider.\n", name)
			continue
		}

		useTofu, ok := useTofuRaw.(bool)
		if !ok {
			return nil, fmt.Errorf("Provider %s none boolean have useOpentofu. Skip provider.\n", name)
		}

		if !useTofu {
			log.DebugF("provider %s does not use OpenTofu. Skip provider.\n", name)
			continue
		}

		cloudNameRaw, ok := settings["cloudName"]
		if !ok {
			return nil, fmt.Errorf("Provider %s does not have cloudName key", name)
		}

		cloudName, ok := cloudNameRaw.(string)
		if !ok {
			return nil, fmt.Errorf("Provider %s have none string cloudName key", name)
		}

		cloudNameToUseOpenTofu[strings.ToLower(cloudName)] = struct{}{}
	}
	return cloudNameToUseOpenTofu, nil
}

func NeedToUseOpentofu(metaConfig *config.MetaConfig) bool {
    useTofuMap, err := getCloudNameToUseOpentofuMap(infra.InfrastructureVersions)
    if err != nil {
        panic(fmt.Errorf("Cannot get use tofu map: %v", err))
    }

	provider := strings.ToLower(metaConfig.ProviderName)

	_, useTofu := useTofuMap[provider]

	return useTofu
}

func IsMasterInstanceDestructiveChanged(_ context.Context, rc ResourceChange, rm map[string]string) bool {
	for providerKey, vmType := range rm {
		// ex: providerKey = "yandex-cloud/yandex"
		// rc.ProviderName = "registry.terraform.io/yandex-cloud/yandex"
		if strings.Contains(rc.ProviderName, providerKey) {
			return rc.Type == vmType
		}
	}
	return false
}

func LoadProviderVMTypesFromYAML(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	resTypeMap := make(map[string]string)
	for _, v := range raw {

		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		ns, _ := m["namespace"].(string)
		typ, _ := m["type"].(string)
		vm, _ := m["vmResourceType"].(string)
		if ns != "" && typ != "" && vm != "" {
			resTypeMap[ns+"/"+typ] = vm
		}
	}
	return resTypeMap, nil
}