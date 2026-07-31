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

package conditions

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func TestCalculateConditionSummary(t *testing.T) {
	tests := []struct {
		name      string
		statusMsg string
		wantReady string
	}{
		{name: "no message is ready", statusMsg: "", wantReady: "True"},
		{name: "with message is not ready", statusMsg: "Machine creation failed.", wantReady: "False"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateConditionSummary(nil, tt.statusMsg)
			if got.Ready != tt.wantReady {
				t.Errorf("Ready = %q, want %q", got.Ready, tt.wantReady)
			}
			if got.StatusMessage != tt.statusMsg {
				t.Errorf("StatusMessage = %q, want %q", got.StatusMessage, tt.statusMsg)
			}
		})
	}
}

func drainEvents(rec *record.FakeRecorder) []string {
	var events []string
	for {
		select {
		case e := <-rec.Events:
			events = append(events, e)
		default:
			return events
		}
	}
}

func TestCreateEventIfChanged_DeduplicatesByMessage(t *testing.T) {
	rec := record.NewFakeRecorder(10)
	s := &Service{Recorder: rec}
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}

	s.CreateEventIfChanged(ng, "boom")
	s.CreateEventIfChanged(ng, "boom") // same message, should be suppressed
	s.CreateEventIfChanged(ng, "bang") // new message, recorded

	events := drainEvents(rec)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %v", len(events), events)
	}
	if !strings.Contains(events[0], "Warning") || !strings.Contains(events[0], "MachineFailed") {
		t.Errorf("expected first event to be Warning/MachineFailed, got %q", events[0])
	}
}

func TestCreateEventIfChanged_MachineCreatingIsNormal(t *testing.T) {
	rec := record.NewFakeRecorder(10)
	s := &Service{Recorder: rec}
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}

	s.CreateEventIfChanged(ng, "Started Machine creation process")

	events := drainEvents(rec)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !strings.Contains(events[0], "Normal") || !strings.Contains(events[0], "MachineCreating") {
		t.Errorf("expected Normal/MachineCreating event, got %q", events[0])
	}
}

func TestCreateEventIfChanged_PerNodeGroupTracking(t *testing.T) {
	rec := record.NewFakeRecorder(10)
	s := &Service{Recorder: rec}
	ngA := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	ngB := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "b"}}

	s.CreateEventIfChanged(ngA, "boom")
	s.CreateEventIfChanged(ngB, "boom") // different NG, same message -> still recorded

	events := drainEvents(rec)
	if len(events) != 2 {
		t.Fatalf("expected 2 events across two nodegroups, got %d", len(events))
	}
}
