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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/statusmapper"
)

// ManagedSpec defines the Managed condition rules.
// Currently follows Ready; extensible for managed/unmanaged modes.
func ManagedSpec() statusmapper.Spec {
	return statusmapper.Spec{
		Type: status.ConditionManaged,
		Rule: statusmapper.FirstMatch{
			// Actively managed when runtime ready
			{
				When:   statusmapper.IsTrue(status.ConditionReadyInRuntime),
				Status: metav1.ConditionTrue,
			},
			// Future: add unmanaged mode support
			// {
			//     When:   statusmapper.Predicate{Name: "unmanaged", Fn: isUnmanagedMode},
			//     Status: metav1.ConditionFalse,
			//     Reason: "UnmanagedModeActivated",
			// },
			// Hooks failed
			{
				When:        statusmapper.IsFalse(status.ConditionHooksProcessed),
				Status:      metav1.ConditionFalse,
				Reason:      "OperationFailed",
				MessageFrom: status.ConditionHooksProcessed,
			},
			// Default
			{
				Status: metav1.ConditionFalse,
			},
		},
	}
}
