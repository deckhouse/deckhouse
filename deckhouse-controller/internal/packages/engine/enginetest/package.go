// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package enginetest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"sync"

	"github.com/Masterminds/semver/v3"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/script"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// Spec describes a package version the fake loader serves; the zero value is a valid package.
type Spec struct {
	// Constraints of the loaded object; a nil Floor gets the kind's (modules Disable, apps Enable).
	Constraints schedule.Constraints
	// Defaults are merged under the user settings (the effective settings).
	Defaults addonutils.Values
	// Validate rejects settings with its error; nil accepts everything.
	Validate func(values addonutils.Values) error
	// LatestSettingsVersion is the object's latest settings-schema version.
	LatestSettingsVersion int
	// EnabledScript, when set, makes the module ship an enabled script with this descriptor.
	EnabledScript *script.Descriptor
	// ExclusiveGroup is the module's exclusive group (modules.Module.GetExclusiveGroup).
	ExclusiveGroup string
	// Digests are the module's image digests (modules.Module.GetDigests).
	Digests map[string]string
}

// Package is the fake package object (model.Package): settings, values and hooks in memory. Hook
// behaviour comes from the kit's Hooks programs, keyed by package name, so it survives reloads.
type Package struct {
	name    string
	version *semver.Version
	path    string
	spec    Spec
	kit     *Kit

	mu          sync.Mutex
	settings    addonutils.Values
	schema      int               // schema is the settings version of the last ApplySettings.
	extra       addonutils.Values // extra are values hooks wrote (runtime hooks, AfterHelm).
	maintenance nelm.MaintenanceState
	onStartup   bool // onStartup: OnStartup succeeded since the hooks were last disabled.
}

var _ model.Package = (*Package)(nil)

func newPackage(k *Kit, kind model.Kind, image model.Image, spec Spec) *Package {
	if spec.Constraints.Floor == nil {
		if kind == model.KindModule {
			spec.Constraints.Floor = rule.Static(rule.Disable)
		} else {
			spec.Constraints.Floor = rule.Static(rule.Enable)
		}
	}

	return &Package{
		name:     image.Name,
		version:  semver.MustParse(image.Version),
		path:     image.Path,
		spec:     spec,
		kit:      k,
		settings: addonutils.Values{},
		extra:    addonutils.Values{},
	}
}

// GetName is the package name.
func (p *Package) GetName() string { return p.name }

// GetPath is the image path the object was loaded from.
func (p *Package) GetPath() string { return p.path }

// GetRuntimeValues is empty: the fake renders nothing.
func (p *Package) GetRuntimeValues() string { return "" }

// GetValues returns the values hooks and templates see: effective settings plus hook values.
func (p *Package) GetValues() addonutils.Values {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.valuesLocked()
}

func (p *Package) valuesLocked() addonutils.Values {
	values := addonutils.Values{"settings": p.effectiveLocked()}
	maps.Copy(values, p.extra)

	return values
}

// GetMaintenance returns the maintenance mode last set.
func (p *Package) GetMaintenance() nelm.MaintenanceState {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.maintenance
}

// GetVersion is the version the image declares.
func (p *Package) GetVersion() *semver.Version { return p.version }

// GetConstraints returns the spec's constraints.
func (p *Package) GetConstraints() schedule.Constraints { return p.spec.Constraints }

// LatestSettingsVersion is the spec's latest settings-schema version.
func (p *Package) LatestSettingsVersion() int { return p.spec.LatestSettingsVersion }

// ValidateSettings runs Spec.Validate; it changes nothing.
func (p *Package) ValidateSettings(_ context.Context, _ int, settings addonutils.Values) (settingscheck.Result, error) {
	p.kit.record("validate:" + p.name + "@" + p.version.Original())

	if err := p.kit.Hooks.settingsError(p.name); err != nil {
		return settingscheck.Result{}, err
	}

	if p.spec.Validate != nil {
		if err := p.spec.Validate(settings); err != nil {
			return settingscheck.Result{Valid: false, Message: err.Error()}, nil
		}
	}

	return settingscheck.Result{Valid: true}, nil
}

// ApplySettings stores a copy of settings.
func (p *Package) ApplySettings(version int, settings addonutils.Values) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.settings = maps.Clone(settings)
	p.schema = version

	return nil
}

// AppliedSettingsVersion is the settings-schema version of the last ApplySettings.
func (p *Package) AppliedSettingsVersion() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.schema
}

