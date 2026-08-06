/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"fmt"
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
	// clusterConfigLabel marks the kinds that hold the configuration of the
	// cluster -- the objects a human writes. The backup of the cluster
	// configuration is built from it, which is why it is maintained.
	clusterConfigLabel = "backup.deckhouse.io/cluster-config"

	deckhouseHeritage = "deckhouse"

	// refreshTimeout bounds one pass over the CRDs and APIServices. A refresh
	// that hangs would hold the loop forever and the index would silently stop
	// following the cluster.
	refreshTimeout = 30 * time.Second
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
	// ClusterConfig is true for a kind that holds the configuration of the
	// cluster: the objects a human writes, and the ones a coverage review is
	// really about.
	ClusterConfig bool
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

// NewModuleIndex builds the index and fills it once, so the first report does
// not have to wait for the refresh loop.
func NewModuleIndex(ctx context.Context, client metadata.Interface) *ModuleIndex {
	index := &ModuleIndex{
		client:          client,
		refreshInterval: defaultRefreshInterval,
		origins:         make(map[string]ResourceOrigin),
		groupModules:    make(map[string]string),
	}

	index.refresh(ctx)

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

// StartRefreshLoop keeps the index current. Blocks until ctx is cancelled, and
// cancelling it also aborts a refresh already in flight.
func (i *ModuleIndex) StartRefreshLoop(ctx context.Context) {
	for {
		timer := time.NewTimer(i.refreshInterval)

		select {
		case <-timer.C:
			i.refresh(ctx)
		case <-ctx.Done():
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
//
// Both maps are swapped under one lock. Swapping them separately would leave a
// window where a resource is attributed by a CRD read now and by an APIService
// read five minutes ago -- rare, harmless, and impossible to reason about.
func (i *ModuleIndex) refresh(ctx context.Context) {
	if i.client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()

	crds, err := i.client.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("ModuleIndex: listing CRDs failed: %v, keeping the previous index", err)

		return
	}

	origins := make(map[string]ResourceOrigin, len(crds.Items))
	for _, item := range crds.Items {
		labels := item.GetLabels()
		origins[item.GetName()] = ResourceOrigin{
			ClusterConfig: labels[clusterConfigLabel] == "true",
			Custom:        labels[heritageLabel] != deckhouseHeritage,
			Module:        labels[moduleLabel],
		}
	}

	groupModules, err := i.aggregatedGroups(ctx)
	if err != nil {
		// The CRDs are still worth having: aggregated groups are a handful, and
		// losing their module names is not a reason to lose the rest.
		klog.Warningf("ModuleIndex: listing APIServices failed, aggregated APIs stay unattributed: %v", err)
	}

	i.mu.Lock()
	i.origins = origins
	if err == nil {
		i.groupModules = groupModules
	}
	i.mu.Unlock()

	klog.V(4).Infof("ModuleIndex: refreshed with %d CRDs and %d aggregated API groups", len(origins), len(groupModules))
}

// aggregatedGroups maps an API group to the module whose APIService serves it.
func (i *ModuleIndex) aggregatedGroups(ctx context.Context) (map[string]string, error) {
	list, err := i.client.Resource(apiServiceGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list apiservices: %w", err)
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

	return groupModules, nil
}
