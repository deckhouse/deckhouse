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
// The single "how far down may we go" rule, in one place because admission and the soft guard in
// global-hooks/discovery/target_kubernetes_version.go must answer it identically — they used to be
// two copies of this switch. The minor comparison is an addition on target so the uint64 never
// underflows.
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
