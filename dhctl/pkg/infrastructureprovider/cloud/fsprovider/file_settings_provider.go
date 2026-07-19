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
	"cmp"
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vmresource"
)

const (
	opentofuKey  = "opentofu"
	terraformKey = "terraform"
)

type (
	settingsStore map[string]*settings.Simple
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

	candi, err := loadVersionsFile(ctx, infraVersionsFile, toolVersions{})
	if err != nil {
		return nil, err
	}
	store = candi.providers

	mergeBundleSettings(ctx, store, downloadDir, candi.tools)

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

// toolVersions are the terraform/opentofu versions every provider entry in a
// versions file is pinned to.
type toolVersions struct {
	terraform string
	opentofu  string
}

// versionsFile is one parsed terraform_versions.yml.
type versionsFile struct {
	tools     toolVersions
	providers settingsStore
}

// versionsDoc is that file as it lies on disk: the two tool versions and one
// entry per provider, all at the top level.
type versionsDoc struct {
	Terraform string `json:"terraform"`
	Opentofu  string `json:"opentofu"`

	providers map[string]json.RawMessage
}

// loadVersionsFile parses a terraform_versions.yml. A provider bundle ships
// only the fragment describing itself and omits the tool versions it does not
// use, so it inherits them from the candi file it extends; the candi file
// itself is parsed with no inherited versions and must carry both.
func loadVersionsFile(ctx context.Context, filename string, inherited toolVersions) (versionsFile, error) {
	doc, err := readVersionsDoc(filename)
	if err != nil {
		return versionsFile{}, err
	}

	tools := toolVersions{
		terraform: cmp.Or(doc.Terraform, inherited.terraform),
		opentofu:  cmp.Or(doc.Opentofu, inherited.opentofu),
	}
	if tools.terraform == "" || tools.opentofu == "" {
		return versionsFile{}, fmt.Errorf("infrastructure versions file %s must set both terraform and opentofu versions", filename)
	}

	providers := make(settingsStore, len(doc.providers))
	for name, entry := range doc.providers {
		set, err := parseProvider(entry, tools)
		if err != nil {
			return versionsFile{}, fmt.Errorf("parse settings for provider %s in %s: %w", name, filename, err)
		}

		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found provider settings for %s: %s", name, set.CloudName()))
		providers[set.CloudName()] = set
	}

	planRule, err := loadPlanRules(filename)
	if err != nil {
		return versionsFile{}, err
	}
	if err := attachPlanRules(filename, providers, planRule); err != nil {
		return versionsFile{}, err
	}

	return versionsFile{tools: tools, providers: providers}, nil
}

func readVersionsDoc(filename string) (versionsDoc, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return versionsDoc{}, fmt.Errorf("read infrastructure versions file %s: %w", filename, err)
	}

	doc := versionsDoc{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return versionsDoc{}, fmt.Errorf("parse tool versions in %s: %w", filename, err)
	}
	if err := yaml.Unmarshal(data, &doc.providers); err != nil {
		return versionsDoc{}, fmt.Errorf("parse provider entries in %s: %w", filename, err)
	}
	delete(doc.providers, terraformKey)
	delete(doc.providers, opentofuKey)

	return doc, nil
}

func parseProvider(entry json.RawMessage, tools toolVersions) (*settings.Simple, error) {
	set := settings.Simple{}
	if err := json.Unmarshal(entry, &set); err != nil {
		return nil, err
	}
	if err := set.Validate(false); err != nil {
		return nil, err
	}

	set.InfrastructureVersionVal = new(tools.terraform)
	if set.UseOpenTofu() {
		set.InfrastructureVersionVal = new(tools.opentofu)
	}
	set.CloudNameVal = new(strings.ToLower(*set.CloudNameVal))

	return &set, nil
}

// attachPlanRules attaches the plan rules that sit next to a single-provider
// bundle's versions file. Such a bundle must carry them (they say which
// resource a VM is), and a multi-provider file must not: the rules describe one
// provider, so finding them there means the two files got out of sync.
func attachPlanRules(filename string, providers settingsStore, planRule *vmresource.Rule) error {
	if len(providers) != 1 {
		if planRule != nil {
			return fmt.Errorf("plan_rules.yml next to %s requires a single-provider bundle, got %d providers", filename, len(providers))
		}
		return nil
	}

	for cloudName, set := range providers {
		if planRule != nil {
			set.VMResourceVal = planRule
			if err := set.Validate(false); err != nil {
				return fmt.Errorf("validate provider %s after plan_rules merge: %w", cloudName, err)
			}
		}

		if set.VMResourceVal == nil {
			return fmt.Errorf("single-provider bundle %q requires plan_rules.yml with vmResource next to %s", cloudName, filename)
		}
	}

	return nil
}

// mergeBundleSettings adds the settings of providers that the candi image does
// not ship — today only external ones like DVP, whose terraform_versions.yml
// and plan_rules.yml travel inside its OCI bundle. They are read where the
// bundle keeps them: copying them into the shared candi dir does not survive,
// because the next run extracts the candi image over that same file.
//
// A provider already known from candi keeps those settings: in-tree providers
// unpack a terraform-manager bundle into the very same download dir, and their
// fragment describes only themselves. A bundle that fails to parse is skipped
// with a warning rather than failing every other provider along with it.
func mergeBundleSettings(ctx context.Context, store settingsStore, downloadDir string, inherited toolVersions) {
	if downloadDir == "" {
		return
	}

	matches, err := filepath.Glob(filepath.Join(downloadDir, "*", "terraform-manager", versionFile))
	if err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Cannot look up provider bundle versions files in %s: %v", downloadDir, err))
		return
	}

	for _, match := range matches {
		// A bundle is unpacked into <provider>@<digest> (and, while unpacking,
		// <provider>@<digest>.partial) with a plain <provider> symlink pointing
		// at the current one. Read through that symlink only: the digest dirs of
		// previously delivered versions may still be around, and an unfinished
		// one holds an incomplete tree.
		provider := filepath.Base(filepath.Dir(filepath.Dir(match)))
		if strings.Contains(provider, "@") {
			continue
		}
		if _, known := store[provider]; known {
			continue
		}

		bundle, err := loadVersionsFile(ctx, match, inherited)
		if err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Skipping provider bundle settings %s: %v", match, err))
			continue
		}

		for cloudName, set := range bundle.providers {
			if _, known := store[cloudName]; known {
				continue
			}
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Provider settings for %s taken from bundle %s", cloudName, match))
			store[cloudName] = set
		}
	}
}
