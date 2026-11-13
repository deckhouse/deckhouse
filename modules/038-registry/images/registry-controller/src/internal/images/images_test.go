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

package images

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/layout"
)

const (
	shared   = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	broken   = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	pulled   = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	declared = "sha256:4444444444444444444444444444444444444444444444444444444444444444"
)

var owner = registryimagev1alpha1.ImageOwner{
	Kind: registryimagev1alpha1.OwnerPlatform, Module: "prometheus", Image: "prometheus", Versions: []string{"v1.70.1"},
}

func index(node string, images ...registryimagev1alpha1.IndexedImage) *registryimagev1alpha1.ReplicaIndex {
	at := metav1.Now()
	return &registryimagev1alpha1.ReplicaIndex{Node: node, Ready: true, SetKnown: true, IndexedAt: &at, Images: images}
}

func TestTheReplicasAreMergedImageByImage(t *testing.T) {
	images := Merge([]Answer{
		{Node: "master-0", Index: index("master-0",
			registryimagev1alpha1.IndexedImage{Digest: shared, State: registryimagev1alpha1.ImagePresent, InSet: true, Owners: []registryimagev1alpha1.ImageOwner{owner}, Size: 100, ReclaimableSize: 80, Repositories: []string{"system/deckhouse"}},
			registryimagev1alpha1.IndexedImage{Digest: broken, State: registryimagev1alpha1.ImageCorrupted, InSet: true, CorruptedBlobs: []string{"sha256:ff"}},
			registryimagev1alpha1.IndexedImage{Digest: declared, State: registryimagev1alpha1.ImagePresent, InSet: true, Size: 10},
		)},
		{Node: "master-1", Index: index("master-1",
			registryimagev1alpha1.IndexedImage{Digest: shared, State: registryimagev1alpha1.ImagePresent, InSet: true, Owners: []registryimagev1alpha1.ImageOwner{owner}, Size: 100, ReclaimableSize: 80},
			registryimagev1alpha1.IndexedImage{Digest: broken, State: registryimagev1alpha1.ImagePresent, InSet: true},
			registryimagev1alpha1.IndexedImage{Digest: pulled, State: registryimagev1alpha1.ImagePresent},
		)},
		{Node: "master-2", Err: errors.New("connection refused")},
	})

	byName := map[string]registryimagev1alpha1.RegistryImage{}
	for _, image := range images {
		byName[image.Name] = image
	}
	require.Len(t, byName, 4)

	image := byName["sha256-1111111111111111111111111111111111111111111111111111111111111111"]
	assert.Equal(t, shared, image.Digest)
	assert.Equal(t, registryimagev1alpha1.ImageUnknown, image.State, "a replica that did not answer is Unknown")
	require.Len(t, image.Replicas, 3)
	assert.Equal(t, registryimagev1alpha1.ImageUnknown, image.Replicas[2].State)
	assert.Contains(t, image.Replicas[2].Message, "connection refused")
	assert.EqualValues(t, 100, *image.Size)
	assert.EqualValues(t, 80, *image.ReclaimableSize)
	assert.Equal(t, "prometheus", image.Labels[registryimagev1alpha1.ModuleLabel])
	assert.Equal(t, "Platform", image.Labels[registryimagev1alpha1.OwnerKindLabel])
	assert.Equal(t, "true", image.Labels[registryimagev1alpha1.InSetLabel])

	assert.Equal(t, registryimagev1alpha1.ImageCorrupted, byName[registryimagev1alpha1.NameOf(broken)].State,
		"the worst state among the replicas")
	assert.Contains(t, byName[registryimagev1alpha1.NameOf(broken)].Replicas[0].Message, "sha256:ff")

	// Of the set and not named by a replica that answered: missing there.
	replicas := byName[registryimagev1alpha1.NameOf(declared)].Replicas
	require.Len(t, replicas, 3)
	assert.Equal(t, registryimagev1alpha1.ImageMissing, replicas[1].State)

	// Outside the set and not held: not on that replica at all.
	outside := byName[registryimagev1alpha1.NameOf(pulled)]
	assert.False(t, outside.InSet)
	assert.Equal(t, "false", outside.Labels[registryimagev1alpha1.InSetLabel])
	nodes := []string{}
	for _, replica := range outside.Replicas {
		nodes = append(nodes, replica.Node)
	}
	assert.Equal(t, []string{"master-1", "master-2"}, nodes)
}

func TestTheNameIsTheDigest(t *testing.T) {
	digest, ok := registryimagev1alpha1.DigestOf(registryimagev1alpha1.NameOf(shared))
	assert.True(t, ok)
	assert.Equal(t, shared, digest)
	_, ok = registryimagev1alpha1.DigestOf("prometheus")
	assert.False(t, ok)
}

func TestTheCollectorAsksEveryReplicaWithItsToken(t *testing.T) {
	var tokens []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens = append(tokens, r.Header.Get("Authorization"))
		require.Equal(t, registryimagev1alpha1.IndexPath, r.URL.Path)
		_ = json.NewEncoder(w).Encode(index("", registryimagev1alpha1.IndexedImage{Digest: shared, State: registryimagev1alpha1.ImagePresent}))
	}))
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	portNumber, err := strconv.Atoi(port)
	require.NoError(t, err)

	token := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(token, []byte("dh-test-secret-0123456789abcdef\n"), 0o600))

	storage := &registryv1alpha1.RegistryStorage{ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName}}
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{
		{Node: "master-0", Address: "127.0.0.1:4281"},
		{Node: "master-1"},
		{Node: "master-9", Address: "127.0.0.1:4281"},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(storage,
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "master-0", Labels: map[string]string{layout.ControlPlaneLabel: ""}}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "master-1", Labels: map[string]string{layout.ControlPlaneLabel: ""}}},
	).Build()

	collector := &Collector{Reader: reader, TokenFile: token, Port: portNumber, Transport: server.Client().Transport}
	answers, err := collector.Collect(context.Background())
	require.NoError(t, err)
	require.Len(t, answers, 2, "a master that left is not asked")

	assert.Equal(t, "master-0", answers[0].Node)
	require.NoError(t, answers[0].Err)
	assert.Equal(t, "master-0", answers[0].Index.Node)
	assert.Equal(t, []string{"Bearer dh-test-secret-0123456789abcdef"}, tokens)

	assert.Equal(t, "master-1", answers[1].Node)
	assert.ErrorContains(t, answers[1].Err, "has not reported its address")

	_, err = (&Collector{Reader: fake.NewClientBuilder().WithScheme(scheme).Build()}).Collect(context.Background())
	assert.ErrorIs(t, err, ErrNoCache)
}

