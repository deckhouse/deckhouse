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

package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

// resolveCandiDir mirrors NewSchemaStore: prefer the value from
// GlobalOptions, fall back to options.DefaultCandiDir.
func resolveCandiDir(globalOptions *options.GlobalOptions) string {
	if globalOptions != nil && globalOptions.CandiDir != "" {
		return globalOptions.CandiDir
	}
	return options.DefaultCandiDir
}

const (
	cniBootstrapFileName   = "cni-bootstrap.yml"
	cniBootstrapSourcePCC  = "providerClusterConfiguration"
	cniBootstrapSupportedV = 1
)

// sigs.k8s.io/yaml parses by converting YAML to JSON and unmarshalling with
// encoding/json, so these structs use json tags, not yaml tags.
type cniBootstrap struct {
	SchemaVersion int                `json:"schemaVersion"`
	Name          string             `json:"name"`
	Config        cniBootstrapConfig `json:"config"`
}

type cniBootstrapConfig struct {
	Default map[string]any     `json:"default"`
	Rules   []cniBootstrapRule `json:"rules"`
}

type cniBootstrapRule struct {
	Source   string                `json:"source"`
	Match    cniBootstrapRuleMatch `json:"match"`
	Settings map[string]any        `json:"settings"`
}

type cniBootstrapRuleMatch struct {
	JSONPath string `json:"jsonPath"`
	Values   []any  `json:"values"`
}

// CNIBootstrapMismatchReason mirrors proto CNIBootstrapMismatchReason.
type CNIBootstrapMismatchReason string

const (
	CNIBootstrapMismatchReasonNone              CNIBootstrapMismatchReason = ""
	CNIBootstrapMismatchReasonDifferentModule   CNIBootstrapMismatchReason = "DifferentModule"
	CNIBootstrapMismatchReasonDifferentSettings CNIBootstrapMismatchReason = "DifferentSettings"
)

// CNIBootstrapSkipReason mirrors proto CNIBootstrapSkipReason.
type CNIBootstrapSkipReason string

const (
	CNIBootstrapSkipReasonNone          CNIBootstrapSkipReason = ""
	CNIBootstrapSkipReasonStaticCluster CNIBootstrapSkipReason = "StaticCluster"
)

type CNIBootstrapModuleConfigs struct {
	Recommended *ModuleConfig
	UserInput   *ModuleConfig
}

type CNIBootstrapAnalysis struct {
	ProviderName   string
	ModuleConfig   *CNIBootstrapModuleConfigs
	Matches        bool
	MismatchReason CNIBootstrapMismatchReason
	SkipReason     CNIBootstrapSkipReason
	ReasonMessage  string
}

// AnalyzeCNIBootstrap is pure (non-mutating) analysis of the recommended CNI
// ModuleConfig vs the user's input. cni-bootstrap.yml is mandatory for cloud
// providers; its absence is a repository/installer bug and returns an error.
// globalOptions may be nil — the default candi dir is used in that case.
func AnalyzeCNIBootstrap(ctx context.Context, m *MetaConfig, globalOptions *options.GlobalOptions) (*CNIBootstrapAnalysis, error) {
	return analyzeCNIBootstrap(ctx, m, globalOptions, "")
}

