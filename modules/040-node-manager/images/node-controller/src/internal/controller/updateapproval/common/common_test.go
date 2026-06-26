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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func TestBuildNodeInfo(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want NodeInfo
	}{
		{
			name: "nil annotations, no labels, not ready",
			node: &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n0"}},
			want: NodeInfo{Name: "n0"},
		},
		{
			name: "all approval annotations and ready condition",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "n1",
					Labels: map[string]string{NodeGroupLabel: "worker"},
					Annotations: map[string]string{
						ConfigurationChecksumAnnotation: "abc",
						ApprovedAnnotation:              "",
						WaitingForApprovalAnnotation:    "",
						DisruptionRequiredAnnotation:    "",
						DisruptionApprovedAnnotation:    "",
						RollingUpdateAnnotation:         "",
						DrainingAnnotation:              "bashible",
						DrainedAnnotation:               "bashible",
					},
				},
				Spec: corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			},
			want: NodeInfo{
				Name:                  "n1",
				NodeGroup:             "worker",
				ConfigurationChecksum: "abc",
				IsReady:               true,
				IsApproved:            true,
				IsWaitingForApproval:  true,
				IsDisruptionRequired:  true,
				IsDisruptionApproved:  true,
				IsRollingUpdate:       true,
				IsUnschedulable:       true,
				IsDraining:            true,
				IsDrained:             true,
			},
		},
		{
			name: "draining/drained with non-bashible value is not set",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "n2",
					Annotations: map[string]string{
						DrainingAnnotation: "manual",
						DrainedAnnotation:  "manual",
					},
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					},
				},
			},
			want: NodeInfo{Name: "n2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildNodeInfo(tt.node)
			if got != tt.want {
				t.Fatalf("BuildNodeInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetApprovalMode(t *testing.T) {
	tests := []struct {
		name string
		ng   *v1.NodeGroup
		want string
	}{
		{
			name: "no disruptions defaults to Automatic",
			ng:   &v1.NodeGroup{},
			want: "Automatic",
		},
		{
			name: "disruptions with empty mode defaults to Automatic",
			ng:   &v1.NodeGroup{Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{}}},
			want: "Automatic",
		},
		{
			name: "explicit Manual mode",
			ng: &v1.NodeGroup{Spec: v1.NodeGroupSpec{
				Disruptions: &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalModeManual},
			}},
			want: "Manual",
		},
		{
			name: "explicit RollingUpdate mode",
			ng: &v1.NodeGroup{Spec: v1.NodeGroupSpec{
				Disruptions: &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalModeRollingUpdate},
			}},
			want: "RollingUpdate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetApprovalMode(tt.ng); got != tt.want {
				t.Fatalf("GetApprovalMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCalculateConcurrency(t *testing.T) {
	intVal := intstr.FromInt32(3)
	percent50 := intstr.FromString("50%")
	percent1 := intstr.FromString("1%")
	percentInvalid := intstr.FromString("abc%")
	plainString := intstr.FromString("4")

	tests := []struct {
		name          string
		maxConcurrent *intstr.IntOrString
		totalNodes    int
		want          int
	}{
		{name: "nil defaults to 1", maxConcurrent: nil, totalNodes: 10, want: 1},
		{name: "int value used directly", maxConcurrent: &intVal, totalNodes: 10, want: 3},
		{name: "percentage of total nodes", maxConcurrent: &percent50, totalNodes: 10, want: 5},
		{name: "percentage rounding to zero clamps to 1", maxConcurrent: &percent1, totalNodes: 10, want: 1},
		{name: "invalid percentage parses to zero then clamps to 1", maxConcurrent: &percentInvalid, totalNodes: 10, want: 1},
		{name: "non-percent string falls back to IntValue", maxConcurrent: &plainString, totalNodes: 10, want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateConcurrency(tt.maxConcurrent, tt.totalNodes); got != tt.want {
				t.Fatalf("CalculateConcurrency() = %d, want %d", got, tt.want)
			}
		})
	}
}

// 2021-01-01 is a Friday.
func mustTime(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		t.Fatalf("parse time %q: %v", hhmm, err)
	}
	return time.Date(2021, 1, 1, parsed.Hour(), parsed.Minute(), 0, 0, time.UTC)
}

