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

package quota

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/apis/deckhouse.io/v1alpha1"
)

func rl(pairs ...string) corev1.ResourceList {
	out := corev1.ResourceList{}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[corev1.ResourceName(pairs[i])] = resource.MustParse(pairs[i+1])
	}
	return out
}

func TestCalculated(t *testing.T) {
	hard := rl("pods", "10", "requests.cpu", "2")
	tests := []struct {
		name     string
		rq       *corev1.ResourceQuota
		expected bool
	}{
		{name: "no quota", rq: nil},
		{name: "not counted yet", rq: &corev1.ResourceQuota{}},
		{name: "counted with every key", rq: &corev1.ResourceQuota{Status: corev1.ResourceQuotaStatus{Hard: rl("pods", "10", "requests.cpu", "2000m")}}, expected: true},
		{name: "a key not counted yet", rq: &corev1.ResourceQuota{Status: corev1.ResourceQuotaStatus{Hard: rl("pods", "10")}}},
		{name: "an old value", rq: &corev1.ResourceQuota{Status: corev1.ResourceQuotaStatus{Hard: rl("pods", "5", "requests.cpu", "2")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Calculated(tt.rq, hard))
		})
	}

	// the resource quota controller writes no status.hard for a quota without limits
	assert.True(t, Calculated(&corev1.ResourceQuota{}, nil), "a quota without limits, once it exists")
	assert.True(t, Calculated(&corev1.ResourceQuota{}, corev1.ResourceList{}))
	assert.False(t, Calculated(nil, nil), "a missing quota")
}

func TestSum(t *testing.T) {
	hard := rl("pods", "10", "requests.memory", "4Gi")
	got := Sum(hard, []Namespace{
		{Name: "a", Used: rl("pods", "3", "requests.memory", "1Gi", "secrets", "7")},
		{Name: "b", Used: rl("pods", "2")},
		{Name: "c"},
	})
	require.Len(t, got, 2, "only the keys of the quota are summed")
	pods, memory := got[corev1.ResourcePods], got[corev1.ResourceRequestsMemory]
	assert.Equal(t, int64(5), pods.Value())
	assert.Equal(t, int64(1<<30), memory.Value())
}

func TestTop(t *testing.T) {
	hard := rl("pods", "100", "requests.cpu", "10")
	namespaces := []Namespace{
		{Name: "idle"},
		{Name: "pods-heavy", Used: rl("pods", "50")},
		{Name: "cpu-heavy", Used: rl("requests.cpu", "9")},
		{Name: "b-tie", Used: rl("pods", "10")},
		{Name: "a-tie", Used: rl("pods", "10")},
	}
	got := Top(hard, namespaces, 4)
	names := make([]string, 0, len(got))
	for _, ns := range got {
		names = append(names, ns.Name)
	}
	assert.Equal(t, []string{"cpu-heavy", "pods-heavy", "a-tie", "b-tie"}, names)

	many := make([]Namespace, 0, 120)
	for i := range 120 {
		many = append(many, Namespace{Name: fmt.Sprintf("ns-%03d", i), Used: rl("pods", "1")})
	}
	assert.Len(t, Top(hard, many, v1alpha1.ProjectQuotaTopNamespaces), v1alpha1.ProjectQuotaTopNamespaces)
}

