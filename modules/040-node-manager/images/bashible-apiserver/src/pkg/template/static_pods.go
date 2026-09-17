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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type staticPodRequest struct {
	Name              string
	Manifest          string
	CreationTimestamp time.Time
}

func (s *StepsStorage) AddStaticPodRequest(request *NodeStaticPodRequest) {
	klog.Infof("Adding NodeStaticPodRequest %s to context", request.Name)

	stored := staticPodRequest{
		Name:              request.Name,
		Manifest:          request.Spec.Manifest,
		CreationTimestamp: request.CreationTimestamp.Time,
	}

	s.m.Lock()
	defer s.m.Unlock()
	for _, ngPair := range generateNgPairs(request.Spec.NodeGroupSelector.MatchNames) {
		s.staticPodRequests[ngPair] = append(s.staticPodRequests[ngPair], &stored)
	}
}

func (s *StepsStorage) RemoveStaticPodRequest(request *NodeStaticPodRequest) {
	klog.Infof("Removing NodeStaticPodRequest %s from context", request.Name)

	s.m.Lock()
	defer s.m.Unlock()
	for _, ngPair := range generateNgPairs(request.Spec.NodeGroupSelector.MatchNames) {
		stored := s.staticPodRequests[ngPair]
		for i, candidate := range stored {
			if candidate.Name != request.Name {
				continue
			}
			s.staticPodRequests[ngPair] = append(stored[:i], stored[i+1:]...)
			break
		}
	}
}

// staticPodsFor returns this group's requests and the wildcard ones, sorted by
// object name. The pod contest is settled over all stored requests, not just
// this group's, so an object that lost a pod is refused by every group.
func (s *StepsStorage) staticPodsFor(ng string) []*staticPodRequest {
	keyNg := fmt.Sprintf("*:%s", ng)
	wildcard := "*:*"

	s.m.RLock()
	defer s.m.RUnlock()

	accepted := acceptedStaticPods(s.staticPodRequests)

	requests := make([]*staticPodRequest, 0, len(s.staticPodRequests[keyNg])+len(s.staticPodRequests[wildcard]))
	for _, request := range slices.Concat(s.staticPodRequests[keyNg], s.staticPodRequests[wildcard]) {
		if !accepted[request.Name] {
			continue
		}
		requests = append(requests, request)
	}

	slices.SortFunc(requests, func(a, b *staticPodRequest) int {
		return strings.Compare(a.Name, b.Name)
	})

	return requests
}

// acceptedStaticPods names the requests that claimed their pod: oldest first,
// ties by object name, one manifest per namespace/name. Mirrors rejectedNSPRs
// in node-controller (internal/controller/nodeconfig/staticpods.go).
func acceptedStaticPods(stored map[string][]*staticPodRequest) map[string]bool {
	ordered := make([]*staticPodRequest, 0, len(stored))
	seen := make(map[string]bool, len(stored))

	for _, requests := range stored {
		for _, request := range requests {
			if seen[request.Name] {
				continue
			}
			seen[request.Name] = true
			ordered = append(ordered, request)
		}
	}

	slices.SortFunc(ordered, func(a, b *staticPodRequest) int {
		if !a.CreationTimestamp.Equal(b.CreationTimestamp) {
			return a.CreationTimestamp.Compare(b.CreationTimestamp)
		}

		return strings.Compare(a.Name, b.Name)
	})

	accepted := make(map[string]bool, len(ordered))
	claimed := make(map[string]string, len(ordered))

	for _, request := range ordered {
		identity, err := podIdentity(request.Manifest)
		if err != nil {
			klog.Errorf("Skipping NodeStaticPodRequest %s: %s", request.Name, err)
			continue
		}

		if owner, taken := claimed[identity]; taken {
			klog.Errorf("Skipping NodeStaticPodRequest %s: pod %s is already asked for by %s, which is older", request.Name, identity, owner)
			continue
		}

		claimed[identity] = request.Name
		accepted[request.Name] = true
	}

	return accepted
}

// podIdentity reads the namespace/name the manifest claims. Only those two
// fields are decoded: the manifest is run by kubelet's Pod type, not the one
// vendored here, and node-controller's webhook refused anything that is not one.
func podIdentity(manifest string) (string, error) {
	var pod struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	}

	if err := yaml.Unmarshal([]byte(manifest), &pod); err != nil {
		return "", fmt.Errorf("parse manifest: %w", err)
	}

	if pod.Metadata.Name == "" {
		return "", errors.New("parse manifest: metadata.name is empty")
	}

	return pod.Metadata.Namespace + "/" + pod.Metadata.Name, nil
}

