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

package profile

import (
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/domain/fsm"
)

// TestTimingsReadsTheProfileByTheCRDSchema decodes a profile written in the
// field names of the CRD, so a wrong JSON name in the Go type fails here.
func TestTimingsReadsTheProfileByTheCRDSchema(t *testing.T) {
	const manifest = `
apiVersion: node-manager.deckhouse.io/v1alpha1
kind: FencingSLAProfile
metadata:
  name: critical
spec:
  controlPlaneLostAfter: 3s
  memberlist:
    probeInterval: 100ms
  watchdog:
    timeout: 7s
    feedInterval: 500ms
    margin: 400ms
  heartbeat:
    interval: 1s
    apiTimeout: 3s
  eviction:
    writerWaveInterval: 2s
`

	var profile v1alpha1.FencingSLAProfile
	if err := yaml.Unmarshal([]byte(manifest), &profile); err != nil {
		t.Fatalf("unmarshal profile: %v", err)
	}

	got := timings(&profile)

	// The silent delay is 3s + 200ms + 7s + 400ms + 900ms.
	want := fsm.Params{
		HeartbeatTTL:           5 * time.Second,
		AttestedNodeTaintAfter: 7400 * time.Millisecond,
		HeartbeatInterval:      time.Second,
		SilentNodeTaintAfter:   11500 * time.Millisecond,
	}
	if got != want {
		t.Errorf("read %+v, want %+v", got, want)
	}
}
