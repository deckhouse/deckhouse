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

package writer

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

// witnessCluster runs the writer w-0. The victim w-1 is alive and does not run a writer.
func witnessCluster(t *testing.T, maintenance *atomic.Bool, metaOf func(string) domain.NodeMeta) (*cluster, *Writer) {
	t.Helper()

	all := names("w", 2)
	c := newCluster(t, all, clusterOptions{maintenance: maintenance, down: all[1:]}, metaOf)

	return c, c.writers["w-0"]
}

func wantAttestation(t *testing.T, c *cluster, victim string) *v1alpha1.FencingFailedNodeState {
	t.Helper()

	ffns, ok := c.api.get(victim)
	if !ok {
		t.Fatalf("no FFNS %s", victim)
	}

	if a := ffns.Status.Attestation; a == nil || a.Witness != "w-0" || a.At.IsZero() {
		t.Fatalf("attestation = %+v, want the witness w-0", ffns.Status.Attestation)
	}

	return ffns
}

// No FFNS: the witness creates it with the source DeclareSuicide and the
// data of the victim from its NodeMeta, then writes the attestation.
func TestWitnessCreatesTheFFNS(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := witnessCluster(t, nil, armedMeta)

		w.Attest("w-1")
		synctest.Wait()

		ffns := wantAttestation(t, c, "w-1")
		want := v1alpha1.FencingFailedNodeStateSpec{
			Source: v1alpha1.SourceDeclareSuicide, DetectedBy: "w-0", DetectedAt: ffns.Spec.DetectedAt,
			BootID: "boot-w-1", Armed: true, ProfileRef: &v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
		}

		if ffns.Spec.Source != want.Source || ffns.Spec.DetectedBy != want.DetectedBy || ffns.Spec.BootID != want.BootID ||
			!ffns.Spec.Armed || ffns.Spec.ProfileRef == nil || *ffns.Spec.ProfileRef != *want.ProfileRef {
			t.Errorf("spec = %+v, want %+v", ffns.Spec, want)
		}

		if ref := ffns.OwnerReferences; len(ref) != 1 || ref[0].UID != "uid-w-1" {
			t.Errorf("owner references = %+v, want the Node w-1", ref)
		}
	})
}

// The FFNS exists: the witness adds the attestation and does not create.
func TestWitnessAttestsTheExistingFFNS(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := witnessCluster(t, nil, armedMeta)

		c.api.add(&v1alpha1.FencingFailedNodeState{
			ObjectMeta: metav1.ObjectMeta{Name: "w-1"},
			Spec:       v1alpha1.FencingFailedNodeStateSpec{Source: v1alpha1.SourceMemberlistDead, DetectedBy: "w-5"},
			Status:     v1alpha1.FencingFailedNodeStateStatus{Phase: v1alpha1.PhaseSuspected},
		})

		w.Attest("w-1")
		synctest.Wait()

		ffns := wantAttestation(t, c, "w-1")
		if ffns.Spec.Source != v1alpha1.SourceMemberlistDead || ffns.Status.Phase != v1alpha1.PhaseSuspected || len(c.api.calls()) != 0 {
			t.Errorf("FFNS = %+v, creates %d: want the attestation in the existing FFNS only", ffns, len(c.api.calls()))
		}
	})
}

// The FFNS appears between the read and the create: AlreadyExists gives the
// attestation in the existing FFNS.
func TestWitnessAttestsOnAlreadyExists(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := witnessCluster(t, nil, armedMeta)

		c.api.onCreate = func(_ context.Context, ffns *v1alpha1.FencingFailedNodeState) error {
			c.api.add(&v1alpha1.FencingFailedNodeState{
				ObjectMeta: metav1.ObjectMeta{Name: ffns.Name},
				Spec:       v1alpha1.FencingFailedNodeStateSpec{Source: v1alpha1.SourceMemberlistDead},
			})

			return nil
		}

		w.Attest("w-1")
		synctest.Wait()

		if ffns := wantAttestation(t, c, "w-1"); ffns.Spec.Source != v1alpha1.SourceMemberlistDead {
			t.Errorf("source = %q, want the FFNS that exists", ffns.Spec.Source)
		}
	})
}

// A witness in maintenance does not create the FFNS and does not add the
// attestation. A victim in maintenance gets no FFNS.
func TestWitnessDoesNotWrite(t *testing.T) {
	tests := []struct {
		name        string
		maintenance bool
		meta        func(string) domain.NodeMeta
		exists      bool
	}{
		{name: "witness in maintenance, FFNS exists", maintenance: true, meta: armedMeta, exists: true},
		{name: "witness in maintenance, no FFNS", maintenance: true, meta: armedMeta},
		{name: "victim in maintenance", meta: func(name string) domain.NodeMeta {
			m := armedMeta(name)
			m.Maintenance = true

			return m
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				maintenance := &atomic.Bool{}
				c, w := witnessCluster(t, maintenance, tt.meta)
				maintenance.Store(tt.maintenance)

				if tt.exists {
					c.api.add(&v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "w-1"}})
				}

				w.Attest("w-1")
				synctest.Wait()

				if len(c.api.calls()) != 0 || len(c.api.patched()) != 0 {
					t.Fatalf("creates %d, status writes %d, want none", len(c.api.calls()), len(c.api.patched()))
				}
			})
		})
	}
}

// An attestation that exists stays.
func TestWitnessKeepsTheAttestation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, w := witnessCluster(t, nil, armedMeta)

		c.api.add(&v1alpha1.FencingFailedNodeState{
			ObjectMeta: metav1.ObjectMeta{Name: "w-1"},
			Status: v1alpha1.FencingFailedNodeStateStatus{
				Attestation: &v1alpha1.FencingFailedNodeStateAttestation{Witness: "w-7", At: metav1.NowMicro()},
			},
		})

		w.Attest("w-1")
		synctest.Wait()

		if len(c.api.patched()) != 0 {
			t.Fatal("the witness wrote over the attestation of another witness")
		}
	})
}
