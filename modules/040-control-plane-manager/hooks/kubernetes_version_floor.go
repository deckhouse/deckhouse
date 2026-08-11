/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import "github.com/Masterminds/semver/v3"

// KubernetesVersionBelowFloor reports whether target lands more than one minor below floor.
//
// This is the single "how far down may we go" rule of the whole version subsystem, and it lives in
// one place because two independent components have to answer it identically:
//
//   - admission — the ClusterConfiguration downgrade check and the ModuleConfig guard in
//     deckhouse-controller, where the floor is maxUsedKubernetesVersion;
//   - the soft guard in global-hooks/discovery/target_kubernetes_version.go, which freezes the
//     published target when the Deckhouse default falls outside the same window.
//
// If the two ever disagree, the cluster gets the worst of both: admission accepts a pin that the
// resolver immediately freezes away from, or the resolver publishes a target admission would have
// rejected. They used to be two copies of this switch.
//
// Callers own parsing. The soft guard has to trim and parse strings that arrive from Secret data and
// a hand-editable ConfigMap, while admission already holds parsed versions; keeping that out of here
// is what lets both use the same rule without one of them faking a signature.
//
// The minor comparison is written as an addition on target so the uint64 minor never underflows on a
// 1.0-style version.
func KubernetesVersionBelowFloor(target, floor *semver.Version) bool {
	switch {
	case target.Major() > floor.Major():
		return false
	case target.Major() == floor.Major() && target.Minor()+1 >= floor.Minor():
		return false
	default:
		return true
	}
}
