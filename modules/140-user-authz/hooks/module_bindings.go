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
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// Shared plumbing of the hooks that walk the rule bindings of the module (the ClusterRoleBindings
// and RoleBindings named user-authz:<rule>:<postfix>) with a dedicated, faster client.

const (
	// ruleBindingPrefix is the common prefix of every rule binding, user-authz:<rule>:<postfix>.
	ruleBindingPrefix = "user-authz:"

	// moduleBindingsSelector matches every binding of the module, whoever created it.
	moduleBindingsSelector = "heritage=deckhouse,module=user-authz"

	moduleBindingsClientQPS   = 100
	moduleBindingsClientBurst = 200
	moduleBindingsPageSize    = 500
)

var ruleBindingResources = []schema.GroupVersionResource{
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
}

// newModuleBindingsClient builds a dynamic client with the raised rate limit; overridable in tests.
var newModuleBindingsClient = func(dc dependency.Container) (dynamic.Interface, error) {
	config, err := dc.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("get client config: %w", err)
	}

	config.QPS = moduleBindingsClientQPS
	config.Burst = moduleBindingsClientBurst

	return dynamic.NewForConfig(config)
}

// bindingRef identifies one object of the walk (a binding, or a rule when statuses are rewritten).
type bindingRef struct {
	gvr        schema.GroupVersionResource
	namespace  string
	name       string
	generation int64
}

func isRuleBinding(obj *unstructured.Unstructured) bool {
	return strings.HasPrefix(obj.GetName(), ruleBindingPrefix)
}

// forEachRuleBinding pages through the bindings matching selector and calls fn for every one that
// passes filter. fn returning an error stops the iteration.
func forEachRuleBinding(ctx context.Context, dynClient dynamic.Interface, selector string, filter func(*unstructured.Unstructured) bool, fn func(bindingRef) error) error {
	for _, gvr := range ruleBindingResources {
		opts := metav1.ListOptions{LabelSelector: selector, Limit: moduleBindingsPageSize}
		for {
			list, err := dynClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, opts)
			if err != nil {
				return fmt.Errorf("list %s: %w", gvr.Resource, err)
			}
			for i := range list.Items {
				item := &list.Items[i]
				if !filter(item) {
					continue
				}
				if err := fn(bindingRef{gvr: gvr, namespace: item.GetNamespace(), name: item.GetName()}); err != nil {
					return err
				}
			}
			if list.GetContinue() == "" {
				break
			}
			opts.Continue = list.GetContinue()
		}
	}
	return nil
}

// forEachResourceParallel runs do on every object of gvr (all namespaces, paged) with the given
// number of workers and returns how many calls succeeded.
func forEachResourceParallel(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, workers int, do func(ctx context.Context, namespace, name string, generation int64) error) (int, error) {
	return runParallel(ctx, workers, func(ctx context.Context, emit func(bindingRef) error) error {
		opts := metav1.ListOptions{Limit: moduleBindingsPageSize}
		for {
			list, err := dynClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, opts)
			if err != nil {
				return fmt.Errorf("list %s: %w", gvr.Resource, err)
			}
			for i := range list.Items {
				item := &list.Items[i]
				if err := emit(bindingRef{gvr: gvr, namespace: item.GetNamespace(), name: item.GetName(), generation: item.GetGeneration()}); err != nil {
					return err
				}
			}
			if list.GetContinue() == "" {
				return nil
			}
			opts.Continue = list.GetContinue()
		}
	}, func(ctx context.Context, ref bindingRef) error {
		return do(ctx, ref.namespace, ref.name, ref.generation)
	})
}

// forEachRuleBindingParallel runs do on every binding matching selector and filter with the given
// number of workers and returns how many calls succeeded. The first failure cancels the rest;
// every failure is reported.
func forEachRuleBindingParallel(ctx context.Context, dynClient dynamic.Interface, selector string, filter func(*unstructured.Unstructured) bool, workers int, do func(context.Context, bindingRef) error) (int, error) {
	return runParallel(ctx, workers, func(ctx context.Context, emit func(bindingRef) error) error {
		return forEachRuleBinding(ctx, dynClient, selector, filter, emit)
	}, do)
}

// runParallel feeds the refs produced by produce to workers running do and returns how many calls
// succeeded. The first failure cancels the rest; every failure is reported.
func runParallel(ctx context.Context, workers int, produce func(ctx context.Context, emit func(bindingRef) error) error, do func(context.Context, bindingRef) error) (int, error) {
	if workers <= 0 {
		workers = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg    sync.WaitGroup
		done  atomic.Int64
		mu    sync.Mutex
		errs  []error
		queue = make(chan bindingRef, workers)
	)

	fail := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
		cancel()
	}

	for range workers {
		wg.Go(func() {
			for ref := range queue {
				if ctx.Err() != nil {
					// another worker failed: drain the queue without adding cancellation noise
					continue
				}
				if err := do(ctx, ref); err != nil {
					if errors.Is(err, context.Canceled) {
						continue
					}
					fail(err)
					continue
				}
				done.Add(1)
			}
		})
	}

	// The producer lists page by page and hands every target to the workers; a failure cancels the
	// context, which stops both the listing and the remaining work.
	listErr := produce(ctx, func(ref bindingRef) error {
		select {
		case queue <- ref:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	close(queue)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(errs) != 0 {
		// The worker errors are the cause; the listing error after them is just the cancellation.
		return int(done.Load()), errors.Join(errs...)
	}
	if listErr != nil {
		return int(done.Load()), listErr
	}

	return int(done.Load()), nil
}