func TestExceededCondition(t *testing.T) {
	trueWith := func(reason string) *metav1.Condition {
		return &metav1.Condition{Type: v1alpha1.ProjectQuotaConditionExceeded, Status: metav1.ConditionTrue, Reason: reason}
	}
	falseCondition := &metav1.Condition{Type: v1alpha1.ProjectQuotaConditionExceeded, Status: metav1.ConditionFalse, Reason: v1alpha1.ProjectQuotaReasonWithinLimits}
	tests := []struct {
		name         string
		previous     *metav1.Condition
		previousHard corev1.ResourceList
		hard         corev1.ResourceList
		used         corev1.ResourceList
		generation   int64
		status       metav1.ConditionStatus
		reason       string
	}{
		{name: "within limits", hard: rl("pods", "10"), used: rl("pods", "10"), status: metav1.ConditionFalse, reason: v1alpha1.ProjectQuotaReasonWithinLimits},
		{name: "created below the usage", hard: rl("pods", "10"), used: rl("pods", "11"), status: metav1.ConditionTrue, reason: v1alpha1.ProjectQuotaReasonHardLowered},
		{name: "the limit lowered below the usage", previous: falseCondition, previousHard: rl("pods", "20"), hard: rl("pods", "10"), used: rl("pods", "11"),
			status: metav1.ConditionTrue, reason: v1alpha1.ProjectQuotaReasonHardLowered},
		{name: "the usage grew past the limit", previous: falseCondition, previousHard: rl("pods", "10"), hard: rl("pods", "10"), used: rl("pods", "11"),
			status: metav1.ConditionTrue, reason: v1alpha1.ProjectQuotaReasonOvershoot},
		{name: "a reason sticks", previous: trueWith(v1alpha1.ProjectQuotaReasonHardLowered), previousHard: rl("pods", "10"), hard: rl("pods", "10"), used: rl("pods", "12"),
			status: metav1.ConditionTrue, reason: v1alpha1.ProjectQuotaReasonHardLowered},
		// the first evaluation of a quota whose namespaces were not all calculated before, as after an
		// upgrade with the usage already above the limit: the usage did not grow past it
		{name: "found above the limit of a new generation", previous: falseCondition, previousHard: rl("pods", "1"), hard: rl("pods", "1"), used: rl("pods", "3"),
			generation: 2, status: metav1.ConditionTrue, reason: v1alpha1.ProjectQuotaReasonHardLowered},
		{name: "lowering a limit keeps an overshoot", previous: trueWith(v1alpha1.ProjectQuotaReasonOvershoot), previousHard: rl("pods", "10"), hard: rl("pods", "5"),
			used: rl("pods", "12"), status: metav1.ConditionTrue, reason: v1alpha1.ProjectQuotaReasonOvershoot},
		{name: "lowering again overrides a sticky HardLowered", previous: trueWith(v1alpha1.ProjectQuotaReasonHardLowered), previousHard: rl("pods", "10"),
			hard: rl("pods", "5"), used: rl("pods", "12"), status: metav1.ConditionTrue, reason: v1alpha1.ProjectQuotaReasonHardLowered},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExceededCondition(tt.previous, tt.previousHard, tt.hard, tt.used, tt.generation, "")
			assert.Equal(t, tt.status, got.Status)
			assert.Equal(t, tt.reason, got.Reason)
			if tt.status == metav1.ConditionTrue {
				assert.Contains(t, got.Message, "pods: used")
			}
		})
	}
}

func TestEnforcedCondition(t *testing.T) {
	enforce := Config{ControlPlaneManaged: true}
	ready := []Namespace{{Name: "a", Calculated: true, Labelled: true}}
	tests := []struct {
		name   string
		in     EnforcedInput
		status metav1.ConditionStatus
		reason string
	}{
		{name: "enforced", in: EnforcedInput{Config: enforce, ProjectFound: true, Namespaces: ready}, status: metav1.ConditionTrue, reason: v1alpha1.ProjectQuotaReasonEnforced},
		{name: "managed control plane", in: EnforcedInput{Config: Config{}, ProjectFound: true, Namespaces: ready},
			status: metav1.ConditionFalse, reason: v1alpha1.ProjectQuotaReasonControlPlaneNotManaged},
		{name: "no project", in: EnforcedInput{Config: enforce}, status: metav1.ConditionFalse, reason: v1alpha1.ProjectQuotaReasonProjectNotFound},
		{name: "a key not served", in: EnforcedInput{Config: enforce, ProjectFound: true, NotServed: []string{"count/x.example.com"}, Namespaces: ready},
			status: metav1.ConditionFalse, reason: v1alpha1.ProjectQuotaReasonKeyNotServed},
		{name: "a namespace not calculated", in: EnforcedInput{Config: enforce, ProjectFound: true, Namespaces: []Namespace{{Name: "a", Labelled: true}}},
			status: metav1.ConditionFalse, reason: v1alpha1.ProjectQuotaReasonKeyRollout},
		{name: "a namespace not labelled", in: EnforcedInput{Config: enforce, ProjectFound: true, Namespaces: []Namespace{{Name: "a", Calculated: true}}},
			status: metav1.ConditionFalse, reason: v1alpha1.ProjectQuotaReasonKeyRollout},
		{name: "an API server that does not call the webhook", in: EnforcedInput{Config: Config{ControlPlaneManaged: true, WebhookNotCalledBy: []string{"10.0.0.1:6443"}},
			ProjectFound: true, Namespaces: ready}, status: metav1.ConditionFalse, reason: v1alpha1.ProjectQuotaReasonWebhookNotCalled},
		{name: "a namespace without its quota", in: EnforcedInput{Config: enforce, ProjectFound: true, Namespaces: []Namespace{{Name: "a", Labelled: true, Problem: "forbidden"}}},
			status: metav1.ConditionFalse, reason: v1alpha1.ProjectQuotaReasonKeyRollout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnforcedCondition(tt.in)
			assert.Equal(t, tt.status, got.Status)
			assert.Equal(t, tt.reason, got.Reason)
		})
	}
}

