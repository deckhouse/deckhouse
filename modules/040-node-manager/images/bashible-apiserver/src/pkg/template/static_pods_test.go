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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
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
		systemScripts:          map[string]map[string][]byte{"all:": {}},
		staticPodRequests:      make(map[string][]*staticPodRequest),
		staticPodRequestsQueue: make(chan nodeConfigurationQueueAction, 100),
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
		// Named to sort after the wildcard one: the order is by name, not by the
		// order the keys are read in.
		storage.AddStaticPodRequest(staticPodRequestObject("zz-node-local-dns", time.Unix(200, 0), []string{"worker"}, nodeLocalDNSManifest))

		require.Equal(t, []string{"registry-agent", "zz-node-local-dns"}, staticPodNames(storage.staticPodsFor("worker")))
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

func renderStaticPodsStepFor(t *testing.T, storage *StepsStorage, ng string) string {
	t.Helper()

	steps, err := storage.Render("all", "", map[string]interface{}{}, ng)
	require.NoError(t, err)

	step, exists := steps[staticPodsStepName]
	require.True(t, exists, "step %s must be rendered", staticPodsStepName)

	return step
}

func TestRenderStaticPodsStep(t *testing.T) {
	storage := newStaticPodsStorage()
	storage.AddStaticPodRequest(staticPodRequestObject("registry-agent", time.Unix(100, 0), nil, registryAgentManifest))
	storage.AddStaticPodRequest(staticPodRequestObject("node-local-dns", time.Unix(200, 0), []string{"worker"}, nodeLocalDNSManifest))

	worker := renderStaticPodsStepFor(t, storage, "worker")

	// The manifest reaches the node byte for byte, inside a quoted heredoc.
	require.Contains(t, worker, "<<\"EOF\"\n"+registryAgentManifest+"EOF\n)\"\n")
	require.Contains(t, worker, "<<\"EOF\"\n"+nodeLocalDNSManifest+"EOF\n)\"\n")

	// $MY_IP is substituted by bash on the node, never here.
	require.Contains(t, worker, "--listen-address=$MY_IP:5001")
	require.Contains(t, worker, `printf '%s\n' "${manifest//\$MY_IP/${node_ip}}" | bb-sync-file "${manifests_dir}/registry-agent.yaml" -`)
	require.Contains(t, worker, `printf '%s\n' "${manifest//\$MY_IP/${node_ip}}" | bb-sync-file "${manifests_dir}/node-local-dns.yaml" -`)
	require.Contains(t, worker, `node_ip="$(bb-d8-node-ip)"`)
	require.Contains(t, worker, `mkdir -p "$manifests_dir"`)

	require.Contains(t, worker, `new_names='["node-local-dns","registry-agent"]'`)
	require.Contains(t, worker, `"node.deckhouse.io/static-pods=node-local-dns,registry-agent"`)

	// The state file is recorded before the manifests are written: a recorded
	// name whose file is absent is written by the next run, while a written file
	// no run remembers is never removed. The annotation stays last.
	require.Less(t, strings.Index(worker, `rm -f "${manifests_dir}/${old_name}.yaml"`), strings.Index(worker, "bb-sync-file"))
	require.Less(t, strings.Index(worker, `echo "$new_names" > "$state_file"`), strings.Index(worker, "bb-sync-file"))
	require.Less(t, strings.LastIndex(worker, "bb-sync-file"), strings.Index(worker, "bb-curl-helper-patch-node-metadata"))

	master := renderStaticPodsStepFor(t, storage, "master")

	require.Contains(t, master, registryAgentManifest)
	require.NotContains(t, master, nodeLocalDNSManifest)
	require.Contains(t, master, `new_names='["registry-agent"]'`)
}

func TestRenderStaticPodsStepWithoutRequests(t *testing.T) {
	storage := newStaticPodsStorage()

	step := renderStaticPodsStepFor(t, storage, "worker")

	// The step is rendered with nothing to write too: it is what removes the
	// manifests of objects that are gone and takes the annotation off the Node.
	require.Contains(t, step, `new_names='[]'`)
	require.Contains(t, step, `rm -f "${manifests_dir}/${old_name}.yaml"`)
	require.Contains(t, step, `"node.deckhouse.io/static-pods-"`)
	require.NotContains(t, step, "bb-sync-file")
}

