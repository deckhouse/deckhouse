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

package api

const (
	// MigrationConfigMapName is the ConfigMap that marks an in-progress provider migration.
	MigrationConfigMapName = "d8-module-is-migrating"
	// MigrationSecretName is the Secret that stores temporary migration resources.
	MigrationSecretName = "d8-migration-resources"
)

// MigrationStatus describes whether validation should follow the legacy or new resource model.
type MigrationStatus struct {
	// LegacyPCCPresent reports whether providerClusterConfiguration is still present.
	LegacyPCCPresent bool
	// NewResourcesComplete reports whether all new-model resources are ready.
	NewResourcesComplete bool
	// MigrationPending reports whether migration is in progress and incomplete.
	MigrationPending bool
}

// ShouldSkipNewModelValidation reports whether new-model validation must be skipped during migration.
func ShouldSkipNewModelValidation(status MigrationStatus) bool {
	return status.MigrationPending || (status.LegacyPCCPresent && !status.NewResourcesComplete)
}