// analyzeCNIBootstrap is the AnalyzeCNIBootstrap implementation with an
// optional content override for tests. Production callers go through
// AnalyzeCNIBootstrap and read from disk.
func analyzeCNIBootstrap(ctx context.Context, m *MetaConfig, globalOptions *options.GlobalOptions, contentOverride string) (*CNIBootstrapAnalysis, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil || m.ClusterType != CloudClusterType || m.ProviderName == "" {
		return &CNIBootstrapAnalysis{
			Matches:       true,
			SkipReason:    CNIBootstrapSkipReasonStaticCluster,
			ReasonMessage: "Static cluster: cni-bootstrap analysis is not applicable",
		}, nil
	}
	out := &CNIBootstrapAnalysis{ProviderName: m.ProviderName, Matches: true}

	var raw []byte
	var path string
	if contentOverride != "" {
		raw = []byte(contentOverride)
		path = "<injected>"
	} else {
		path = filepath.Join(resolveCandiDir(globalOptions), "cloud-providers", m.ProviderName, cniBootstrapFileName)
		var err error
		raw, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read cni-bootstrap file %s: %w", path, err)
		}
	}

	var b cniBootstrap
	if err := yaml.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("parse cni-bootstrap file %s: %w", path, err)
	}
	if b.SchemaVersion != cniBootstrapSupportedV {
		return nil, fmt.Errorf("cni-bootstrap file %s: unsupported schemaVersion %d, want %d", path, b.SchemaVersion, cniBootstrapSupportedV)
	}
	if b.Name == "" {
		return nil, fmt.Errorf("cni-bootstrap file %s: empty name", path)
	}

	settings, err := resolveCNIBootstrapSettings(b, m.ProviderClusterConfig)
	if err != nil {
		return nil, fmt.Errorf("resolve cni-bootstrap settings: %w", err)
	}

	moduleName := "cni-" + b.Name
	store := NewSchemaStore(nil)
	if !store.HasSchemaForModuleConfig(moduleName) {
		return nil, fmt.Errorf("cni-bootstrap file %s references unknown ModuleConfig %q (schema missing from installer)", path, moduleName)
	}

	recommended, err := buildModuleConfig(store, moduleName, true, settings)
	if err != nil {
		return nil, fmt.Errorf("build ModuleConfig %s: %w", moduleName, err)
	}

	_, user := findUserCNIModuleConfig(m.ModuleConfigs)
	out.ModuleConfig = &CNIBootstrapModuleConfigs{
		Recommended: recommended,
		UserInput:   user,
	}

	out.MismatchReason, out.ReasonMessage = cniBootstrapDecision(user, recommended)
	out.Matches = out.MismatchReason == CNIBootstrapMismatchReasonNone
	return out, nil
}

// ApplyCNIBootstrap appends or replaces the user's cni-* ModuleConfig with
// the recommendation derived from cni-bootstrap.yml. On mismatch the user is
// prompted to confirm; in non-interactive mode the user's MC is kept. The
// override is a full replace (not a merge): any custom fields outside the
// recommendation are discarded, which is what the confirm prompt warns about.
func ApplyCNIBootstrap(ctx context.Context, m *MetaConfig, globalOptions *options.GlobalOptions) error {
	analysis, err := AnalyzeCNIBootstrap(ctx, m, globalOptions)
	if err != nil {
		return err
	}
	if analysis.SkipReason != "" {
		return nil
	}

	recommended := analysis.ModuleConfig.Recommended
	user := analysis.ModuleConfig.UserInput

	if user == nil {
		m.ModuleConfigs = append(m.ModuleConfigs, recommended)
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("cni-bootstrap: added recommended ModuleConfig %q", recommended.GetName()))
		return nil
	}

	if analysis.Matches {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("cni-bootstrap: user ModuleConfig %q matches recommendation", user.GetName()))
		return nil
	}

	userIdx, _ := findUserCNIModuleConfig(m.ModuleConfigs)
	msg := cniBootstrapConfirmMessage(analysis)
	if input.NewConfirmation().WithMessage(msg).Ask() {
		m.ModuleConfigs[userIdx] = recommended
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("cni-bootstrap: replaced user ModuleConfig %q with %q", user.GetName(), recommended.GetName()))
		return nil
	}
	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("cni-bootstrap: keeping user ModuleConfig %q", user.GetName()))
	return nil
}

func cniBootstrapDecision(user, recommended *ModuleConfig) (CNIBootstrapMismatchReason, string) {
	if user == nil {
		return CNIBootstrapMismatchReasonNone, ""
	}
	if user.GetName() != recommended.GetName() {
		return CNIBootstrapMismatchReasonDifferentModule, fmt.Sprintf(
			"user configured %q, provider recommends %q", user.GetName(), recommended.GetName(),
		)
	}
	if cniEnabledValue(user) != cniEnabledValue(recommended) {
		return CNIBootstrapMismatchReasonDifferentSettings, fmt.Sprintf(
			"%s enabled differs from recommendation (user=%t, recommended=%t)",
			recommended.GetName(), cniEnabledValue(user), cniEnabledValue(recommended),
		)
	}
	same, err := sameCNISettings(user.Spec.Settings, recommended.Spec.Settings)
	if err != nil {
		return CNIBootstrapMismatchReasonDifferentSettings, fmt.Sprintf("compare settings: %v", err)
	}
	if !same {
		return CNIBootstrapMismatchReasonDifferentSettings, fmt.Sprintf(
			"settings for %s differ from recommendation (user=%s, recommended=%s)",
			recommended.GetName(),
			formatCNISettings(user.Spec.Settings),
			formatCNISettings(recommended.Spec.Settings),
		)
	}
	return CNIBootstrapMismatchReasonNone, ""
}

