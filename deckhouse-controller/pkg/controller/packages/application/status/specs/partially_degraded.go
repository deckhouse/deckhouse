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

// PartiallyDegradedSpec defines the PartiallyDegraded condition rules.
// Inverse of Ready; extensible for non-critical degradation.
func PartiallyDegradedSpec() statusmapper.Spec {
	return statusmapper.Spec{
		Type: status.ConditionPartiallyDegraded,
		Rule: statusmapper.FirstMatch{
			// Not degraded when fully operational
			{
				When:   statusmapper.IsTrue(status.ConditionReadyInRuntime),
				Status: metav1.ConditionFalse,
			},
			// Future: add rules for non-critical degradation here
			// {
			//     When:   statusmapper.Predicate{Name: "scaling", Fn: isScalingInProgress},
			//     Status: metav1.ConditionTrue,
			//     Reason: "ScalingInProgress",
			// },
			// Default: degraded
			{
				Status: metav1.ConditionTrue,
			},
		},
	}
}
