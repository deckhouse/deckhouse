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

package specs

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status/types"
)

// ReadySpec defines the Ready condition rules.
// Reflects current operational state.
func ReadySpec() types.MappingSpec {
	return types.MappingSpec{
		Type: types.ConditionReady,
		MappingRules: []types.MappingRule{
			// Ready when runtime is ready
			{
				Name:    "runtime-ready",
				Matcher: types.InternalTrue(status.ConditionReadyInRuntime),
				Status:  corev1.ConditionTrue,
			},
			// Not ready with details
			{
				Name:        "runtime-not-ready",
				Matcher:     types.InternalFalse(status.ConditionReadyInRuntime),
				Status:      corev1.ConditionFalse,
				Reason:      "NotReady",
				MessageFrom: status.ConditionReadyInRuntime,
			},
			// Default fallback
			{
				Name:    "default-not-ready",
				Matcher: types.Always{},
				Status:  corev1.ConditionFalse,
				Reason:  "NotReady",
				Message: "application is not fully operational",
			},
		},
	}
}
