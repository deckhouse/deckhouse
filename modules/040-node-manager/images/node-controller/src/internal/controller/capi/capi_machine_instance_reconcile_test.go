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

package capi

import (
	"testing"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMergeMachineConditions(t *testing.T) {
	t.Parallel()

	existing := []deckhousev1alpha2.InstanceCondition{
		{
			Type:   deckhousev1alpha2.InstanceConditionTypeBashibleReady,
			Status: metav1.ConditionTrue,
			Reason: "StepsCompleted",
		},
		{
			Type:   deckhousev1alpha2.InstanceConditionTypeMachineReady,
			Status: metav1.ConditionFalse,
			Reason: "OldReason",
		},
	}

	overrides := []deckhousev1alpha2.InstanceCondition{
		{
			Type:   deckhousev1alpha2.InstanceConditionTypeMachineReady,
			Status: metav1.ConditionTrue,
			Reason: "Ready",
		},
	}

	merged := mergeMachineConditions(existing, overrides)
	if len(merged) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(merged))
	}

	if merged[0].Type != deckhousev1alpha2.InstanceConditionTypeBashibleReady {
		t.Fatalf("expected first condition to be %q, got %q", deckhousev1alpha2.InstanceConditionTypeBashibleReady, merged[0].Type)
	}
	if merged[1].Type != deckhousev1alpha2.InstanceConditionTypeMachineReady {
		t.Fatalf("expected second condition to be %q, got %q", deckhousev1alpha2.InstanceConditionTypeMachineReady, merged[1].Type)
	}
	if merged[1].Reason != "Ready" {
		t.Fatalf("expected machine condition to be overridden, got reason %q", merged[1].Reason)
	}
}