// GetSettings returns the effective settings: Defaults overlaid with the stored settings.
func (p *Package) GetSettings() addonutils.Values {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.effectiveLocked()
}

func (p *Package) effectiveLocked() addonutils.Values {
	out := maps.Clone(p.spec.Defaults)
	if out == nil {
		out = addonutils.Values{}
	}

	maps.Copy(out, p.settings)

	return out
}

// SetMaintenance stores the maintenance mode.
func (p *Package) SetMaintenance(state nelm.MaintenanceState) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.maintenance = state
}

// GetValuesChecksum hashes the values hooks and templates see.
func (p *Package) GetValuesChecksum() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, _ := json.Marshal(p.valuesLocked())
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

// SetValue writes a hook value, as a hook patch would; the caller decides whether to call changed.
func (p *Package) SetValue(key string, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.extra[key] = value
}

// InitializeHooks records the initialization.
func (p *Package) InitializeHooks() {
	p.kit.record("initialize-hooks:" + p.name)
}

// DisableHooks clears the hooks; the next enable runs OnStartup again.
func (p *Package) DisableHooks() {
	p.mu.Lock()
	p.onStartup = false
	p.mu.Unlock()

	p.kit.record("disable-hooks:" + p.name)
}

// GetHooksQueues returns no queues: the fake hook bindings own no queue.
func (p *Package) GetHooksQueues() []string { return nil }

// GetHooksByBinding returns no hooks: bindings are faked by the kit's Hooks.
func (p *Package) GetHooksByBinding(shtypes.BindingType) []hooks.ControllableHook { return nil }

// UnlockKubernetesMonitors does nothing.
func (p *Package) UnlockKubernetesMonitors(string, ...string) {}

// RunHookByName does nothing: runtime hooks are simulated with Hooks.RuntimeChange.
func (p *Package) RunHookByName(context.Context, string, []bctx.BindingContext) error { return nil }

// RunHooksByBinding runs the kit's program for (name, binding): a failure, a block until released
// or cancelled, a values write. OnStartup runs once per enable, as the real object.
func (p *Package) RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error {
	if binding == shtypes.OnStartup {
		p.mu.Lock()
		done := p.onStartup
		p.mu.Unlock()

		if done {
			return nil
		}
	}

	p.kit.record("hook:" + p.name + ":" + string(binding))

	if err := p.kit.Hooks.run(ctx, p, binding); err != nil {
		return err
	}

	if binding == shtypes.OnStartup {
		p.mu.Lock()
		p.onStartup = true
		p.mu.Unlock()
	}

	return nil
}

// GetDigests returns the spec's image digests.
func (p *Package) GetDigests() map[string]string { return p.spec.Digests }

// GetExclusiveGroup returns the spec's exclusive group.
func (p *Package) GetExclusiveGroup() string { return p.spec.ExclusiveGroup }

// GetEnabledScriptDescriptor returns the spec's enabled script; nil when it has none.
func (p *Package) GetEnabledScriptDescriptor() *script.Descriptor {
	return p.spec.EnabledScript
}

// GlobalModule is the fake global module (global.Module).
type GlobalModule struct {
	*Package

	mu           sync.Mutex
	enabled      []string
	capabilities []string
	digests      map[string]map[string]string
}

// SetEnabledModules publishes the enabled set into the global values.
func (g *GlobalModule) SetEnabledModules(modules []string) {
	g.mu.Lock()
	g.enabled = append([]string(nil), modules...)
	g.mu.Unlock()

	g.SetValue("enabledModules", append([]string(nil), modules...))
}

// SetCapabilities publishes the capabilities into the global values.
func (g *GlobalModule) SetCapabilities(apiVersions []string) {
	g.mu.Lock()
	g.capabilities = append([]string(nil), apiVersions...)
	g.mu.Unlock()
}

// SetModulesImagesDigests records the published image digests; like the real module, outside the
// values, so they do not change Global's Output.
func (g *GlobalModule) SetModulesImagesDigests(digests map[string]map[string]string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.digests = digests
}

// ImagesDigests returns the last published image digests of module name; nil without any.
func (g *GlobalModule) ImagesDigests(name string) map[string]string {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.digests[name]
}

// EnabledModules returns the last published enabled set.
func (g *GlobalModule) EnabledModules() []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	return append([]string(nil), g.enabled...)
}
