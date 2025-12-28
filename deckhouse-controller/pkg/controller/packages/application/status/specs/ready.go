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

// ReadySpec defines the Ready condition rules.
// Reflects current operational state.
func ReadySpec() statusmapper.Spec {
	return statusmapper.Spec{
		Type: status.ConditionReady,
		Rule: statusmapper.FirstMatch{
			// Ready when runtime is ready
			{
				When:   statusmapper.IsTrue(status.ConditionReadyInRuntime),
				Status: metav1.ConditionTrue,
			},
			// Not ready with details
			{
				When:        statusmapper.IsFalse(status.ConditionReadyInRuntime),
				Status:      metav1.ConditionFalse,
				Reason:      "NotReady",
				MessageFrom: status.ConditionReadyInRuntime,
			},
			// Default fallback
			{
				Status:  metav1.ConditionFalse,
				Reason:  "NotReady",
				Message: "application is not fully operational",
			},
		},
	}
}
