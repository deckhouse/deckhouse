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
	"fmt"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
	"os"
	"slices"
	"sync"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var infraVersionKeys = []string{"opentofu", "terraform"}

type settingsStore map[string]Settings

func loadTerraformVersionFileSettings(filename string) (settingsStore, error) {
	infrastructureProviders := make(map[string]interface{})

	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Cannot read infrastructure versions file %s: %v", filename, err)
	}

	err = yaml.Unmarshal(file, &infrastructureProviders)
	if err != nil {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: %v", filename, err)
	}

	terraformVersion, tofuVersion := "", ""

	for name, rawSettings := range infrastructureProviders {
		var ok bool
		switch name {
		case infraVersionKeys[0]:
			tofuVersion, ok = rawSettings.(string)
			if !ok {
				return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: wrong type for OpenTofu version setting", name)
			}
			log.DebugF("Found opentofu version: %s", tofuVersion)
		case infraVersionKeys[1]:
			terraformVersion, ok = rawSettings.(string)
			if !ok {
				return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: wrong type for Terraform version setting", name)
			}
			log.DebugF("Found terraform version: %s", terraformVersion)
		}
	}

	if terraformVersion == "" {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: missing terraform version", filename)
	}

	if tofuVersion == "" {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: missing terraform version", filename)
	}

	res := make(settingsStore)

	for name, rawSettings := range infrastructureProviders {
		if slices.Contains(infraVersionKeys, name) {
			log.DebugF("Found not provider name key %s\n", name)
			continue
		}

		settings, err := settingsSimpleFromMap(rawSettings, terraformVersion, tofuVersion)
		if err != nil {
			return nil, fmt.Errorf("Cannot unmarshal infrastructure settings for provider %s: %v", name, err)
		}

		res[settings.CloudName()] = settings
	}

	return res, nil
}

func candiTerraformVersionFileSettingsGetter(providerName string) Settings {
	store := sync.OnceValue[settingsStore](func() settingsStore {
		file := infrastructure.GetInfrastructureVersions()

		res, err := loadTerraformVersionFileSettings(file)
		if err != nil {
			panic(fmt.Errorf("Cannot read infrastructure versions file %s: %v", file, err))
		}

		return res
	})

	settings, ok := store()[providerName]
	if !ok {
		panic(fmt.Errorf("Settings not found for provider %s", providerName))
	}

	return settings
}
