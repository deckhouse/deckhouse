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

package template

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const registryAgentManifest = `apiVersion: v1
kind: Pod
metadata:
  name: registry-agent
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
    - name: registry-agent
      image: deckhouse.local/images:registry-agent
      args:
        - --listen-address=$MY_IP:5001
`

const rivalAgentManifest = `apiVersion: v1
kind: Pod
metadata:
  name: registry-agent
  namespace: kube-system
spec:
  containers:
    - name: rival
      image: deckhouse.local/images:rival
`

const nodeLocalDNSManifest = `apiVersion: v1
kind: Pod
metadata:
  name: node-local-dns
  namespace: kube-system
spec:
  containers:
    - name: node-local-dns
      image: deckhouse.local/images:node-local-dns
`

func newStaticPodsStorage() *StepsStorage {
	return &StepsStorage{
		// "all:" is the cache key of the common steps for an empty provider: a
		// populated cache keeps Render off the filesystem.
		systemScripts:     map[string]map[string][]byte{"all:": {}},
		staticPodRequests: make(map[string][]*staticPodRequest),
	}
}

func staticPodRequestObject(name string, created time.Time, matchNames []string, manifest string) *NodeStaticPodRequest {
	return &NodeStaticPodRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: NodeStaticPodRequestSpec{
			NodeGroupSelector: NodeGroupSelector{MatchNames: matchNames},
			Manifest:          manifest,
		},
	}
}

func staticPodNames(requests []*staticPodRequest) []string {
	names := make([]string, 0, len(requests))
	for _, request := range requests {
		names = append(names, request.Name)
	}

	return names
}

func TestStaticPodsFor(t *testing.T) {
	t.Run("a group gets its own requests and the wildcard ones", func(t *testing.T) {
		storage := newStaticPodsStorage()
		storage.AddStaticPodRequest(staticPodRequestObject("registry-agent", time.Unix(100, 0), nil, registryAgentManifest))
		storage.AddStaticPodRequest(staticPodRequestObject("node-local-dns", time.Unix(200, 0), []string{"worker"}, nodeLocalDNSManifest))

		require.Equal(t, []string{"node-local-dns", "registry-agent"}, staticPodNames(storage.staticPodsFor("worker")))
		require.Equal(t, []string{"registry-agent"}, staticPodNames(storage.staticPodsFor("master")))
	})

	t.Run("one pod is claimed by the older object", func(t *testing.T) {
		storage := newStaticPodsStorage()
		storage.AddStaticPodRequest(staticPodRequestObject("registry-agent", time.Unix(100, 0), nil, registryAgentManifest))
		storage.AddStaticPodRequest(staticPodRequestObject("rival-agent", time.Unix(200, 0), nil, rivalAgentManifest))

		require.Equal(t, []string{"registry-agent"}, staticPodNames(storage.staticPodsFor("worker")))
	})

	t.Run("the loser of the contest is refused by every group", func(t *testing.T) {
		storage := newStaticPodsStorage()
		storage.AddStaticPodRequest(staticPodRequestObject("registry-agent", time.Unix(100, 0), []string{"worker"}, registryAgentManifest))
		storage.AddStaticPodRequest(staticPodRequestObject("rival-agent", time.Unix(200, 0), []string{"master"}, rivalAgentManifest))

		require.Equal(t, []string{"registry-agent"}, staticPodNames(storage.staticPodsFor("worker")))
		require.Empty(t, storage.staticPodsFor("master"))
	})

	t.Run("equal timestamps break the tie by object name", func(t *testing.T) {
		storage := newStaticPodsStorage()
		storage.AddStaticPodRequest(staticPodRequestObject("zeta", time.Unix(100, 0), nil, registryAgentManifest))
		storage.AddStaticPodRequest(staticPodRequestObject("alpha", time.Unix(100, 0), nil, rivalAgentManifest))

		require.Equal(t, []string{"alpha"}, staticPodNames(storage.staticPodsFor("worker")))
	})

	t.Run("a manifest with no readable identity is skipped", func(t *testing.T) {
		storage := newStaticPodsStorage()
		storage.AddStaticPodRequest(staticPodRequestObject("broken", time.Unix(100, 0), nil, "not a pod: ["))
		storage.AddStaticPodRequest(staticPodRequestObject("nameless", time.Unix(200, 0), nil, "apiVersion: v1\nkind: Pod\n"))

		require.Empty(t, storage.staticPodsFor("worker"))
	})

	t.Run("a removed object leaves every key it was stored under", func(t *testing.T) {
		storage := newStaticPodsStorage()
		request := staticPodRequestObject("node-local-dns", time.Unix(200, 0), []string{"worker", "master"}, nodeLocalDNSManifest)
		storage.AddStaticPodRequest(request)
		require.Len(t, storage.staticPodsFor("master"), 1)

		storage.RemoveStaticPodRequest(request)

		require.Empty(t, storage.staticPodsFor("worker"))
		require.Empty(t, storage.staticPodsFor("master"))
	})

	t.Run("an update moves the request to the groups its new selector names", func(t *testing.T) {
		storage := newStaticPodsStorage()
		old := staticPodRequestObject("node-local-dns", time.Unix(200, 0), []string{"worker"}, nodeLocalDNSManifest)
		storage.AddStaticPodRequest(old)

		updated := staticPodRequestObject("node-local-dns", time.Unix(200, 0), []string{"master"}, nodeLocalDNSManifest)
		require.False(t, updated.Spec.IsEqual(old.Spec))
		storage.RemoveStaticPodRequest(old)
		storage.AddStaticPodRequest(updated)

		require.Empty(t, storage.staticPodsFor("worker"))
		require.Equal(t, []string{"node-local-dns"}, staticPodNames(storage.staticPodsFor("master")))

		storage.RemoveStaticPodRequest(updated)
		require.Empty(t, storage.staticPodsFor("master"))
	})

	t.Run("a group nobody selected gets nothing", func(t *testing.T) {
		storage := newStaticPodsStorage()
		storage.AddStaticPodRequest(staticPodRequestObject("node-local-dns", time.Unix(200, 0), []string{"worker"}, nodeLocalDNSManifest))

		require.Empty(t, storage.staticPodsFor("nowhere"))
		require.Empty(t, newStaticPodsStorage().staticPodsFor("worker"))
	})
}