func TestRenderStaticPodsStepSkipsTheLoserOfACollision(t *testing.T) {
	storage := newStaticPodsStorage()
	storage.AddStaticPodRequest(staticPodRequestObject("registry-agent", time.Unix(100, 0), nil, registryAgentManifest))
	storage.AddStaticPodRequest(staticPodRequestObject("rival-agent", time.Unix(200, 0), nil, rivalAgentManifest))

	step := renderStaticPodsStepFor(t, storage, "worker")

	require.Contains(t, step, registryAgentManifest)
	require.NotContains(t, step, rivalAgentManifest)
	require.Contains(t, step, `new_names='["registry-agent"]'`)
	require.Contains(t, step, `"node.deckhouse.io/static-pods=registry-agent"`)
}

func TestRenderStaticPodsStepPicksAFreeHeredocDelimiter(t *testing.T) {
	const manifestWithEOF = `apiVersion: v1
kind: Pod
metadata:
  name: eof
  namespace: kube-system
spec:
  containers:
    - name: eof
      args:
        - EOF
`

	storage := newStaticPodsStorage()
	storage.AddStaticPodRequest(staticPodRequestObject("eof", time.Unix(100, 0), nil, manifestWithEOF))

	step := renderStaticPodsStepFor(t, storage, "eof-group")

	require.Contains(t, step, "<<\"EOF_STATIC_POD_1\"\n"+manifestWithEOF+"EOF_STATIC_POD_1\n)\"\n")
}

func TestRenderStaticPodsStepIsLiteralAndStable(t *testing.T) {
	const manifest = `apiVersion: v1
kind: Pod
metadata:
  name: nasty
  namespace: kube-system
spec:
  containers:
    - name: nasty
      args:
        - --listen=$MY_IP:5001
        - "backtick: ` + "`whoami`" + `"
        - "subshell: $(rm -rf /)"
        - 'single quoted'
        - "dollar brace: ${HOME} $* $@ $?"
        - "heredoc: EOF"
`

	storage := newStaticPodsStorage()
	// Two requests under two node-group keys, so the map walk in
	// acceptedStaticPods has an order to randomise between the two renders.
	storage.AddStaticPodRequest(staticPodRequestObject("nasty", time.Unix(100, 0), nil, manifest))
	storage.AddStaticPodRequest(staticPodRequestObject("alpha-node-local-dns", time.Unix(200, 0), []string{"worker"}, nodeLocalDNSManifest))

	step := renderStaticPodsStepFor(t, storage, "worker")

	// Nothing is escaped or expanded on the way to the node.
	require.Contains(t, step, manifest)
	// By object name, not by the order the keys are read in.
	require.Contains(t, step, `new_names='["alpha-node-local-dns","nasty"]'`)
	// The bundle checksum is computed over the rendered steps, so the same
	// requests must render the same bytes.
	require.Equal(t, step, renderStaticPodsStepFor(t, storage, "worker"))
}

func staticPodRequestUnstructured(name, manifest string, matchNames []string) *unstructured.Unstructured {
	spec := map[string]interface{}{"manifest": manifest}
	if len(matchNames) > 0 {
		names := make([]interface{}, 0, len(matchNames))
		for _, matchName := range matchNames {
			names = append(names, matchName)
		}
		spec["nodeGroupSelector"] = map[string]interface{}{"matchNames": names}
	}

	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "NodeStaticPodRequest",
		"metadata": map[string]interface{}{
			"name":              name,
			"creationTimestamp": "2026-01-01T00:00:00Z",
		},
		"spec": spec,
	}}
}

