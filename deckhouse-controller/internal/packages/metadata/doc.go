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

// Package metadata projects parsed package definitions
// (internal/packages/dto) onto the version metadata the catalog CRs carry:
// ModulePackageVersionStatusMetadata and its nested Package* shapes.
//
// The projection has more than one writer:
//
//   - the module-package-version controller fills draft versions with
//     definitions it pulls from the package repository;
//   - the startup version sync (internal/versionsync) fills embedded module
//     versions with definitions it reads from the image disk.
//
// Both must produce identical metadata for the same definition: what a
// version object says about its package cannot depend on which writer got
// there first. Keeping the mapping in one internal package is what holds that
// invariant, and it lets the sync reuse the mapping without importing
// controller code.
//
// The legacy module.yaml converter (moduletypes.Definition) deliberately
// stays in the module-package-version controller: moving it here would drag a
// pkg/controller/moduleloader dependency into internal. Only its requirements
// projection is shared (LegacyRequirementsToCR), because that piece speaks
// pure API types.
package metadata
