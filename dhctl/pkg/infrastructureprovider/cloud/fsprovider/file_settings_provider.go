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
	"maps"
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
	settingsStore map[string]*settings.Simple
	loader        func(ctx context.Context, infraVersionsFile, bundleDir string) (settingsStore, error)
)

type SettingsProvider struct {
	initError error

	m     sync.Mutex
	store settingsStore
}

var (
	candiStoreMutex sync.Mutex
	candiStoreCache = make(map[string]versionsFile)
)

// loadOrGetStore builds the provider settings store: candi's providers plus the one whose bundle is
// unpacked in bundleDir. Only the candi file is cached; the bundle is merged on every call.
func loadOrGetStore(ctx context.Context, infraVersionsFile, bundleDir string) (settingsStore, error) {
	candi, err := loadCandiVersions(ctx, infraVersionsFile)
	if err != nil {
		return nil, err
	}

	store := maps.Clone(candi.providers)
	if err := mergeBundleSettings(ctx, store, bundleDir, candi.tools); err != nil {
		return nil, err
	}

	return store, nil
}

func loadCandiVersions(ctx context.Context, infraVersionsFile string) (versionsFile, error) {
	candiStoreMutex.Lock()
	defer candiStoreMutex.Unlock()

	if candi, ok := candiStoreCache[infraVersionsFile]; ok {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Candi provider settings for terraform versions file %s loaded from cache", infraVersionsFile))
		return candi, nil
	}

	candi, err := loadVersionsFile(ctx, infraVersionsFile, toolVersions{})
	if err != nil {
		return versionsFile{}, err
	}

	candiStoreCache[infraVersionsFile] = candi
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Candi provider settings for terraform versions file %s loaded from file and cached", infraVersionsFile))

	return candi, nil
}

func newSettingsProvider(ctx context.Context, infraVersionsFile, bundleDir string, loader loader) *SettingsProvider {
	store, err := loader(ctx, infraVersionsFile, bundleDir)
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

// loadVersionsFile parses the providers and tool versions from a
// terraform_versions.yml. A provider bundle ships only the fragment describing
// itself and omits the tool versions it does not use, so it inherits them from
// the candi file it extends; the candi file itself is parsed with no inherited
// versions and must carry both.
//
// plan_rules.yml is not read here: it belongs to a single-provider bundle, and
// the bundle loader attaches it (see attachBundlePlanRules). The multi-provider
// candi file has no plan rules, so nothing looks for them next to it.
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

// attachBundlePlanRules folds a bundle's plan_rules.yml into its single
// provider's settings. The rules (which manifest a VM change touches) live next
// to the bundle's terraform_versions.yml, and every external bundle must carry
// them.
func attachBundlePlanRules(filename string, providers settingsStore) error {
	planRule, err := loadPlanRules(filename)
	if err != nil {
		return err
	}
	if planRule == nil {
		return fmt.Errorf("provider bundle %s is missing plan_rules.yml with vmResource", filename)
	}

	for cloudName, set := range providers {
		set.VMResourceVal = planRule
		if err := set.Validate(false); err != nil {
			return fmt.Errorf("validate provider %s after plan_rules merge: %w", cloudName, err)
		}
	}

	return nil
}

// mergeBundleSettings adds the settings of the provider whose bundle is unpacked in bundleDir when
// the candi image does not ship it, like DVP. A provider candi knows keeps its candi settings.
func mergeBundleSettings(ctx context.Context, store settingsStore, bundleDir string, inherited toolVersions) error {
	if bundleDir == "" {
		return nil
	}

	filename := filepath.Join(bundleDir, terraformManagerDir, versionFile)

	bundle, err := loadVersionsFile(ctx, filename, inherited)
	if err != nil {
		return fmt.Errorf("load provider bundle settings: %w", err)
	}
	if len(bundle.providers) != 1 {
		return fmt.Errorf("provider bundle %s must describe exactly one provider, got %d", filename, len(bundle.providers))
	}

	maps.DeleteFunc(bundle.providers, func(cloudName string, _ *settings.Simple) bool {
		_, known := store[cloudName]
		return known
	})
	if len(bundle.providers) == 0 {
		return nil
	}

	if err := attachBundlePlanRules(filename, bundle.providers); err != nil {
		return fmt.Errorf("load provider bundle settings: %w", err)
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Provider settings taken from bundle %s", filename))
	maps.Copy(store, bundle.providers)

	return nil
}
