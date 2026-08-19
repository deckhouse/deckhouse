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

// Package modulesync fills Module (v1alpha2) resources with facts already
// present in the cluster: the package version and repository come from the
// image, a pull override or a deployed release; the configuration comes
// from the ModuleConfig.
//
// The Module spec is what the package system drives a module by: which
// version to install, from which repository, with what settings, on or off.
// Today those facts live scattered across the older resources, and the
// Module spec stays empty. This package keeps it filled and current, so the
// package system can take over the modules exactly as they already run.
//
// Identifiers carry the resource version: moduleV2 is the v1alpha2 Module
// this package fills, moduleV1 is the legacy one - it shows up in a single
// read, the source of an overridden module.
//
// # Data sources for the Module fields
//
//	embedded modules dir (the running image)
//	  ├─ spec.packageVersion        = the Deckhouse version
//	  ├─ spec.packageRepositoryName = "embedded"
//	  └─ annotation modules.deckhouse.io/embedded
//
//	ready ModulePullOverride
//	  ├─ spec.packageVersion        = spec.imageTag
//	  ├─ spec.packageRepositoryName = the module's source
//	  └─ annotation modules.deckhouse.io/dev
//
//	deployed ModuleRelease (the newest one)
//	  ├─ spec.packageVersion        = the release version
//	  └─ spec.packageRepositoryName = the release source
//
//	live ModuleConfig
//	  ├─ spec.settings, spec.settingsVersion
//	  └─ spec.enabled, spec.maintenance
//
// When several sources claim the same module, the first one wins:
// image > pull override > deployed release.
//
// The sync also supersedes an older release still marked deployed - its
// only write outside Module resources.
//
// # When it runs
//
// Sync runs once at every deckhouse-controller start, before the
// controllers begin reconciling. A start is exactly when the facts change:
//   - an upgrade brings an image with a new set of embedded modules;
//   - a deployed release or a pull override reaches the filesystem only
//     through a restart.
//
// Running first means the controllers wake up to already filled Module
// resources.
//
// While the old module stack is still in charge, its controllers also
// mirror their events into the Modules one at a time - see ensure.go.
//
// # Write contract
//
// The sync writes the fields listed above and nothing else:
//   - the v1alpha1 properties and the status stay untouched;
//   - writes are merge patches (or a create), so fields owned by other
//     writers survive;
//   - the sync sends no patch when nothing changed;
//   - with WithOrphanDeletion (the bootstrap mode) the sync deletes
//     orphaned modules - those no source claims and that carry no package;
//     without it they are left to the old module stack.
//
// # Clients
//
// New takes a writing client and a reading client separately. All reads go
// through the reader, and callers pass a direct (uncached) one: the sync
// then sees its own prior writes and runs before the controller-runtime
// manager starts.
//
// Two callers share the package and write the same data the same way: the
// transitional deckhouse-controller startup and the standalone
// package-runtime bootstrap (internal/controller).
//
// # Lifecycle
//
// The image is the one permanent source: embedded modules follow it on
// every upgrade. The other inputs leave the platform with the package
// system rollout - pull overrides and releases first, module configs last.
// Their resolvers (origin_pull_overrides.go, origin_module_releases.go,
// configs.go) and the controller mirrors (ensure.go) die with them; the
// core stays.
package modulesync
