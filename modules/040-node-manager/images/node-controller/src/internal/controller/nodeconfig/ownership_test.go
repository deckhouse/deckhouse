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

package nodeconfig

import (
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// The node registers its object with everything it booted with, and node-controller
// applies only what it renders. The machine's fields stay the machine's; whatever the
// render no longer produces leaves the object instead of lingering under the node's
// ownership of the same value.
func TestTheApplyKeepsTheMachineFieldsAndDropsWhatTheRenderLeftOut(t *testing.T) {
	c := nodeConfigAPIClient(t)
	ctx := t.Context()

	registered := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "owned-0"},
		Spec: internalv1alpha1.NodeSpec{
			SystemType:  internalv1alpha1.SystemTypeImmutable,
			NodeName:    "owned-0",
			OSImage:     internalv1alpha1.OSImage{Digest: "sha256:" + strings.Repeat("a", 64)},
			StatusToken: "status-token",
			Storage: internalv1alpha1.Storage{
				Disk:   internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: ">=30Gi"}},
				Mounts: []internalv1alpha1.Mount{{Name: "etcd", Device: "/dev/sdb1", BindTo: "/var/lib/etcd"}},
			},
			Network: internalv1alpha1.Network{
				Hostname:   "owned-0",
				Interfaces: []internalv1alpha1.NetworkInterface{{Name: "ens3", Addresses: []string{"192.0.2.10/24"}}},
			},
			Kubelet: internalv1alpha1.Kubelet{
				MaxPods:            110,
				ClusterCABundle:    "ca",
				NodeLabels:         map[string]internalv1alpha1.NodeLabelValue{"role": "worker", "stale": "yes"},
				RegisterWithTaints: []internalv1alpha1.Taint{{Key: "dedicated", Effect: "NoSchedule"}},
			},
			Extensions: []internalv1alpha1.Extension{
				{Name: "nodelet", Digest: "sha256:" + strings.Repeat("b", 64)},
				{Name: "retired", Digest: "sha256:" + strings.Repeat("c", 64)},
			},
		},
	}
	require.NoError(t, c.Create(ctx, registered, client.FieldOwner("nodelet")))

	rendered := withoutMachineFields(registered.Spec)
	rendered.Kubelet.MaxPods = 0
	rendered.Kubelet.NodeLabels = map[string]internalv1alpha1.NodeLabelValue{"role": "worker"}
	rendered.Extensions = rendered.Extensions[:1]
	rendered.Kubelet.ServerTLSBootstrap = ptr.To(true)

	r := &Reconciler{}
	r.Client = c
	existing := &internalv1alpha1.NodeConfig{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "owned-0"}, existing))
	existing, err := r.takeOver(ctx, existing, logr.Discard())
	require.NoError(t, err)
	require.NotNil(t, existing)
	current, err := r.applySpec(ctx, existing, rendered, logr.Discard())
	require.NoError(t, err)
	require.NotNil(t, current)

	stored := &internalv1alpha1.NodeConfig{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "owned-0"}, stored))
	require.Equal(t, current.Generation, stored.Generation, "the object is returned as it now stands")

	// The machine's own.
	require.Equal(t, registered.Spec.StatusToken, stored.Spec.StatusToken)
	require.Equal(t, registered.Spec.Storage, stored.Spec.Storage)
	require.Equal(t, registered.Spec.Network, stored.Spec.Network)
	require.Equal(t, registered.Spec.Kubelet.RegisterWithTaints, stored.Spec.Kubelet.RegisterWithTaints)

	// The render's, as rendered: what it left out is gone.
	require.Zero(t, stored.Spec.Kubelet.MaxPods, "a setting the group dropped stays on the node")
	require.Equal(t, rendered.Kubelet.NodeLabels, stored.Spec.Kubelet.NodeLabels)
	require.Equal(t, []string{"nodelet"}, extensionNames(stored.Spec.Extensions))
	require.Equal(t, ptr.To(true), stored.Spec.Kubelet.ServerTLSBootstrap)

	// And a second pass finds nothing left to take over.
	_, taken, err := takeOverSpec(stored.ManagedFields, metav1.Now())
	require.NoError(t, err)
	require.False(t, taken)
}

func extensionNames(extensions []internalv1alpha1.Extension) []string {
	names := make([]string, 0, len(extensions))
	for _, ext := range extensions {
		names = append(names, ext.Name)
	}
	return names
}

// The takeover moves spec fields only, never the machine's, and leaves the status
// and the metadata to whoever owns them.
func TestTakeOverSpecMovesOnlyTheRenderedSpec(t *testing.T) {
	fields := func(raw string) *metav1.FieldsV1 { return &metav1.FieldsV1{Raw: []byte(raw)} }
	entries := []metav1.ManagedFieldsEntry{
		{
			Manager: "nodelet", Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: "internal.deckhouse.io/v1alpha1",
			FieldsType: "FieldsV1",
			FieldsV1: fields(`{"f:metadata":{"f:labels":{".":{},"f:node.deckhouse.io/group":{}}},` +
				`"f:spec":{".":{},"f:kubelet":{".":{},"f:maxPods":{},"f:registerWithTaints":{}},"f:network":{".":{},"f:hostname":{}},"f:statusToken":{}}}`),
		},
		{
			Manager: "nodelet", Operation: metav1.ManagedFieldsOperationApply, Subresource: "status",
			APIVersion: "internal.deckhouse.io/v1alpha1", FieldsType: "FieldsV1", FieldsV1: fields(`{"f:status":{"f:phase":{}}}`),
		},
		{
			Manager: "kubectl", Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: "internal.deckhouse.io/v1alpha1",
			FieldsType: "FieldsV1", FieldsV1: fields(`{"f:spec":{"f:kernel":{"f:sysctl":{"f:vm.swappiness":{}}}}}`),
		},
	}

	kept, taken, err := takeOverSpec(entries, metav1.Now())
	require.NoError(t, err)
	require.True(t, taken)

	byManager := map[string]string{}
	for _, entry := range kept {
		byManager[entry.Manager+"/"+string(entry.Operation)+"/"+entry.Subresource] = string(entry.FieldsV1.Raw)
	}
	require.Len(t, byManager, 3, "kubectl owned spec fields only and is dropped: %v", byManager)
	require.Equal(t, `{"f:metadata":{"f:labels":{".":{},"f:node.deckhouse.io/group":{}}},`+
		`"f:spec":{"f:kubelet":{"f:registerWithTaints":{}},"f:network":{".":{},"f:hostname":{}},"f:statusToken":{}}}`,
		byManager["nodelet/Update/"])
	require.Equal(t, `{"f:status":{"f:phase":{}}}`, byManager["nodelet/Apply/status"])
	require.Equal(t, `{"f:spec":{".":{},"f:kernel":{"f:sysctl":{"f:vm.swappiness":{}}},"f:kubelet":{".":{},"f:maxPods":{}}}}`,
		byManager[fieldManager+"/Apply/"])
}
