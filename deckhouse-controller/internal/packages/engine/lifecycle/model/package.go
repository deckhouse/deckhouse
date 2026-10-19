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

package model

import (
	"context"
	"strings"

	"github.com/Masterminds/semver/v3"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// Kind is the package kind; it selects the disk, the loader and Current's step lists.
type Kind string

const (
	KindApplication Kind = "Application" // KindApplication is an apps.Application.
	KindModule      Kind = "Module"      // KindModule is a modules.Module.
	// KindGlobal addresses Global (the global module the deckhouse image ships). It has no
	// package Reconciler: Engine routes an Apply of this kind to Global.
	KindGlobal Kind = "Global"
)

// Package is the loaded package object, the revision itself (apps.Application, modules.Module);
// the loader returns it as this type, so the compiler checks both. Each load creates a new object,
// so a Candidate and Current never share one. The object keeps its own values and hook
// controllers, and may read the cluster internally (grants); the core never does.
//
// It is the carrier type the object travels in (loader → preparation → package Reconciler →
// Current, and to Global in ModuleView), so its method set is the union of what consumers call;
// each consumer calls it through its own narrow interface. Remove a method here once no consumer
// calls it.
type Package interface {
	nelm.Package // GetName, GetPath, GetRuntimeValues, GetValues, GetMaintenance

	// GetVersion is the parsed version dependents' rules compare; the revision's identity is
	// Image.Version.
	GetVersion() *semver.Version
	GetConstraints() schedule.Constraints

	// ValidateSettings converts settings from settingsVersion to the revision's schema and
	// validates them, without changing the object. A verdict is a function of the settings alone
	// (conversion, schema, the settings check hook's answer): Result{Valid: false} with its
	// Message; the final gate memoizes it. Anything that depends on cluster state (an unavailable
	// grant) or reached no verdict (a hook that could not run, ctx ended) is an error, which
	// callers retry with backoff. It may read the cluster and run the settings check hook, so it
	// blocks.
	ValidateSettings(ctx context.Context, settingsVersion int, settings addonutils.Values) (settingscheck.Result, error)
	// ApplySettings converts and stores settings in the object's values.
	ApplySettings(settingsVersion int, settings addonutils.Values) error
	// GetSettings returns the effective settings: stored settings merged with the schema defaults.
	GetSettings() addonutils.Values
	SetMaintenance(state nelm.MaintenanceState)
	// GetValuesChecksum is the checksum of the values hooks and templates see.
	GetValuesChecksum() string

	// The hooks: bindings are started in the hook lanes (lifecycle/hooklane); the core runs only
	// the bindings of its own operations.
	InitializeHooks()
	DisableHooks()
	GetHooksQueues() []string
	GetHooksByBinding(binding shtypes.BindingType) []hooks.ControllableHook
	UnlockKubernetesMonitors(hook string, monitors ...string)
	RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error
	RunHookByName(ctx context.Context, hook string, bctx []bctx.BindingContext) error
}

// Source is where a package image comes from in the registry.
type Source struct {
	Repository registry.Remote
	Package    string // Package is the package name in the repository.
}

// Image is an attached package image, immutable. The package Reconciler owns every image of its
// package and is the only one that detaches it: what a preparation attached is the package
// Reconciler's once its Done delivered, Restore hands its image over in the checkpoint.
type Image struct {
	Kind Kind   // Kind selects the disk.
	Name string // Name is the package name: the node name and the release name.
	// Version is the revision's identity: what the update decision compares, the node's revision
	// in the scheduler protocol (Node.Revision, Facts.Version, Event.Version; facts of another
	// revision are dropped) and the stored image.
	Version  string
	Source   Source // Source is empty for an embedded image.
	Embedded bool   // Embedded: the deckhouse image directory; nothing is attached or detached.
	// Instance is the attach instance (name@version) that keys the mount and its detach; empty for
	// an embedded image.
	Instance string
	Path     string // Path is the image root; valid until it is detached.
}

// Revision is an attached image with the object loaded from it. A Candidate is a Revision the
// package Reconciler prepared; Current runs one. Its owner owns the image; ownership is never
// shared.
type Revision struct {
	Image   Image
	Package Package
}

// ReleaseNamespace is the namespace of a package's release: an application's is the part of its
// name before the first dot (apps.BuildName; a namespace has no dot), a module's is
// modulesNamespace. It needs only the name, so a remove without a loaded object finds the release
// too.
func ReleaseNamespace(kind Kind, name, modulesNamespace string) string {
	if kind == KindApplication {
		namespace, _, _ := strings.Cut(name, ".")

		return namespace
	}

	return modulesNamespace
}