func TestEnforcedConditionNamesTheProblem(t *testing.T) {
	got := EnforcedCondition(EnforcedInput{Config: Config{ControlPlaneManaged: true}, ProjectFound: true,
		Namespaces: []Namespace{{Name: "a", Calculated: true, Labelled: true}, {Name: "b", Problem: "create the quota: forbidden"}}})
	assert.Equal(t, metav1.ConditionFalse, got.Status)
	assert.Contains(t, got.Message, "The controller fails to set it up in b: create the quota: forbidden.")
}

func TestEnforcedConditionNamesSomeNamespaces(t *testing.T) {
	namespaces := make([]Namespace, 0, 15)
	for i := range 15 {
		namespaces = append(namespaces, Namespace{Name: fmt.Sprintf("ns-%02d", i)})
	}
	got := EnforcedCondition(EnforcedInput{Config: Config{ControlPlaneManaged: true}, ProjectFound: true, Namespaces: namespaces})
	assert.Contains(t, got.Message, "ns-09")
	assert.NotContains(t, got.Message, "ns-10")
	assert.Contains(t, got.Message, "and 5 more namespaces")
}

func TestWriteDue(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := v1alpha1.ProjectQuotaStatus{Hard: rl("pods", "1000"), Used: rl("pods", "100"), NamespaceCount: 2, ObservedGeneration: 1}
	with := func(edit func(s *v1alpha1.ProjectQuotaStatus)) *v1alpha1.ProjectQuotaStatus {
		out := base
		out.Hard, out.Used = base.Hard.DeepCopy(), base.Used.DeepCopy()
		edit(&out)
		return &out
	}
	tests := []struct {
		name      string
		desired   *v1alpha1.ProjectQuotaStatus
		lastWrite time.Time
		due       bool
		wait      time.Duration
	}{
		{name: "nothing changed", desired: with(func(*v1alpha1.ProjectQuotaStatus) {}), lastWrite: now.Add(-time.Hour)},
		{name: "a new generation", desired: with(func(s *v1alpha1.ProjectQuotaStatus) { s.ObservedGeneration = 2 }), lastWrite: now, due: true},
		{name: "a namespace more", desired: with(func(s *v1alpha1.ProjectQuotaStatus) { s.NamespaceCount = 3 }), lastWrite: now, due: true},
		{name: "a new condition", desired: with(func(s *v1alpha1.ProjectQuotaStatus) {
			s.Conditions = []metav1.Condition{{Type: v1alpha1.ProjectQuotaConditionExceeded, Status: metav1.ConditionFalse}}
		}), lastWrite: now, due: true},
		{name: "a small move", desired: with(func(s *v1alpha1.ProjectQuotaStatus) { s.Used = rl("pods", "105") }), lastWrite: now.Add(-time.Hour)},
		{name: "a move of 1% soon after a write", desired: with(func(s *v1alpha1.ProjectQuotaStatus) { s.Used = rl("pods", "110") }), lastWrite: now.Add(-4 * time.Second),
			wait: 6 * time.Second},
		{name: "a move of 1% later", desired: with(func(s *v1alpha1.ProjectQuotaStatus) { s.Used = rl("pods", "110") }), lastWrite: now.Add(-StatusWriteInterval), due: true},
	}
	// above the quota, the message of Exceeded shows the usage; that alone is throttled like the usage
	over := func(used string) *v1alpha1.ProjectQuotaStatus {
		return with(func(s *v1alpha1.ProjectQuotaStatus) {
			s.Hard, s.Used = rl("pods", "100", "secrets", "1"), rl("pods", used, "secrets", "1")
			s.Conditions = []metav1.Condition{exceededOver(s)}
		})
	}
	overBase := over("150")
	overTests := []struct {
		name    string
		desired *v1alpha1.ProjectQuotaStatus
		due     bool
	}{
		{name: "above the quota, a small move", desired: over("150")},
		{name: "above the quota, a small move of the usage in the message", desired: over("150.5")},
		{name: "back within the quota", desired: over("90"), due: true},
		{name: "another key above the quota", desired: with(func(s *v1alpha1.ProjectQuotaStatus) {
			s.Hard, s.Used = rl("pods", "100", "secrets", "1"), rl("pods", "150", "secrets", "1.005")
			s.Conditions = []metav1.Condition{exceededOver(s)}
		}), due: true},
	}
	for _, tt := range overTests {
		t.Run(tt.name, func(t *testing.T) {
			due, _ := WriteDue(overBase, tt.desired, now, now)
			assert.Equal(t, tt.due, due)
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			due, wait := WriteDue(&base, tt.desired, tt.lastWrite, now)
			assert.Equal(t, tt.due, due)
			assert.Equal(t, tt.wait, wait)
		})
	}
}

