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

package common

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertConditionsRoundtrip(t *testing.T) {
	now := metav1.NewTime(time.Now().UTC().Truncate(time.Second))
	in := []metav1.Condition{
		{Type: ConditionTypeReady, Status: metav1.ConditionTrue, Message: "ok", LastTransitionTime: now},
		{Type: ConditionTypeError, Status: metav1.ConditionFalse, Message: "", LastTransitionTime: now},
	}

	calc := ConvertToCalcConditions(in)
	out := ConvertFromCalcConditions(calc)

	if len(out) != len(in) {
		t.Fatalf("unexpected length: got %d want %d", len(out), len(in))
	}
	if out[0].Type != in[0].Type || out[0].Status != in[0].Status || out[0].Message != in[0].Message {
		t.Fatalf("first condition mismatch: got %#v want %#v", out[0], in[0])
	}
	if out[1].Type != in[1].Type || out[1].Status != in[1].Status {
		t.Fatalf("second condition mismatch: got %#v want %#v", out[1], in[1])
	}
}
