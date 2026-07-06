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

package lifecycle

import (
	"context"

	addonutils "github.com/flant/addon-operator/pkg/utils"
)

// Store manages lifecycle contexts and pending settings for all runtime packages.
// It is type-agnostic — it does not hold the loaded Application/Module instances,
// only the version, settings, and context tree needed for change detection and
// cancellation. The actual runtime instances live in plain maps on Runtime.
//
// Store is not thread-safe; callers must hold Runtime.mu before calling any method.
type Store struct {
	packages map[string]*Package
}

// NewStore creates an empty Store ready for use.
func NewStore() *Store {
	return &Store{
		packages: make(map[string]*Package),
	}
}

// NeedUpdate reports whether the package needs processing: true if the package
// is new, the version changed, or the settings checksum differs.
// Used as a fast-path check before the more expensive Update call.
func (s *Store) NeedUpdate(name, version, checksum string) bool {
	pkg, ok := s.packages[name]
	if !ok {
		return true
	}

	if pkg.version != version {
		return true
	}

	if pkg.settings.Checksum() != checksum {
		return true
	}

	return false
}

// Update registers a new package or processes a version change.
//
// Returns a new root context (EventUpdate) when:
//  1. Package not in store → creates entry, returns root context
//  2. Version differs → cancels all in-flight tasks, returns new root context
//
// Returns nil when only settings or settingsVersion changed (no new context needed —
// settings are stored and will be picked up by the scheduler via GetPendingSettings on
// next Reschedule, or by the next Configure task in the schedule pipeline).
//
// Callers should check for nil: a nil return with a settings-only change means
// the caller should trigger Reschedule to re-apply settings through the scheduler.
func (s *Store) Update(name, version string, settingsVersion int, settings addonutils.Values) context.Context {
	pkg, ok := s.packages[name]
	if !ok {
		s.packages[name] = &Package{
			version:         version,
			settingsVersion: settingsVersion,
			settings:        settings,
			cancels:         make(map[int]context.CancelFunc),
		}

		ctx := s.packages[name].newContext(EventUpdate)
		return ctx
	}

	if pkg.version != version {
		pkg.version = version
		pkg.settingsVersion = settingsVersion
		pkg.settings = settings

		ctx := pkg.newContext(EventUpdate)
		return ctx
	}

	checksumChanged := pkg.settings.Checksum() != settings.Checksum()
	versionChanged := pkg.settingsVersion != settingsVersion

	if checksumChanged {
		pkg.settings = settings
	}
	if checksumChanged || versionChanged {
		pkg.settingsVersion = settingsVersion
	}

	return nil
}

// UpdateSettings stores new pending settings and their schema version for an
// already-tracked package without touching its version or context tree.
// Returns true if the settings checksum or settingsVersion changed and the
// caller should Reschedule, false if nothing changed or the package is not
// tracked yet.
//
// Unlike Update, this never creates or cancels a context: in-flight deploy and
// load tasks are left running. It is the settings-only counterpart to Update,
// used when settings change independently of a version change. The ModuleConfig
// enabled intent is tracked separately by the global module, not here.
func (s *Store) UpdateSettings(name string, settingsVersion int, settings addonutils.Values) bool {
	pkg, ok := s.packages[name]
	if !ok {
		return false
	}

	checksumChanged := pkg.settings.Checksum() != settings.Checksum()
	versionChanged := pkg.settingsVersion != settingsVersion

	if !checksumChanged && !versionChanged {
		return false
	}

	if checksumChanged {
		pkg.settings = settings
	}
	pkg.settingsVersion = settingsVersion

	return true
}

// HandleEvent renews the context for the given event type and returns it.
//
// For EventRemove: clears version and settings before renewing context, so a
// subsequent Update sees the package as new (enabling re-create after remove).
//
// Returns nil if the package doesn't exist in the store.
func (s *Store) HandleEvent(event int, name string) context.Context {
	pkg, ok := s.packages[name]
	if !ok {
		return nil
	}

	if event == EventRemove {
		pkg.version = ""
		pkg.settingsVersion = 0
		pkg.settings = make(addonutils.Values)
	}

	return pkg.newContext(event)
}

// GetPendingSettings returns the latest settings and their schema version stored
// for a package. Called by schedulePackage to pass current settings and version
// into the Configure task so it can convert from the stored version to latest.
// This late-binding approach ensures settings changes that arrive between Update
// and schedule are automatically picked up.
func (s *Store) GetPendingSettings(name string) (addonutils.Values, int) {
	return s.packages[name].settings, s.packages[name].settingsVersion
}

// Delete removes a package entry from the store if it still exists and is in
// the removed state (version cleared by HandleEvent(EventRemove)).
// Returns true if the entry was deleted.
//
// Safe against re-creation races: if Update has already set a new version
// between the remove and this cleanup, the version is non-empty and Delete
// returns false, preserving the re-created entry.
func (s *Store) Delete(name string) bool {
	pkg, ok := s.packages[name]
	if !ok || pkg.version != "" {
		return false
	}

	delete(s.packages, name)
	return true
}
