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
)

// Store is the central registry for runtime packages.
// It manages the full lifecycle: creation, updates, event handling, and removal.
type Store[P runtimePackage] struct {
	packages map[string]*Package[P]
}

// NewStore creates an empty Store ready for use.
func NewStore[P runtimePackage]() *Store[P] {
	return &Store[P]{
		packages: make(map[string]*Package[P]),
	}
}

// Callback is invoked by Update when a version or settings change is detected.
// Receives the new context, the event type, and the current app (nil on first install).
type Callback[P runtimePackage] func(ctx context.Context, event int, pkg P)

// Update registers a new package or detects changes to an existing one.
//
// Change detection (evaluated in order, first match wins):
//  1. Package not in store → new entry, EventVersionChanged
//  2. Version differs → EventVersionChanged (cancels all in-flight tasks)
//  3. Settings checksum differs → EventSettingsChanged (cancels previous settings apply)
//  4. No change → callback is not invoked
//
// For EventVersionChanged, app is nil on first install or the previously loaded app on upgrade.
func (s *Store[P]) Update(name, version, settings string, f Callback[P]) {
	pkg, ok := s.packages[name]
	if !ok {
		s.packages[name] = &Package[P]{
			version:  version,
			checksum: settings,
			cancels:  make(map[int]context.CancelFunc),
		}

		ctx := s.packages[name].newContext(EventVersionChanged)
		f(ctx, EventVersionChanged, s.packages[name].pkg)
		return
	}

	if pkg.version != version {
		pkg.version = version
		pkg.checksum = settings
		ctx := pkg.newContext(EventVersionChanged)
		f(ctx, EventVersionChanged, s.packages[name].pkg)
		return
	}

	if pkg.checksum != settings {
		pkg.checksum = settings
		ctx := pkg.newContext(EventSettingsChanged)
		f(ctx, EventSettingsChanged, s.packages[name].pkg)
		return
	}
}

// Range iterates over all loaded packages under a read lock.
// Skips entries where the app has not been loaded yet (pkg == nil).
// The callback must not call other Store methods.
func (s *Store[P]) Range(f func(pkg P)) {
	for _, pkg := range s.packages {
		if pkg.pkg != nil {
			f(pkg.pkg)
		}
	}
}

// GetPackage returns the loaded runtime package, or nil if the package
// doesn't exist or hasn't been loaded yet.
func (s *Store[P]) GetPackage(name string) P {
	pkg, ok := s.packages[name]
	if !ok {
		return nil
	}

	return pkg.pkg
}

// SetPackage stores the loaded runtime package for a package.
// Called by the Load task after successfully building the app from its package files.
// No-op if the package entry doesn't exist (e.g., removed between download and load).
func (s *Store[P]) SetPackage(name string, pkg P) {
	if _, ok := s.packages[name]; !ok {
		return
	}

	s.packages[name].pkg = pkg
}

// HandleEvent renews the context for the given event type and invokes the callback
// with the new context and the loaded app.
//
// For EventRemove: clears version and checksum before renewing context, so a
// subsequent Update sees the package as new (enabling re-create after remove).
//
// No-op if the package doesn't exist or hasn't been loaded yet (pkg == nil).
// The callback executes under the Store lock.
func (s *Store[P]) HandleEvent(event int, name string, f Callback[P]) {
	pkg, ok := s.packages[name]
	if !ok || pkg.pkg == nil {
		return
	}

	f(pkg.newContext(event), event, pkg.pkg)

	if event == EventRemove {
		pkg.version = ""
		pkg.checksum = ""
		pkg.pkg = nil
	}
}

// Delete removes a package entry from the store if it is still in the removed state
// (version empty and pkg nil). Returns true if the entry was deleted.
//
// Safe for concurrent use with Update: if the package has been re-created
// (new version set by Update), the entry is preserved.
func (s *Store[P]) Delete(name string) bool {
	pkg, ok := s.packages[name]
	if !ok {
		return false
	}

	if pkg.version != "" || pkg.pkg != nil {
		return false
	}

	delete(s.packages, name)
	return true
}