const (
	// The step runs after 069_start_kubelet — a manifest nobody picks up is not
	// worth writing — and before 080_setup_timer_to_run_bashible. 070 is taken
	// by 070_start_sysctl_tuner, 071 is the first free weight after it.
	staticPodsStepName = "071_static_pods"

	staticPodsManifestDir = "/etc/kubernetes/manifests"
	staticPodsStateFile   = "/var/lib/bashible/static-pods.json"
	staticPodsAnnotation  = "node.deckhouse.io/static-pods"
)

const staticPodsHeader = `# Generated by bashible-apiserver from NodeStaticPodRequest objects.

manifests_dir="` + staticPodsManifestDir + `"
state_file="` + staticPodsStateFile + `"
mkdir -p "$manifests_dir"
node_ip="$(bb-d8-node-ip)"
`

// renderStaticPodsStep assembles the step in Go, not a template: the manifest has
// to reach the node byte for byte. Prune and record run before the manifests, the
// order of reconcileStaticPods in nodelet internal/controllers/kubelet/staticpods.go.
func (s *StepsStorage) renderStaticPodsStep(ng string) (string, error) {
	requests := s.staticPodsFor(ng)

	names := make([]string, 0, len(requests))
	for _, request := range requests {
		names = append(names, request.Name)
	}

	encodedNames, err := json.Marshal(names)
	if err != nil {
		return "", fmt.Errorf("encode static pod names: %w", err)
	}

	blocks := make([]string, 0, len(requests)+3)
	blocks = append(blocks, staticPodsHeader, staticPodsPruningBlock(string(encodedNames)))
	for _, request := range requests {
		blocks = append(blocks, staticPodManifestBlock(request))
	}
	blocks = append(blocks, staticPodsReportBlock(names))

	return strings.Join(blocks, "\n"), nil
}

// staticPodManifestBlock hands the manifest to bash through a quoted heredoc, so
// nothing inside it is expanded, and lets bash alone substitute $MY_IP — before
// bb-sync-file, or the file would differ on every run.
func staticPodManifestBlock(request *staticPodRequest) string {
	delimiter := heredocDelimiter(request.Manifest)

	manifest := request.Manifest
	if !strings.HasSuffix(manifest, "\n") {
		manifest += "\n"
	}

	return `manifest="$(cat <<"` + delimiter + `"
` + manifest + delimiter + `
)"
printf '%s\n' "${manifest//\$MY_IP/${node_ip}}" | bb-sync-file "${manifests_dir}/` + request.Name + `.yaml" -
`
}

// heredocDelimiter picks a marker the manifest does not carry, so the heredoc
// ends where it is meant to.
func heredocDelimiter(manifest string) string {
	delimiter := "EOF"
	for i := 1; strings.Contains(manifest, delimiter); i++ {
		delimiter = fmt.Sprintf("EOF_STATIC_POD_%d", i)
	}

	return delimiter
}

// staticPodsPruningBlock removes only the manifests the step wrote before and no
// longer writes. Mirrors the host state handling of
// candi/bashible/common-steps/all/030_configure_containerd_registry.sh.tpl.
func staticPodsPruningBlock(encodedNames string) string {
	return `new_names='` + encodedNames + `'
old_names="[]"
if [[ -f "$state_file" ]]; then
  old_names="$(< "$state_file")"
fi

echo "$old_names" | jq -r --argjson new_names "$new_names" '
  .[] | select(. as $name | $new_names | index($name) | not)' | while IFS= read -r old_name; do
  rm -f "${manifests_dir}/${old_name}.yaml"
done

echo "$new_names" > "$state_file"
`
}

// staticPodsReportBlock puts the written names on the Node: node-controller
// counts a bashible node as applied by that annotation. The retry mirrors
// candi/bashible/common-steps/all/098_update_node_annotations.sh.tpl.
func staticPodsReportBlock(names []string) string {
	annotation := staticPodsAnnotation + "-"
	if len(names) > 0 {
		annotation = staticPodsAnnotation + "=" + strings.Join(names, ",")
	}

	return `attempt=0
until bb-curl-helper-patch-node-metadata "$(bb-d8-node-name)" "annotations" "` + annotation + `"; do
  attempt=$((attempt + 1))
  if [[ $attempt -ge 5 ]]; then
    bb-log-error "Failed to report static pods on node $(bb-d8-node-name) after 5 attempts"
    exit 1
  fi
  bb-log-error "Failed to report static pods on node $(bb-d8-node-name), retrying in 10 seconds"
  sleep 10
done
`
}

