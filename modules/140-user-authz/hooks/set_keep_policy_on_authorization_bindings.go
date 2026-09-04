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

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

/*
The bindings of ClusterAuthorizationRules and AuthorizationRules used to be rendered by the module
chart and are now owned by user-authz-controller. On the release that drops them from the chart the
release engine would prune every binding it rendered before — and the controller, deployed by that
same release, cannot be running yet to recreate them. Both engines honour the
`helm.sh/resource-policy: keep` annotation on the live object, so this hook stamps it on every
Helm-managed binding of the module right before the release runs, then verifies and refuses to let
the release proceed if any binding is left unprotected (the same pattern node-manager used when it
moved its machine objects into node-controller).

Once the controller adopts a binding it drops the Helm labels and the annotation, so the hook
becomes a no-op: the selector below only matches bindings still labeled as managed by Helm.

The hook uses its own client with a raised rate limit: the shared hook client is capped at the
client-go default of 5 QPS, which for tens of thousands of bindings would block the module
converge for an hour.
*/

const (
	helmResourcePolicyAnnotation = "helm.sh/resource-policy"
	helmResourcePolicyKeep       = "keep"

	// helmManagedBindingsSelector matches the bindings the chart rendered and the controller has not
	// adopted yet.
	helmManagedBindingsSelector = "heritage=deckhouse,module=user-authz,app.kubernetes.io/managed-by=Helm"

	// ruleBindingPrefix is the common prefix of every rule binding, user-authz:<rule>:<postfix>.
	ruleBindingPrefix = "user-authz:"

	keepPolicyQPS     = 100
	keepPolicyBurst   = 200
	keepPolicyWorkers = 16
)

var keepPolicyBindingResources = []schema.GroupVersionResource{
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        internal.Queue("keep-policy-authorization-bindings"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(setKeepPolicyOnAuthorizationBindings))

// newKeepPolicyClient builds a dynamic client with the raised rate limit; overridable in tests.
var newKeepPolicyClient = func(dc dependency.Container) (dynamic.Interface, error) {
	config, err := dc.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("get client config: %w", err)
	}

	config.QPS = keepPolicyQPS
	config.Burst = keepPolicyBurst

	return dynamic.NewForConfig(config)
}

func setKeepPolicyOnAuthorizationBindings(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	dynClient, err := newKeepPolicyClient(dc)
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}

	started := time.Now()

	stamped, err := stampKeepPolicy(ctx, dynClient, keepPolicyWorkers)
	if err != nil {
		return fmt.Errorf("stamp keep policy on authorization bindings: %w", err)
	}

	if err := verifyKeepPolicy(ctx, dynClient); err != nil {
		return fmt.Errorf("verify keep policy on authorization bindings: %w", err)
	}

	if stamped > 0 {
		input.Logger.Info("stamped keep policy on authorization bindings rendered by the chart",
			slog.Int("count", stamped), slog.Duration("took", time.Since(started)))
	}

	return nil
}

// stampKeepPolicy adds helm.sh/resource-policy: keep to every Helm-managed rule binding that does not
// have it yet and returns how many objects were patched.
func stampKeepPolicy(ctx context.Context, dynClient dynamic.Interface, workers int) (int, error) {
	patch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				helmResourcePolicyAnnotation: helmResourcePolicyKeep,
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("marshal patch: %w", err)
	}

	type target struct {
		gvr       schema.GroupVersionResource
		namespace string
		name      string
	}

	var targets []target
	for _, gvr := range keepPolicyBindingResources {
		list, err := dynClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: helmManagedBindingsSelector})
		if err != nil {
			return 0, fmt.Errorf("list %s: %w", gvr.Resource, err)
		}
		for _, item := range list.Items {
			if !isRuleBindingWithoutKeep(&item) {
				continue
			}
			targets = append(targets, target{gvr: gvr, namespace: item.GetNamespace(), name: item.GetName()})
		}
	}

	if len(targets) == 0 {
		return 0, nil
	}

	if workers <= 0 {
		workers = 1
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		queue    = make(chan target)
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range queue {
				_, err := dynClient.Resource(t.gvr).Namespace(t.namespace).Patch(ctx, t.name, types.MergePatchType, patch, metav1.PatchOptions{})
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("patch %s %s/%s: %w", t.gvr.Resource, t.namespace, t.name, err)
						cancel()
					}
					mu.Unlock()
				}
			}
		}()
	}

feed:
	for _, t := range targets {
		select {
		case queue <- t:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()

	if firstErr != nil {
		return 0, firstErr
	}

	return len(targets), nil
}

// verifyKeepPolicy re-lists the Helm-managed rule bindings and fails if any still lacks the keep
// annotation: letting the release run would prune it.
func verifyKeepPolicy(ctx context.Context, dynClient dynamic.Interface) error {
	for _, gvr := range keepPolicyBindingResources {
		list, err := dynClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: helmManagedBindingsSelector})
		if err != nil {
			return fmt.Errorf("list %s: %w", gvr.Resource, err)
		}
		for _, item := range list.Items {
			if isRuleBindingWithoutKeep(&item) {
				return fmt.Errorf("keep policy is not set on %s %s/%s: refusing to proceed to avoid prune", gvr.Resource, item.GetNamespace(), item.GetName())
			}
		}
	}

	return nil
}

func isRuleBindingWithoutKeep(obj *unstructured.Unstructured) bool {
	if !strings.HasPrefix(obj.GetName(), ruleBindingPrefix) {
		return false
	}

	return obj.GetAnnotations()[helmResourcePolicyAnnotation] != helmResourcePolicyKeep
}
