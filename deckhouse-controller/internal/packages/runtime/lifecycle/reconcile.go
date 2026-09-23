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

package lifecycle

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// DecisionKind is what the store makes of a desired state: nothing, a reschedule, or the pipeline.
type DecisionKind uint8

const (
	// DecisionNone means the stored state already matches; the caller does nothing.
	DecisionNone DecisionKind = iota
	// DecisionReconfigure means settings moved: reschedule and the package picks them up.
	DecisionReconfigure
	// DecisionUpdate means the version moved (or a reload was forced): run the pipeline.
	DecisionUpdate
)

// Decision is the verdict on a desired state, with the changes that produced it.
type Decision struct {
	Kind    DecisionKind
	Changes Changes
}

// Changes names the fields a desired state moved; Forced stands in for a version change.
type Changes struct {
	Version         bool
	Settings        bool
	SettingsVersion bool
	Maintenance     bool
	Forced          bool
}

// Any reports whether the desired state moved at all.
func (c Changes) Any() bool {
	return c.Version ||
		c.Settings ||
		c.SettingsVersion ||
		c.Maintenance ||
		c.Forced
}

// DesiredState is a package as its controller wants it. ForceReload restarts the pipeline for a
// mutable tag re-pushed under an unchanged version.
type DesiredState struct {
	Version         string
	Settings        addonutils.Values
	SettingsVersion int
	Maintenance     string
	ForceReload     bool
}

// Reconcile stores the desired state of a package, registering it when it is new, and returns what
// the caller has to do about it. It starts no operation and cancels nothing.
func (s *Store) Reconcile(name string, desired DesiredState) Decision {
	pkg, ok := s.packages[name]
	if !ok {
		pkg = &Package{operations: make(map[OperationKind]operation)}
		s.packages[name] = pkg
	}

	return pkg.reconcile(desired)
}

// reconcile compares the desired state against the stored one and adopts it when it differs.
func (p *Package) reconcile(desired DesiredState) Decision {
	changes := Changes{
		Version:         p.version != desired.Version,
		Settings:        p.settings.Checksum() != desired.Settings.Checksum(),
		SettingsVersion: p.settingsVersion != desired.SettingsVersion,
		Maintenance:     p.maintenance != desired.Maintenance,
		Forced:          desired.ForceReload,
	}

	if !changes.Any() {
		return Decision{Kind: DecisionNone}
	}

	p.version = desired.Version
	p.settings = desired.Settings
	p.settingsVersion = desired.SettingsVersion
	p.maintenance = desired.Maintenance

	switch {
	case changes.Version, changes.Forced:
		return Decision{
			Kind:    DecisionUpdate,
			Changes: changes,
		}
	default:
		return Decision{
			Kind:    DecisionReconfigure,
			Changes: changes,
		}
	}
}
