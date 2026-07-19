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
	"path/filepath"
	"strings"
	"sync"

	"sigs.k8s.io/yaml"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
)

const (
	opentofuKey  = "opentofu"
	terraformKey = "terraform"
)

type (
	settingsStore map[string]settings.ProviderSettings
	loader        func(ctx context.Context, infraVersionsFile, downloadDir string) (settingsStore, error)
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

func loadOrGetStore(ctx context.Context, infraVersionsFile, downloadDir string) (settingsStore, error) {
	fileSettingsStoreMutex.Lock()
	defer fileSettingsStoreMutex.Unlock()

	cacheKey := infraVersionsFile + "\x00" + downloadDir

	store, ok := fileToSettingsStore[cacheKey]
	if ok {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Providers settings store for terraform versions file %s loaded from cache", infraVersionsFile))
		return store, nil
	}

	store, err := loadTerraformVersionFileSettings(ctx, infraVersionsFile)
	if err != nil {
		return nil, err
	}

	if err := mergeBundleSettings(ctx, store, downloadDir); err != nil {
		return nil, err
	}

	fileToSettingsStore[cacheKey] = store

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Providers settings store for terraform versions file %s loaded from file and added to cache", infraVersionsFile))

	return store, nil
}

func newSettingsProvider(ctx context.Context, infraVersionsFile, downloadDir string, loader loader) *SettingsProvider {
	store, err := loader(ctx, infraVersionsFile, downloadDir)
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

func simpleFromMap(s any, terraformVersion, openTofuVersion string) (*settings.Simple, error) {
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
		set.InfrastructureVersionVal = new(openTofuVersion)
	} else {
		set.InfrastructureVersionVal = new(terraformVersion)
	}

	set.CloudNameVal = new(strings.ToLower(*set.CloudNameVal))

	return &set, nil
}

func loadTerraformVersionFileSettings(ctx context.Context, filename string) (settingsStore, error) {
	infrastructureProviders := make(map[string]any)

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
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found opentofu version: %s", tofuVersion))
		case terraformKey:
			terraformVersion, ok = rawSettings.(string)
			if !ok {
				return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: wrong type for Terraform version setting", name)
			}
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found terraform version: %s", terraformVersion))
		}
	}

	if terraformVersion == "" {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: missing terraform version", filename)
	}

	if tofuVersion == "" {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: missing opentofu version", filename)
	}

	res := make(settingsStore)

	noneProviderKeys := map[string]struct{}{
		opentofuKey:  {},
		terraformKey: {},
	}

	for name, rawSettings := range infrastructureProviders {
		if _, ok := noneProviderKeys[name]; ok {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found non-provider-name key %s", name))
			continue
		}

		set, err := simpleFromMap(rawSettings, terraformVersion, tofuVersion)
		if err != nil {
			return nil, fmt.Errorf("Cannot unmarshal infrastructure settings for provider %s: %v", name, err)
		}

		cloudName := strings.ToLower(set.CloudName())

		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found provider settings for %s: %s", name, cloudName))

		res[cloudName] = set
	}

	planRule, err := loadPlanRules(filename)
	if err != nil {
		return nil, err
	}
	if planRule != nil && len(res) != 1 {
		return nil, fmt.Errorf("plan_rules.yml next to %s requires a single-provider bundle, got %d providers", filename, len(res))
	}

	// External providers ship as a single-provider bundle: plan_rules.yml travels
	// next to terraform_versions.yml (delivered into candi by copyTFVersionFile),
	// so the rule is loaded here. The main multi-provider candi has no plan_rules.
	if len(res) == 1 {
		for cloudName, set := range res {
			simple, ok := set.(*settings.Simple)
			if !ok {
				return nil, fmt.Errorf("provider %s settings have unexpected type %T", cloudName, set)
			}
			if planRule != nil {
				simple.VMResourceVal = planRule
				if err := simple.Validate(false); err != nil {
					return nil, fmt.Errorf("validate provider %s after plan_rules merge: %w", simple.CloudName(), err)
				}
			}
			if simple.VMResourceVal == nil {
				return nil, fmt.Errorf("single-provider bundle %q requires plan_rules.yml with vmResource next to %s", cloudName, filename)
			}
		}
	}

	return res, nil
}

// mergeBundleSettings adds the settings of every external provider bundle
// unpacked under downloadDir. A bundle ships its own single-provider
// terraform_versions.yml (with plan_rules.yml beside it), so its settings are
// read where they live instead of being copied into the shared candi dir —
// which the candi image overwrites on every run, leaving the two files
// describing different providers. Bundle settings win over the candi defaults.
func mergeBundleSettings(ctx context.Context, store settingsStore, downloadDir string) error {
	if downloadDir == "" {
		return nil
	}

	matches, err := filepath.Glob(filepath.Join(downloadDir, "*", "terraform-manager", versionFile))
	if err != nil {
		return fmt.Errorf("look up provider bundle versions files: %w", err)
	}

	for _, match := range matches {
		// A bundle is unpacked into <provider>@<digest> (and, while unpacking,
		// <provider>@<digest>.partial) with a plain <provider> symlink pointing
		// at the current one. Read through that symlink only: the digest dirs of
		// previously delivered versions may still be around, and an unfinished
		// one holds an incomplete tree.
		if strings.Contains(filepath.Base(filepath.Dir(filepath.Dir(match))), "@") {
			continue
		}

		bundle, err := loadTerraformVersionFileSettings(ctx, match)
		if err != nil {
			return fmt.Errorf("load provider bundle settings %s: %w", match, err)
		}
		for cloudName, set := range bundle {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Provider settings for %s taken from bundle %s", cloudName, match))
			store[cloudName] = set
		}
	}

	return nil
}
