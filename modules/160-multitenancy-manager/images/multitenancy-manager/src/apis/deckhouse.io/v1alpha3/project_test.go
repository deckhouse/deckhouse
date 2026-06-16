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

package v1alpha3

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestNamespaceStatusUnmarshal_BackwardCompat guards against the startup crash caused by projects
// whose status.namespaces was stored by older controllers as a list of plain strings.
func TestNamespaceStatusUnmarshal_BackwardCompat(t *testing.T) {
	// legacy form: list of plain namespace-name strings
	var legacy ProjectStatus
	err := json.Unmarshal([]byte(`{"namespaces":["foo","foo-bar"]}`), &legacy)
	assert.NoError(t, err)
	assert.Equal(t, []NamespaceStatus{{Name: "foo"}, {Name: "foo-bar"}}, legacy.Namespaces)

	// current form: list of objects
	var current ProjectStatus
	err = json.Unmarshal([]byte(`{"namespaces":[{"name":"foo","kind":"Main"},{"name":"foo-bar","kind":"Additional"}]}`), &current)
	assert.NoError(t, err)
	assert.Equal(t, []NamespaceStatus{{Name: "foo", Kind: "Main"}, {Name: "foo-bar", Kind: "Additional"}}, current.Namespaces)

	// round-trip: re-marshaling always produces the object form
	out, err := json.Marshal(current.Namespaces)
	assert.NoError(t, err)
	assert.JSONEq(t, `[{"name":"foo","kind":"Main"},{"name":"foo-bar","kind":"Additional"}]`, string(out))
}

// TestSetCondition pins the shared condition semantics used by the PRB/CPRB/ProjectNamespace
// reconcilers: a no-op call reports "unchanged" and rewrites nothing, a message change does not move
// LastTransitionTime, and a status transition does.
func TestSetCondition(t *testing.T) {
	var conditions []Condition

	// first set appends and reports a change
	assert.True(t, SetCondition(&conditions, "Ready", corev1.ConditionTrue, ""))
	assert.Len(t, conditions, 1)
	assert.Equal(t, corev1.ConditionTrue, conditions[0].Status)
	assert.False(t, conditions[0].LastTransitionTime.IsZero())

	// an identical set is a no-op: reports no change and does not touch the timestamps
	probe := conditions[0].LastProbeTime
	transition := conditions[0].LastTransitionTime
	assert.False(t, SetCondition(&conditions, "Ready", corev1.ConditionTrue, ""))
	assert.Equal(t, probe, conditions[0].LastProbeTime)
	assert.Equal(t, transition, conditions[0].LastTransitionTime)

	// backdate so a real transition is observable despite second-granularity timestamps
	past := metav1.NewTime(time.Now().Add(-time.Hour))
	conditions[0].LastTransitionTime = past

	// a real transition (True -> False) reports a change and moves LastTransitionTime forward
	assert.True(t, SetCondition(&conditions, "Ready", corev1.ConditionFalse, "boom"))
	assert.Equal(t, corev1.ConditionFalse, conditions[0].Status)
	assert.Equal(t, "boom", conditions[0].Message)
	assert.True(t, conditions[0].LastTransitionTime.After(past.Time), "a status transition must move LastTransitionTime")

	// a message-only change (same status) reports a change but keeps LastTransitionTime
	moved := conditions[0].LastTransitionTime
	assert.True(t, SetCondition(&conditions, "Ready", corev1.ConditionFalse, "still bad"))
	assert.Equal(t, "still bad", conditions[0].Message)
	assert.True(t, conditions[0].LastTransitionTime.Equal(&moved), "a message-only change must not move LastTransitionTime")
}