func TestIsWindowAllowed(t *testing.T) {
	tests := []struct {
		name   string
		window v1.DisruptionWindow
		now    time.Time
		want   bool
	}{
		{
			name:   "inside normal window",
			window: v1.DisruptionWindow{From: "08:00", To: "18:00"},
			now:    mustTime(t, "12:00"),
			want:   true,
		},
		{
			name:   "before normal window",
			window: v1.DisruptionWindow{From: "08:00", To: "18:00"},
			now:    mustTime(t, "07:00"),
			want:   false,
		},
		{
			name:   "after normal window",
			window: v1.DisruptionWindow{From: "08:00", To: "18:00"},
			now:    mustTime(t, "19:00"),
			want:   false,
		},
		{
			name:   "exactly at window start is allowed",
			window: v1.DisruptionWindow{From: "08:00", To: "18:00"},
			now:    mustTime(t, "08:00"),
			want:   true,
		},
		{
			name:   "exactly at window end is allowed",
			window: v1.DisruptionWindow{From: "08:00", To: "18:00"},
			now:    mustTime(t, "18:00"),
			want:   true,
		},
		{
			name:   "invalid from time",
			window: v1.DisruptionWindow{From: "bad", To: "18:00"},
			now:    mustTime(t, "12:00"),
			want:   false,
		},
		{
			name:   "invalid to time",
			window: v1.DisruptionWindow{From: "08:00", To: "bad"},
			now:    mustTime(t, "12:00"),
			want:   false,
		},
		{
			name:   "day not allowed",
			window: v1.DisruptionWindow{From: "08:00", To: "18:00", Days: []string{"Mon"}},
			now:    mustTime(t, "12:00"),
			want:   false,
		},
		{
			name:   "day allowed",
			window: v1.DisruptionWindow{From: "08:00", To: "18:00", Days: []string{"Fri"}},
			now:    mustTime(t, "12:00"),
			want:   true,
		},
		{
			name:   "overnight window matches early morning",
			window: v1.DisruptionWindow{From: "22:00", To: "06:00"},
			now:    mustTime(t, "03:00"),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWindowAllowed(tt.window, tt.now); got != tt.want {
				t.Fatalf("IsWindowAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInAllowedWindow(t *testing.T) {
	now := mustTime(t, "12:00")

	tests := []struct {
		name    string
		windows []v1.DisruptionWindow
		want    bool
	}{
		{name: "no windows is always allowed", windows: nil, want: true},
		{
			name: "first window matches",
			windows: []v1.DisruptionWindow{
				{From: "08:00", To: "18:00"},
				{From: "20:00", To: "22:00"},
			},
			want: true,
		},
		{
			name: "second window matches",
			windows: []v1.DisruptionWindow{
				{From: "00:00", To: "01:00"},
				{From: "08:00", To: "18:00"},
			},
			want: true,
		},
		{
			name: "no window matches",
			windows: []v1.DisruptionWindow{
				{From: "00:00", To: "01:00"},
				{From: "20:00", To: "22:00"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInAllowedWindow(tt.windows, now); got != tt.want {
				t.Fatalf("IsInAllowedWindow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDayAllowed(t *testing.T) {
	friday := mustTime(t, "12:00")

	tests := []struct {
		name string
		days []string
		want bool
	}{
		{name: "empty days allows any day", days: nil, want: true},
		{name: "matching short day", days: []string{"Fri"}, want: true},
		{name: "matching long day", days: []string{"Friday"}, want: true},
		{name: "non-matching day", days: []string{"Mon", "Tue"}, want: false},
		{name: "match among several", days: []string{"Mon", "Fri"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDayAllowed(friday, tt.days); got != tt.want {
				t.Fatalf("IsDayAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDayEqual(t *testing.T) {
	// Anchor each weekday to a known date in early January 2021.
	// 2021-01-04 is Monday ... 2021-01-10 is Sunday.
	day := func(d int) time.Time { return time.Date(2021, 1, d, 0, 0, 0, 0, time.UTC) }

	tests := []struct {
		name      string
		today     time.Time
		dayString string
		want      bool
	}{
		{name: "monday short", today: day(4), dayString: "mon", want: true},
		{name: "monday long", today: day(4), dayString: "Monday", want: true},
		{name: "tuesday", today: day(5), dayString: "tue", want: true},
		{name: "wednesday", today: day(6), dayString: "wednesday", want: true},
		{name: "thursday", today: day(7), dayString: "thu", want: true},
		{name: "friday", today: day(8), dayString: "friday", want: true},
		{name: "saturday", today: day(9), dayString: "sat", want: true},
		{name: "sunday", today: day(10), dayString: "sunday", want: true},
		{name: "mismatch returns false", today: day(4), dayString: "tue", want: false},
		{name: "unknown day string returns false", today: day(4), dayString: "funday", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDayEqual(tt.today, tt.dayString); got != tt.want {
				t.Fatalf("IsDayEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
