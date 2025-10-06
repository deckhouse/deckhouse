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

package fsprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	opentofuKey  = "opentofu"
	terraformKey = "terraform"
)

type (
	settingsStore map[string]settings.ProviderSettings
	loader        func(logger log.Logger, infraVersionsFile string) (settingsStore, error)
)

type SettingsProvider struct {
	initError error

	m     sync.Mutex
	store settingsStore
}

var (
	fileSettingsStoreMutex sync.Mutex
	fileToSettingsStore    = make(map[string]settingsStore)
)

func loadOrGetStore(logger log.Logger, infraVersionsFile string) (settingsStore, error) {
	fileSettingsStoreMutex.Lock()
	defer fileSettingsStoreMutex.Unlock()

	store, ok := fileToSettingsStore[infraVersionsFile]
	if ok {
		logger.LogDebugF("Providers settings store for terraform versions file %s loaded from cache\n", infraVersionsFile)
		return store, nil
	}

	store, err := loadTerraformVersionFileSettings(infraVersionsFile, logger)
	if err != nil {
		return nil, err
	}

	fileToSettingsStore[infraVersionsFile] = store

	logger.LogDebugF("Providers settings store for terraform versions file %s loaded from file and add to cache\n", infraVersionsFile)

	return store, nil
}

func newSettingsProvider(logger log.Logger, infraVersionsFile string, loader loader) *SettingsProvider {
	store, err := loader(logger, infraVersionsFile)
	if err != nil {
		return &SettingsProvider{
			initError: err,
		}
	}

	return &SettingsProvider{
		store:     store,
		initError: nil,
	}
}

func (p *SettingsProvider) GetSettings(_ context.Context, provider string, _ cloud.ProviderAdditionalParams) (settings.ProviderSettings, error) {
	if p.initError != nil {
		return nil, p.initError
	}

	p.m.Lock()
	defer p.m.Unlock()

	set, ok := p.store[provider]
	if !ok {
		return nil, fmt.Errorf("CloudProviderSettings not found for provider %s", provider)
	}

	return set, nil
}

func simpleFromMap(s any, terraformVersion string, openTofuVersion string) (*settings.Simple, error) {
	sJSON, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	set := settings.Simple{}

	if err := json.Unmarshal(sJSON, &set); err != nil {
		return nil, err
	}

	if err := set.Validate(false); err != nil {
		return nil, err
	}

	if set.UseOpenTofu() {
		set.InfrastructureVersionVal = pointer.String(openTofuVersion)
	} else {
		set.InfrastructureVersionVal = pointer.String(terraformVersion)
	}

	set.CloudNameVal = pointer.String(strings.ToLower(*set.CloudNameVal))

	return &set, nil
}

func loadTerraformVersionFileSettings(filename string, logger log.Logger) (settingsStore, error) {
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
		case opentofuKey:
			tofuVersion, ok = rawSettings.(string)
			if !ok {
				return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: wrong type for OpenTofu version setting", name)
			}
			logger.LogDebugF("Found opentofu version: %s\n", tofuVersion)
		case terraformKey:
			terraformVersion, ok = rawSettings.(string)
			if !ok {
				return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: wrong type for Terraform version setting", name)
			}
			logger.LogDebugF("Found terraform version: %s\n", terraformVersion)
		}
	}

	if terraformVersion == "" {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: missing terraform version", filename)
	}

	if tofuVersion == "" {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: missing opentofu version", filename)
	}

	res := make(settingsStore)

	var noneProviderKeys = map[string]struct{}{
		opentofuKey:  {},
		terraformKey: {},
	}

	for name, rawSettings := range infrastructureProviders {
		if _, ok := noneProviderKeys[name]; ok {
			logger.LogDebugF("Found not provider name key %s\n", name)
			continue
		}

		set, err := simpleFromMap(rawSettings, terraformVersion, tofuVersion)
		if err != nil {
			return nil, fmt.Errorf("Cannot unmarshal infrastructure settings for provider %s: %v", name, err)
		}

		cloudName := strings.ToLower(set.CloudName())

		logger.LogDebugF("Found provider settings for %s: %s\n", name, cloudName)

		res[cloudName] = set
	}

	return res, nil
}
