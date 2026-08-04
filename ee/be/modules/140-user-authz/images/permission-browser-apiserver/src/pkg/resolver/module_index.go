/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"
	"k8s.io/klog/v2"
)

const (
	// heritageLabel marks everything the platform installs.
	heritageLabel = "heritage"
	// moduleLabel names the module a CustomResourceDefinition comes from.
	moduleLabel = "module"

	deckhouseHeritage = "deckhouse"
)

var (
	crdGVR = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	apiServiceGVR = schema.GroupVersionResource{
		Group:    "apiregistration.k8s.io",
		Version:  "v1",
		Resource: "apiservices",
	}
)

// ResourceOrigin says where a resource came from.
//
// A coverage review asks two questions the API group cannot answer: which
// module is responsible for this resource, and is it ours at all. Guessing the
// module from the group name gets both wrong -- operator-trivy ships
// aquasecurity.github.io, and a customer CRD can live under any group.
type ResourceOrigin struct {
	// Module is the Deckhouse module that installs the resource, when the CRD
	// says so.
	Module string
	// Custom is true for a CRD the platform does not install: the resources of
	// the cluster owner, the ones a coverage review of "our own" looks at.
	Custom bool
}

// ModuleIndex maps a resource to the module that ships it, reading the labels
// of the CustomResourceDefinitions and of the APIServices.
//
// Two sources because there are two kinds of resource a module can bring: its
// own CRDs, and an aggregated API served by its own apiserver. Guessing the
// module from the API group covers neither honestly -- authorization.deckhouse.io
// is served by user-authz, and "authorization" is not a module at all.
//
// Only metadata is fetched: CRD schemas are the bulk of those objects, and
// nothing here reads them.
type ModuleIndex struct {
	client          metadata.Interface
	refreshInterval time.Duration

	mu sync.RWMutex
	// origins is keyed by CRD name: "<plural>.<group>".
	origins map[string]ResourceOrigin
	// groupModules is keyed by API group and covers the aggregated APIs, whose
	// resources have no CRD of their own.
	groupModules map[string]string
}

func NewModuleIndex(client metadata.Interface) *ModuleIndex {
	index := &ModuleIndex{
		client:          client,
		refreshInterval: defaultRefreshInterval,
		origins:         make(map[string]ResourceOrigin),
		groupModules:    make(map[string]string),
	}

	index.refresh()

	return index
}

// Origin reports what is known about the resource. The second value is false
// for built-in Kubernetes APIs: nothing in the cluster claims them, and calling
// them custom or filing them under a module would both be wrong.
func (i *ModuleIndex) Origin(group, resource string) (ResourceOrigin, bool) {
	// A subresource belongs to its parent: "pods/log" is served by whoever
	// ships "pods".
	if parent, _, ok := strings.Cut(resource, "/"); ok {
		resource = parent
	}

	name := resource
	if group != "" {
		name = resource + "." + group
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	if origin, known := i.origins[name]; known {
		return origin, true
	}

	// No CRD: an aggregated API is served by a module of its own, and the
	// APIService says which one.
	if module, known := i.groupModules[group]; known {
		return ResourceOrigin{Module: module}, true
	}

	return ResourceOrigin{}, false
}

func (i *ModuleIndex) HasData() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return len(i.origins) > 0
}

// StartRefreshLoop keeps the index current. Blocks until stopCh is closed.
func (i *ModuleIndex) StartRefreshLoop(stopCh <-chan struct{}) {
	for {
		timer := time.NewTimer(i.refreshInterval)

		select {
		case <-timer.C:
			i.refresh()
		case <-stopCh:
			timer.Stop()
			klog.Info("ModuleIndex: refresh loop stopped")

			return
		}

		timer.Stop()
	}
}

// refresh rebuilds the index. On error the previous one is kept: a stale module
// name is a cosmetic problem, an empty index turns every resource into "no
// module" and the coverage report loses its grouping.
func (i *ModuleIndex) refresh() {
	if i.client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	crds, err := i.client.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("ModuleIndex: listing CRDs failed: %v, keeping the previous index", err)

		return
	}

	origins := make(map[string]ResourceOrigin, len(crds.Items))
	for _, item := range crds.Items {
		origins[item.GetName()] = ResourceOrigin{
			Module: item.GetLabels()[moduleLabel],
			Custom: item.GetLabels()[heritageLabel] != deckhouseHeritage,
		}
	}

	i.mu.Lock()
	i.origins = origins
	i.mu.Unlock()

	klog.V(4).Infof("ModuleIndex: refreshed with %d CRDs", len(origins))

	i.refreshAPIServices(ctx)
}

// refreshAPIServices attributes the aggregated APIs. A failure here leaves the
// CRDs attributed: aggregated groups are a handful, and losing their module
// names is not a reason to lose the rest.
func (i *ModuleIndex) refreshAPIServices(ctx context.Context) {
	list, err := i.client.Resource(apiServiceGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("ModuleIndex: listing APIServices failed: %v, aggregated APIs stay unattributed: %v", err, err)

		return
	}

	groupModules := make(map[string]string)
	for _, item := range list.Items {
		module := item.GetLabels()[moduleLabel]
		if module == "" {
			continue
		}

		// An APIService is named "<version>.<group>"; the local ones covering
		// built-in APIs are named "v1." and carry no group.
		if _, group, ok := strings.Cut(item.GetName(), "."); ok && group != "" {
			groupModules[group] = module
		}
	}

	i.mu.Lock()
	i.groupModules = groupModules
	i.mu.Unlock()

	klog.V(4).Infof("ModuleIndex: refreshed with %d aggregated API groups", len(groupModules))
}