// cniEnabledValue treats a nil *bool as the documented default (enabled).
func cniEnabledValue(mc *ModuleConfig) bool {
	if mc.Spec.Enabled == nil {
		return true
	}
	return *mc.Spec.Enabled
}

func cniBootstrapConfirmMessage(a *CNIBootstrapAnalysis) string {
	user := a.ModuleConfig.UserInput
	recommended := a.ModuleConfig.Recommended
	if user.GetName() == recommended.GetName() {
		return fmt.Sprintf(
			"Provider cni-bootstrap.yml recommends a different config for %s.\n  user:        enabled=%t settings=%s\n  recommended: enabled=%t settings=%s\nReplace your ModuleConfig with the recommended one? Any custom settings in your ModuleConfig will be discarded.",
			recommended.GetName(),
			cniEnabledValue(user), formatCNISettings(user.Spec.Settings),
			cniEnabledValue(recommended), formatCNISettings(recommended.Spec.Settings),
		)
	}
	return fmt.Sprintf(
		"Provider recommends ModuleConfig %s, but you configured %s.\nReplace your ModuleConfig with the recommended one? Your %s ModuleConfig will be removed.",
		recommended.GetName(), user.GetName(), user.GetName(),
	)
}

// resolveCNIBootstrapSettings applies rule settings over default per top-level
// key (overwrite, not deep merge).
func resolveCNIBootstrapSettings(b cniBootstrap, providerCfg map[string]json.RawMessage) (map[string]any, error) {
	settings := map[string]any{}
	maps.Copy(settings, b.Config.Default)

	if len(b.Config.Rules) == 0 {
		return settings, nil
	}

	data, err := unmarshalProviderClusterConfig(providerCfg)
	if err != nil {
		return nil, err
	}

	for _, r := range b.Config.Rules {
		if r.Source != cniBootstrapSourcePCC {
			ctx := context.Background()
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("cni-bootstrap: skipping rule with unsupported source %q", r.Source))
			continue
		}
		value, ok := cniBootstrapLookup(data, r.Match.JSONPath)
		if !ok {
			continue
		}
		if !cniBootstrapMatches(value, r.Match.Values) {
			continue
		}
		maps.Copy(settings, r.Settings)
	}

	return settings, nil
}

func unmarshalProviderClusterConfig(providerCfg map[string]json.RawMessage) (map[string]any, error) {
	if len(providerCfg) == 0 {
		return map[string]any{}, nil
	}
	pccJSON, err := json.Marshal(providerCfg)
	if err != nil {
		return nil, fmt.Errorf("marshal providerClusterConfiguration: %w", err)
	}
	var data map[string]any
	if err := json.Unmarshal(pccJSON, &data); err != nil {
		return nil, fmt.Errorf("unmarshal providerClusterConfiguration: %w", err)
	}
	return data, nil
}

func findUserCNIModuleConfig(mcs []*ModuleConfig) (int, *ModuleConfig) {
	for i, mc := range mcs {
		if strings.HasPrefix(mc.GetName(), "cni-") {
			return i, mc
		}
	}
	return -1, nil
}

func sameCNISettings(a, b SettingsValues) (bool, error) {
	// Treat nil and empty as equivalent: buildModuleConfig leaves Settings=nil
	// when no settings were provided, while user YAML may produce settings: {}.
	if len(a) == 0 && len(b) == 0 {
		return true, nil
	}
	aj, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(aj, bj), nil
}

func formatCNISettings(s SettingsValues) string {
	if len(s) == 0 {
		return "{}"
	}
	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Sprintf("%v", s)
	}
	return string(b)
}
