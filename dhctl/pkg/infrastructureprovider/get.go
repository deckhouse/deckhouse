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

package infrastructureprovider

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/tofu"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// no in init function because config.InfrastructureVersions loaded dynamically
var cloudNameToUseOpenTofu map[string]struct{}

func ExecutorProvider(metaConfig *config.MetaConfig) infrastructure.ExecutorProvider {
	if metaConfig == nil {
		panic("meta config must be provided")
	}

	if metaConfig.ProviderName == "" {
		return func(_ string, logger log.Logger) infrastructure.Executor {
			return infrastructure.NewDummyExecutor(logger)
		}
	}

	if NeedToUseOpentofu(metaConfig) {
		return func(w string, logger log.Logger) infrastructure.Executor {
			return tofu.NewExecutor(w, logger)
		}
	}

	return func(w string, logger log.Logger) infrastructure.Executor {
		return terraform.NewExecutor(w, logger)
	}
}

func NeedToUseOpentofu(metaConfig *config.MetaConfig) bool {
	useTofuMap, err := getCloudNameToUseOpentofuMap(config.InfrastructureVersions)
	if err != nil {
		panic(fmt.Errorf("Cannot get use tofu map: %v", err))
	}

	provider := strings.ToLower(metaConfig.ProviderName)

	_, useTofu := useTofuMap[provider]

	return useTofu
}

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
