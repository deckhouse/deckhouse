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

// ManagedSpec defines the Managed condition rules.
// Currently follows Ready; extensible for managed/unmanaged modes.
func ManagedSpec() types.MappingSpec {
	return types.MappingSpec{
		Type: types.ConditionManaged,
		MappingRules: []types.MappingRule{
			// Actively managed when runtime ready
			{
				Name:    "actively-managed",
				Matcher: types.InternalTrue(status.ConditionReadyInRuntime),
				Status:  corev1.ConditionTrue,
			},
			// Future: add unmanaged mode support
			// {
			//     Name:    "unmanaged-mode",
			//     Matcher: types.Predicate{Name: "unmanaged", Fn: isUnmanagedMode},
			//     Status:  corev1.ConditionFalse,
			//     Reason:  "UnmanagedModeActivated",
			// },
			// Hooks failed
			{
				Name:        "event-hooks-failed",
				Matcher:     types.InternalFalse(status.ConditionHooksProcessed),
				Status:      corev1.ConditionFalse,
				Reason:      "OperationFailed",
				MessageFrom: status.ConditionHooksProcessed,
			},
			// Default
			{
				Name:    "default-not-managed",
				Matcher: types.Always{},
				Status:  corev1.ConditionFalse,
			},
		},
	}
}
