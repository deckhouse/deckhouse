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
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcerequests"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// Settings are a package's settings as the user wrote them. They carry no generation: Current
// detects a change by the checksum of the effective settings the object reports after applying
// them.
type Settings struct {
	Values addonutils.Values `json:"values"`
	// SchemaVersion is the settings-schema version Values are validated and applied at; the object
	// converts them from it to its own schema. Desired settings carry spec.version; a restored
	// checkpoint, already effective values, carries the restored object's latest version, so it is
	// never converted a second time (set by the core at Boot).
	SchemaVersion int `json:"schemaVersion"`
}

// Configuration is everything the package Reconciler pushes to Current. Current applies all of it
// to the object on accept, and every field is part of its inputs.
type Configuration struct {
	Settings    Settings              `json:"settings"`
	Maintenance nelm.MaintenanceState `json:"maintenance"` // Maintenance is spec.maintenance.
	// ResourceRequests are Application spec.resourceRequests; nil for modules.
	ResourceRequests []resourcerequests.Request `json:"resourceRequests"`
}

// UpdateMode selects whether an update waits for an update window.
type UpdateMode string

const (
	UpdateModeAuto   UpdateMode = "Auto"   // UpdateModeAuto updates within the windows.
	UpdateModeManual UpdateMode = "Manual" // UpdateModeManual updates as soon as the version is desired.
)

// Windows is the update calendar (go_lib/hooks/update.Windows satisfies it), carried to the window
// gate, which declares its own interface over it.
type Windows interface {
	IsAllowed(t time.Time) bool
	NextAllowedTime(t time.Time) time.Time
}

// Policy controls when an update may be activated.
type Policy struct {
	Mode    UpdateMode
	Windows Windows // Windows nil means any time.
	Force   bool    // Force skips the window check.
}

// Desired is the full desired state of one package, always passed whole, so the latest one is all
// the package Reconciler needs. The caller must not mutate its maps and slices afterwards: the
// package Reconciler keeps the value.
type Desired struct {
	Name string // Name is the package name: the node name and the release name.
	// Kind is fixed for the life of a package Reconciler. For KindGlobal Engine routes it to
	// Global, which reads only Configuration.Settings.
	Kind Kind
	// Version is the revision that should become Current: the update decision compares it, and it
	// names the stored image, the node's revision and status.currentVersion.
	Version       string
	Configuration Configuration
	Policy        Policy
	// Requirements are the constraints declared on the package version resource, checked before
	// anything is downloaded.
	Requirements schedule.Constraints
	Source       Source // Source is unused for an embedded package.
	// Embedded takes the image from the deckhouse image directory. It does not switch the update
	// off: a first install of an embedded module passes the ordinary gates.
	Embedded bool
	// Intent is the module's enable intent from Module.spec.enabled; nil means none. Read only for
	// KindModule; it reaches the scheduler before the Desired, so the first node never runs against it.
	Intent *bool
}

// BootPackage is the checkpoint of one installed package. Spec settings are not used at boot, so
// invalid settings in spec cannot prevent the package from starting; they arrive through Apply
// and are reported as a configuration failure.
type BootPackage struct {
	Name string
	Kind Kind
	// Version is status.currentVersion, written at activation; empty restores nothing. For an
	// embedded package it is the version the image ships, the only one after a Deckhouse upgrade.
	Version string
	// Settings is the settings checkpoint, possibly of the previous version: the effective values
	// of status.lastAppliedConfiguration, whose SchemaVersion the core sets to the restored
	// object's latest at Boot. nil means absent
	// (resources installed before the field existed): Current waits for the first Apply rather
	// than run with schema defaults instead of the user's settings. Empty Values mean the package
	// ran without user settings.
	Settings *Settings
	// Maintenance and ResourceRequests come from spec: they are not validated against a schema, so
	// they cannot prevent a start, and the restored Current runs before the first Apply; without
	// them it would reconcile a release the user put into maintenance.
	Maintenance      nelm.MaintenanceState
	ResourceRequests []resourcerequests.Request
	// Intent is the module's enable intent from Module.spec.enabled; nil means none. It must reach
	// the scheduler before Resume: an unknown intent falls to the module floor (Disable).
	Intent *bool
	// Source is the repository in spec, not necessarily the one the version came from (the
	// checkpoint does not record it). The repositories of one package are mirrors; if a new one
	// lacks currentVersion, the package cannot be restored and Deckhouse exits until it has it or
	// is set back. Accepted.
	Source   Source
	Embedded bool
	// Removing: the resource is being deleted. The package is restored only to be uninstalled as
	// at runtime (hooks, release, purge), since a restart interrupted its removal.
	Removing bool
}

// Restored is a restored package as the package Reconciler starts it.
type Restored struct {
	// Revision is attached, loaded and its node added; the package Reconciler owns its image.
	Revision Revision
	// Configuration is nil when the settings checkpoint is absent: Current then waits for Apply.
	Configuration *Configuration
	// Removing starts the package Reconciler removing, before any run (BootPackage.Removing).
	Removing bool
}
