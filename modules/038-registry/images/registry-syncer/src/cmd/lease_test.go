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

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/run"
)

// TestTheLeaseSaysWhoLeads: a holder renewing is live; one that stopped renewing a moment ago still
// leads, so that a minute without the API server does not restart an incomplete fill elsewhere; one
// gone for longer, or a released lease, names nobody, and the election is open.
func TestTheLeaseSaysWhoLeads(t *testing.T) {
	renewed := time.Date(2026, 9, 30, 18, 37, 0, 0, time.UTC)
	lease := func(holder string) *coordinationv1.Lease {
		return &coordinationv1.Lease{Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To(holder),
			RenewTime:            &metav1.MicroTime{Time: renewed},
			LeaseDurationSeconds: ptr.To[int32](30),
		}}
	}

	assert.Equal(t, run.Lease{Known: true, Holder: "master-0", Live: true},
		leaseAt(lease("master-0"), renewed.Add(10*time.Second)))
	assert.Equal(t, run.Lease{Known: true, Holder: "master-0"},
		leaseAt(lease("master-0"), renewed.Add(31*time.Second)), "expired a moment ago: still the one leading")
	assert.Equal(t, run.Lease{Known: true},
		leaseAt(lease("master-0"), renewed.Add(91*time.Second)), "gone: nobody leads")
	assert.Equal(t, run.Lease{Known: true}, leaseAt(lease(""), renewed), "released on stepping aside")
	assert.Equal(t, run.Lease{Known: true}, leaseAt(&coordinationv1.Lease{}, renewed), "never held")
}

// TestAReplicaWithoutAPodIsNotInTheElection: a report outlives its pod. The leader's replica removed,
// its pod's replacement waiting unscheduled on a cordoned node, the others read its last report — the
// fullest, and role Leader — and waited for it.
func TestAReplicaWithoutAPodIsNotInTheElection(t *testing.T) {
	replicas := []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0"}, {Node: "master-1"}, {Node: "master-2"}, {Node: "master-3"},
	}
	pod := func(node string, phase corev1.PodPhase, deleting bool) corev1.Pod {
		p := corev1.Pod{Spec: corev1.PodSpec{NodeName: node}, Status: corev1.PodStatus{Phase: phase}}
		if deleting {
			p.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		}
		return p
	}
	pods := []corev1.Pod{
		pod("master-0", corev1.PodRunning, false),
		pod("", corev1.PodPending, false), // master-1's replacement, not scheduled anywhere
		pod("master-2", corev1.PodRunning, true),
		pod("master-3", corev1.PodPending, false), // scheduled, still starting
	}

	var nodes []string
	for _, replica := range replicasOnNodes("master-2", replicas, pods) {
		nodes = append(nodes, replica.Node)
	}
	assert.Equal(t, []string{"master-0", "master-2", "master-3"}, nodes,
		"master-1 has no pod; master-2 is the one asking, and always stands for itself")
}
