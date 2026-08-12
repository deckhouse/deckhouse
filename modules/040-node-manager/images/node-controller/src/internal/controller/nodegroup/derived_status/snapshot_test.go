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

package derived_status

import (
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// The snapshot is the package's whole input. Building it in one place is what makes the derive and
// the validate halves see the same world: they used to read the zones, the InstanceClass and the
// type catalog once each.
func TestBuildSnapshot_StaticNodeGroupReadsStaticConfigOnly(t *testing.T) {
	s := newTestService(t, testSecret(staticConfigSecretNamespace, staticConfigSecretName, map[string][]byte{
		staticConfigKey: []byte("internalNetworkCIDRs:\n- 10.0.0.0/24\n"),
	}))
	ng := &v1.NodeGroup{}
	ng.Name = "master"
	ng.Spec.NodeType = v1.NodeTypeStatic

	snap, err := s.BuildSnapshot(t.Context(), ng)

	require.NoError(t, err)
	require.NotNil(t, snap.StaticConfig)
	require.Empty(t, snap.Provider.Type)
	require.Nil(t, snap.InstanceClass)
	require.Nil(t, snap.DefaultZones, "zones belong to the cloud overlay a Static NodeGroup never gets")
}

// An unpublished version leaves the InstanceClass unread rather than read at a guessed version: a
// guess goes through the provider's conversion webhook, changes the spec, and renames the immutable
// machine template the checksum points at.
func TestBuildSnapshot_CloudEphemeralWithoutPublishedVersionSkipsInstanceClass(t *testing.T) {
	s := newTestService(t, testSecret(cloudProviderSecretNamespace, cloudProviderSecretName, map[string][]byte{
		"type":              []byte(`aws`),
		"instanceClassKind": []byte(`AWSInstanceClass`),
	}))
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}

	snap, err := s.BuildSnapshot(t.Context(), ng)

	require.NoError(t, err)
	require.Empty(t, snap.Provider.InstanceClassAPIVersion)
	require.Nil(t, snap.InstanceClass)
	require.Nil(t, snap.KnownClassNames)
}

// A cluster with no cloud provider at all yields an empty snapshot, not an error: that is a static
// cluster, and every Static NodeGroup in it must still resolve.
func TestBuildSnapshot_NoCloudProviderIsNotAnError(t *testing.T) {
	s := newTestService(t)
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral

	snap, err := s.BuildSnapshot(t.Context(), ng)

	require.NoError(t, err)
	require.Empty(t, snap.Provider.InstanceClassKind)
	require.Nil(t, snap.InstanceClass)
}

// An unreadable source must abort the pass: an empty provider reads as "no cloud", which drops
// instanceClass from the published element and re-runs bashible on every node.
func TestBuildSnapshot_UnreadableProviderAborts(t *testing.T) {
	s := newDeniedSecretService(t, cloudProviderSecretName)
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral

	_, err := s.BuildSnapshot(t.Context(), ng)

	require.ErrorContains(t, err, "read cloud provider secret")
}
