// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	gocontext "context"
	"fmt"
	"testing"

	klient "github.com/flant/kube-client/client"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

var nodeGroupGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}

// execCapableClient adds the one method libcon.KubeClient asks for on top of the
// fake apiserver client; nothing under test runs a pod command.
type execCapableClient struct {
	client.KubeClient
}

func (execCapableClient) Exec(gocontext.Context, *libcon.PodExecParams) error { return nil }

type staticKubeProvider struct {
	libcon.KubeProvider
	client libcon.KubeClient
}

func (p staticKubeProvider) Client(gocontext.Context) (libcon.KubeClient, error) {
	return p.client, nil
}

type unreachableKubeProvider struct {
	libcon.KubeProvider
}

func (unreachableKubeProvider) Client(gocontext.Context) (libcon.KubeClient, error) {
	return nil, fmt.Errorf("cluster is unreachable")
}

func clusterWithNodeGroups(t *testing.T, nodeGroups ...*unstructured.Unstructured) (*context.Context, *dynamicfake.FakeDynamicClient) {
	t.Helper()

	kubeCl := client.NewFakeKubernetesClientWithListGVR(
		map[schema.GroupVersionResource]string{nodeGroupGVR: "NodeGroupList"},
	)

	fakeClient, ok := kubeCl.KubeClient.(*klient.Client)
	require.True(t, ok, "fake kube client is not backed by kube-client")

	dyn, ok := fakeClient.Dynamic().(*dynamicfake.FakeDynamicClient)
	require.True(t, ok, "fake kube client is not backed by a fake dynamic client")

	for _, ng := range nodeGroups {
		_, err := dyn.Resource(nodeGroupGVR).Create(t.Context(), ng, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	convergeCtx := context.NewContext(t.Context(), context.Params{
		KubeProvider: staticKubeProvider{client: execCapableClient{kubeCl.KubeClient}},
	})

	return convergeCtx, dyn
}

func immutableNodeGroup(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"nodeType": "CloudPermanent", "systemType": "Immutable"},
	}}
}

// A NodeGroup can be deleted while its infrastructure state still lists machines.
// Converge is what removes those machines, so it must not stop on the missing group.
func TestIsImmutableNodeGroupTreatsMissingGroupAsMutable(t *testing.T) {
	convergeCtx, _ := clusterWithNodeGroups(t)

	immutable, err := isImmutableNodeGroup(convergeCtx, "worker")

	require.NoError(t, err)
	require.False(t, immutable)
}

// Every converged group starts with this read. A single apiserver hiccup used to
// fail the whole converge.
func TestIsImmutableNodeGroupRetriesTransientReadFailure(t *testing.T) {
	convergeCtx, dyn := clusterWithNodeGroups(t, immutableNodeGroup("master"))

	failed := false
	dyn.PrependReactor("get", "nodegroups", func(k8stesting.Action) (bool, runtime.Object, error) {
		if failed {
			return false, nil, nil
		}
		failed = true

		return true, nil, apierrors.NewInternalError(fmt.Errorf("apiserver is restarting"))
	})

	immutable, err := isImmutableNodeGroup(convergeCtx, "master")

	require.True(t, failed, "the transient failure was never served")
	require.NoError(t, err)
	require.True(t, immutable)
}

const staticClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: cluster.local
`

// Rendering one node's payload must not edit the configuration the converge run
// shares: the image digests and the cluster registry belong to that render alone,
// and in AutoConverge the context outlives the tick.
func TestImmutableMasterPayloadLeavesSharedMetaConfigUntouched(t *testing.T) {
	stateCache := cache.NewTestCache()
	require.NoError(t, stateCache.Save(t.Context(), "uuid", []byte("f1e9c0de-0000-4000-8000-000000000000")))

	convergeCtx := context.NewCommanderContext(t.Context(), context.Params{
		KubeProvider: unreachableKubeProvider{},
		Cache:        stateCache,
		Opts: &options.GlobalOptions{
			CandiDir:    immutabletest.CandiDir(t),
			DownloadDir: t.TempDir(),
		},
	}, &commander.CommanderModeParams{
		ClusterConfigurationData: []byte(staticClusterConfiguration),
	})

	shared, err := convergeCtx.MetaConfig()
	require.NoError(t, err)
	require.Empty(t, shared.Images, "the configuration parsed from the cluster carries no digests")

	_, err = immutableNodePayload(convergeCtx, "master", "cluster-master-0")
	require.Error(t, err, "the payload needs the cluster it cannot reach here")

	require.Empty(t, shared.Images, "the payload loaded its image digests into the shared configuration")
}