// exceededOver is the Exceeded condition of a status above its quota, with the usage in its message.
func exceededOver(s *v1alpha1.ProjectQuotaStatus) metav1.Condition {
	message := "The usage of the project is above the quota:"
	for _, key := range ExceededKeys(s.Hard, s.Used) {
		used, hard := s.Used[key], s.Hard[key]
		message += fmt.Sprintf(" %s: used %s, hard %s;", key, used.String(), hard.String())
	}
	return metav1.Condition{Type: v1alpha1.ProjectQuotaConditionExceeded, Status: metav1.ConditionTrue, Reason: v1alpha1.ProjectQuotaReasonHardLowered, Message: message}
}

func TestOvershootKeepsItsCause(t *testing.T) {
	within := &metav1.Condition{Type: v1alpha1.ProjectQuotaConditionExceeded, Status: metav1.ConditionFalse}
	first := ExceededCondition(within, rl("pods", "10"), rl("pods", "10"), rl("pods", "11"), 0, "the leader changed")
	assert.Contains(t, first.Message, "Probable cause: the leader changed.")
	later := ExceededCondition(&first, rl("pods", "10"), rl("pods", "10"), rl("pods", "12"), 0, "something else")
	assert.Equal(t, v1alpha1.ProjectQuotaReasonOvershoot, later.Reason)
	assert.Contains(t, later.Message, "pods: used 12")
	assert.Contains(t, later.Message, "Probable cause: the leader changed.")
}

func TestEnforcedByTheLedger(t *testing.T) {
	ready := []Namespace{{Name: "a", Calculated: true, Labelled: true}}
	got := EnforcedCondition(EnforcedInput{Config: Config{ControlPlaneManaged: true, Suspended: true}, ProjectFound: true, Namespaces: ready})
	assert.Equal(t, v1alpha1.ProjectQuotaReasonSuspended, got.Reason)
	got = EnforcedCondition(EnforcedInput{Config: Config{ControlPlaneManaged: true, WebhookMissing: true}, ProjectFound: true, Namespaces: ready})
	assert.Equal(t, v1alpha1.ProjectQuotaReasonWebhookMissing, got.Reason)
	assert.Equal(t, "Suspended", Config{ControlPlaneManaged: true, Suspended: true}.MetricMode())
}