func TestSubscribeOnStaticPodRequests(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gvr := schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "nodestaticpodrequests",
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "NodeStaticPodRequestList"},
		staticPodRequestUnstructured("registry-agent", registryAgentManifest, nil),
	)

	factory := dynamicinformer.NewDynamicSharedInformerFactory(client, 0)

	storage := newStaticPodsStorage()
	storage.subscribeOnStaticPodRequests(ctx, factory)

	// require.Eventually runs the condition in its own goroutine, so it must not
	// call require itself.
	rendered := func(needle string, want bool) func() bool {
		return func() bool {
			steps, err := storage.Render("all", "", map[string]interface{}{}, "worker")
			if err != nil {
				return false
			}

			return strings.Contains(steps[staticPodsStepName], needle) == want
		}
	}

	require.Eventually(t, rendered(registryAgentManifest, true), time.Second, 10*time.Millisecond)

	_, err := client.Resource(gvr).Create(
		ctx,
		staticPodRequestUnstructured("node-local-dns", nodeLocalDNSManifest, []string{"worker"}),
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	require.Eventually(t, rendered(nodeLocalDNSManifest, true), time.Second, 10*time.Millisecond)

	require.NoError(t, client.Resource(gvr).Delete(ctx, "registry-agent", metav1.DeleteOptions{}))

	require.Eventually(t, rendered(registryAgentManifest, false), time.Second, 10*time.Millisecond)
}

func TestApplyStaticPodRequestEvent(t *testing.T) {
	storage := newStaticPodsStorage()
	object := staticPodRequestUnstructured("registry-agent", registryAgentManifest, nil)

	require.True(t, storage.applyStaticPodRequestEvent(nodeConfigurationQueueAction{
		action:    "add",
		newObject: object,
	}))

	// A resync repeats the object unchanged: nothing to re-render, so no event
	// and no new configuration checksum.
	require.False(t, storage.applyStaticPodRequestEvent(nodeConfigurationQueueAction{
		action:    "update",
		newObject: object,
		oldObject: object,
	}))

	changed := staticPodRequestUnstructured("registry-agent", nodeLocalDNSManifest, nil)
	require.True(t, storage.applyStaticPodRequestEvent(nodeConfigurationQueueAction{
		action:    "update",
		newObject: changed,
		oldObject: object,
	}))

	step := renderStaticPodsStepFor(t, storage, "worker")
	require.Contains(t, step, nodeLocalDNSManifest)
	require.NotContains(t, step, registryAgentManifest)

	require.True(t, storage.applyStaticPodRequestEvent(nodeConfigurationQueueAction{
		action:    "delete",
		oldObject: changed,
	}))
	require.Empty(t, storage.staticPodsFor("worker"))
}

func TestApplyStaticPodRequestEventMovesTheRequestBetweenGroups(t *testing.T) {
	storage := newStaticPodsStorage()
	object := staticPodRequestUnstructured("node-local-dns", nodeLocalDNSManifest, []string{"worker"})
	require.True(t, storage.applyStaticPodRequestEvent(nodeConfigurationQueueAction{
		action:    "add",
		newObject: object,
	}))

	moved := staticPodRequestUnstructured("node-local-dns", nodeLocalDNSManifest, []string{"master"})
	require.True(t, storage.applyStaticPodRequestEvent(nodeConfigurationQueueAction{
		action:    "update",
		newObject: moved,
		oldObject: object,
	}))

	require.Empty(t, storage.staticPodsFor("worker"))
	require.Equal(t, []string{"node-local-dns"}, staticPodNames(storage.staticPodsFor("master")))

	require.True(t, storage.applyStaticPodRequestEvent(nodeConfigurationQueueAction{
		action:    "delete",
		oldObject: moved,
	}))

	// The last request gone still renders the step: it is what removes the
	// manifest and takes the annotation off the Node.
	step := renderStaticPodsStepFor(t, storage, "master")
	require.Contains(t, step, `new_names='[]'`)
	require.Contains(t, step, `"node.deckhouse.io/static-pods-"`)
}

func TestDeletedStaticPodRequest(t *testing.T) {
	object := staticPodRequestUnstructured("registry-agent", registryAgentManifest, nil)

	request, ok := deletedStaticPodRequest(object)
	require.True(t, ok)
	require.Same(t, object, request)

	request, ok = deletedStaticPodRequest(cache.DeletedFinalStateUnknown{Key: "registry-agent", Obj: object})
	require.True(t, ok)
	require.Same(t, object, request)

	_, ok = deletedStaticPodRequest(cache.DeletedFinalStateUnknown{Key: "registry-agent", Obj: "not an object"})
	require.False(t, ok)
}
