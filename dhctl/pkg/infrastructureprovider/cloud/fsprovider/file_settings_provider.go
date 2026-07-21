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
	loader        func(ctx context.Context, infraVersionsFile, downloadDir string) (settingsStore, error)
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

// loadOrGetStore builds the provider settings store: the providers from the
// candi versions file plus those an external bundle delivers into downloadDir.
//
// Only the candi file is cached — it is fixed for the process. The bundles are
// merged fresh on every call: a long-lived process (dhctl-server, converge
// exporter) can have a bundle delivered after the first store was built, and a
// whole-store cache would keep returning the pre-bundle map, leaving the
// provider unavailable until restart.
func loadOrGetStore(ctx context.Context, infraVersionsFile, downloadDir string) (settingsStore, error) {
	candi, err := loadCandiVersions(ctx, infraVersionsFile)
	if err != nil {
		return nil, err
	}

	store := maps.Clone(candi.providers)
	mergeBundleSettings(ctx, store, downloadDir, candi.tools)

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
// them; a bundle describing more than one provider is malformed.
func attachBundlePlanRules(filename string, providers settingsStore) error {
	if len(providers) != 1 {
		return fmt.Errorf("provider bundle %s must describe exactly one provider, got %d", filename, len(providers))
	}

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
		if err := attachBundlePlanRules(match, bundle.providers); err != nil {
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
