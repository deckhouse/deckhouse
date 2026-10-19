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

// What Global consumes: the global module, the interfaces of its effects, Deps and Config.

package global

import (
	"context"

	"github.com/Masterminds/semver/v3"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/hooklane"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Module is the global module as Global uses it (modules/global.Module).
type Module interface {
	hooklane.Object

	GetVersion() *semver.Version
	ValidateSettings(ctx context.Context, settingsVersion int, settings addonutils.Values) (settingscheck.Result, error)
	ApplySettings(settingsVersion int, settings addonutils.Values) error
	// LatestSettingsVersion is the latest settings-schema version, the one a restored checkpoint
	// is applied at (the core sets it at Boot); 0 without versioned settings.
	LatestSettingsVersion() int
	GetSettings() addonutils.Values
	RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error
	// SetEnabledModules and SetCapabilities publish into the global values.
	SetEnabledModules(modules []string)
	SetCapabilities(apiVersions []string)
	// SetModulesImagesDigests publishes the enabled modules' image digests, by module name, beside
	// the values: renders see them, the values checksum (Output) does not.
	SetModulesImagesDigests(digests map[string]map[string]string)
}

// digested is a loaded module that ships image digests (modules.Module).
type digested interface {
	GetDigests() map[string]string
}

type publisher interface {
	Publish(name string, facts schedule.Facts)
}

// hookLanes starts the global hooks in their lanes (hooklane.Router.Start). The global hooks live
// as long as Global: the stop Start returns is called when Global exits.
type hookLanes interface {
	Start(ctx, parent context.Context, pkg hooklane.Object, changed func()) (func(), error)
}

// crdService applies the CRDs shipped in a module's image (internal/crd.Service).
type crdService interface {
	// Install applies every CRD under path/crds, owned by name; a no-op without CRDs. Safe for
	// concurrent calls for different modules.
	Install(ctx context.Context, name, path string) error
	HasCRDs(path string) error // HasCRDs returns nil when path ships CRDs.
	// GetManagedGVKs returns the GVKs Install applied for the named modules.
	GetManagedGVKs(enabledModules []string) []string
}

// webhookService applies the conversion webhooks a module's chart declares; a no-op when it
// declares none. The composition root implements it, because the rendering returns Kubernetes
// manifests.
type webhookService interface {
	EnsureWebhooks(ctx context.Context, view model.ModuleView) error
}

// Deps are the effects of Global.
type Deps struct {
	Scheduler publisher
	Hooks     hookLanes
	CRDs      crdService
	Webhooks  webhookService
	Clock     retry.Clock // Clock nil means the wall clock.
	Logger    *log.Logger // Logger nil means a no-op logger.
}

// Config is what Engine passes to New.
type Config struct {
	// Module is the global module; Engine.Boot has added its node already.
	Module Module
	// Version is the identity Global publishes its facts with; the global node has the same.
	Version string
	// Settings is the checkpoint; nil makes Global wait for the first Apply.
	Settings *model.Settings
	// Notify is called after Global's Report changed; must not block. Nil disables it.
	Notify func()
}