// staticPodRequestsSyncTimeout bounds the startup wait: with no CRD and no RBAC
// the cache never syncs, and an agent parked on one watch serves none of the
// bundles every node reads.
const staticPodRequestsSyncTimeout = 30 * time.Second

// subscribeOnStaticPodRequests mirrors the NodeGroupConfiguration informer,
// (*StepsStorage).subscribeOnCRD in steps_storage.go, on a second resource; the
// buffered emitter it starts serves this queue too.
func (s *StepsStorage) subscribeOnStaticPodRequests(ctx context.Context, factory dynamicinformer.DynamicSharedInformerFactory) {
	if factory == nil {
		return
	}

	go s.runStaticPodRequestsQueue(ctx)

	ginformer := factory.ForResource(schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "nodestaticpodrequests",
	})

	informer := ginformer.Informer()
	_ = informer.SetWatchErrorHandler(cache.DefaultWatchErrorHandler)

	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			request, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			s.staticPodRequestsQueue <- nodeConfigurationQueueAction{
				action:    "add",
				newObject: request,
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldRequest, oldOK := oldObj.(*unstructured.Unstructured)
			newRequest, newOK := newObj.(*unstructured.Unstructured)
			if !oldOK || !newOK {
				return
			}
			s.staticPodRequestsQueue <- nodeConfigurationQueueAction{
				action:    "update",
				newObject: newRequest,
				oldObject: oldRequest,
			}
		},
		DeleteFunc: func(obj interface{}) {
			request, ok := deletedStaticPodRequest(obj)
			if !ok {
				return
			}
			s.staticPodRequestsQueue <- nodeConfigurationQueueAction{
				action:    "delete",
				oldObject: request,
			}
		},
	})

	go informer.Run(ctx.Done())

	syncCtx, cancelSync := context.WithTimeout(ctx, staticPodRequestsSyncTimeout)
	defer cancelSync()

	// Errorf and bounded, not Fatalf and for ever: the CRD is shipped by the same
	// module, but neither dying nor waiting out the process serves a node.
	if !cache.WaitForCacheSync(syncCtx.Done(), informer.HasSynced) {
		klog.Errorf("unable to sync NodeStaticPodRequest informer: %v", syncCtx.Err())
	}
}

func (s *StepsStorage) runStaticPodRequestsQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case event := <-s.staticPodRequestsQueue:
			if s.applyStaticPodRequestEvent(event) {
				s.emitter.emitChanges()
			}
		}
	}
}

// applyStaticPodRequestEvent reports whether anything changed, so a resync that
// repeats an object does not rebuild the context and bump every checksum.
func (s *StepsStorage) applyStaticPodRequestEvent(event nodeConfigurationQueueAction) bool {
	switch event.action {
	case "add":
		var request NodeStaticPodRequest
		if err := fromUnstructured(event.newObject, &request); err != nil {
			klog.Errorf("Convert NodeStaticPodRequest from unstructured failed: %s", err)
			return false
		}
		s.AddStaticPodRequest(&request)

	case "update":
		var newRequest NodeStaticPodRequest
		if err := fromUnstructured(event.newObject, &newRequest); err != nil {
			klog.Errorf("Convert NodeStaticPodRequest from unstructured failed: %s", err)
			return false
		}

		var oldRequest NodeStaticPodRequest
		if err := fromUnstructured(event.oldObject, &oldRequest); err != nil {
			klog.Errorf("Convert NodeStaticPodRequest from unstructured failed: %s", err)
			return false
		}

		if newRequest.Spec.IsEqual(oldRequest.Spec) {
			return false
		}

		s.RemoveStaticPodRequest(&oldRequest)
		s.AddStaticPodRequest(&newRequest)

	case "delete":
		var request NodeStaticPodRequest
		if err := fromUnstructured(event.oldObject, &request); err != nil {
			klog.Errorf("Convert NodeStaticPodRequest from unstructured failed: %s", err)
			return false
		}
		s.RemoveStaticPodRequest(&request)
	}

	return true
}

func deletedStaticPodRequest(obj interface{}) (*unstructured.Unstructured, bool) {
	if request, ok := obj.(*unstructured.Unstructured); ok {
		return request, true
	}

	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if !ok {
		return nil, false
	}

	request, ok := tombstone.Obj.(*unstructured.Unstructured)
	return request, ok
}
