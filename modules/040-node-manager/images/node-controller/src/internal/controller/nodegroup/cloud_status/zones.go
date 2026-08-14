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

package cloud_status

import (
	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
)

// zonesCount returns how many zones the NodeGroup spreads over: its own zones if it names any,
// otherwise the ones its provider published.
//
// Zero is a legitimate answer for a NodeGroup outside any cloud. It is not a legitimate answer for
// an unreadable registration, which is why the registry is loaded by the caller and any read
// failure has already aborted the reconcile: zero zones makes Min and Max zero, and a Min of zero
// reports the NodeGroup Ready no matter how many nodes are actually up.
func zonesCount(ng *v1.NodeGroup, providers cloudprovider.Providers) int32 {
	if ng.Spec.CloudInstances != nil && len(ng.Spec.CloudInstances.Zones) > 0 {
		return int32(len(ng.Spec.CloudInstances.Zones))
	}
	provider, ok := providers.ForNodeGroup(ng)
	if !ok {
		return 0
	}
	return int32(len(provider.Zones))
}
