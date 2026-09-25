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
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcerequests"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// Store manages the lifecycle state and the running operations of all runtime packages.
// It is type-agnostic — it does not hold the loaded Application/Module instances,
// only the version, settings and contexts needed for change detection and cancellation.
// The actual runtime instances live in plain maps on Runtime.
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

// GetPendingSettings returns the latest settings and their schema version stored
// for a package. Called by schedulePackage to pass current settings and version
// into the Configure task so it can convert from the stored version to latest.
// This late-binding approach ensures settings changes that arrive between Update
// and schedule are automatically picked up.
func (s *Store) GetPendingSettings(name string) (addonutils.Values, int) {
	return s.packages[name].settings, s.packages[name].settingsVersion
}

// GetPendingResourceRequests returns the latest per-workload resource overrides
// stored for a package. Nil for a package that is not tracked, or whose CR has no
// such field.
func (s *Store) GetPendingResourceRequests(name string) []resourcerequests.Request {
	pkg, ok := s.packages[name]
	if !ok {
		return nil
	}

	return pkg.resourceRequests
}

// GetPendingMaintenance returns the latest maintenance mode stored for a package.
// Called by schedulePackage to pass the current mode into the Run task. Empty
// means the package is managed normally.
func (s *Store) GetPendingMaintenance(name string) string {
	return s.packages[name].maintenance
}
