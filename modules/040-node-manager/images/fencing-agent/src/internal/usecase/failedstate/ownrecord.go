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

package failedstate

import (
	"time"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

func StartOfLife(now time.Time) time.Time {
	return now.Truncate(time.Second)
}

func OwnFailedRecord(states []v1alpha1.FencingFailedNodeState, node string, startedAt time.Time) *v1alpha1.FencingFailedNodeState {
	for i := range states {
		state := &states[i]
		if state.Name == node && state.Status.Failed != nil && !state.CreationTimestamp.Time.Before(startedAt) {
			return state
		}
	}

	return nil
}
