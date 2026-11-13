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

package v1alpha1

import (
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Local clients read the section by these names, and the reads come faster
// than once a second, so the timestamps must keep their microseconds.
func TestKubernetesAPIViewWireFormat(t *testing.T) {
	read := metav1.NewMicroTime(time.Date(2026, time.June, 2, 15, 0, 2, 250000000, time.UTC))
	visible := metav1.NewMicroTime(time.Date(2026, time.June, 2, 15, 0, 1, 850000000, time.UTC))

	raw, err := json.Marshal(FencingAgentKubernetesAPIView{
		CPSource:      "proxy",
		LastReadAt:    &read,
		LastReadOK:    true,
		LastVisibleAt: &visible,
	})
	if err != nil {
		t.Fatalf("marshal kubernetesAPI: %v", err)
	}

	want := `{"cpSource":"proxy","lastReadAt":"2026-06-02T15:00:02.250000Z","lastReadOK":true,` +
		`"lastVisibleAt":"2026-06-02T15:00:01.850000Z"}`
	if string(raw) != want {
		t.Errorf("kubernetesAPI marshaled as\n%s\nwant\n%s", raw, want)
	}
}

// Sections are values: a view built before the first read still carries the
// kubernetesAPI section, with its timestamps null.
func TestKubernetesAPIViewIsPresentBeforeTheFirstRead(t *testing.T) {
	raw, err := json.Marshal(FencingAgentNodeViewStatus{})
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}

	var status map[string]json.RawMessage
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}

	want := `{"cpSource":"","lastReadAt":null,"lastReadOK":false,"lastVisibleAt":null}`
	if got := string(status["kubernetesAPI"]); got != want {
		t.Errorf("kubernetesAPI of an empty status is %s, want %s", got, want)
	}
}

func TestKubernetesAPIViewDeepCopyIsIndependent(t *testing.T) {
	read := metav1.NewMicroTime(time.Date(2026, time.June, 2, 15, 0, 2, 0, time.UTC))
	original := &FencingAgentNodeView{
		Status: FencingAgentNodeViewStatus{
			KubernetesAPI: FencingAgentKubernetesAPIView{LastReadAt: &read, LastVisibleAt: &read},
		},
	}

	copied := original.DeepCopy()
	copied.Status.KubernetesAPI.LastReadAt.Time = read.Add(time.Hour)
	copied.Status.KubernetesAPI.LastVisibleAt.Time = read.Add(time.Hour)

	if !original.Status.KubernetesAPI.LastReadAt.Equal(&read) ||
		!original.Status.KubernetesAPI.LastVisibleAt.Equal(&read) {
		t.Errorf("kubernetesAPI section aliased: %+v", original.Status.KubernetesAPI)
	}
}
