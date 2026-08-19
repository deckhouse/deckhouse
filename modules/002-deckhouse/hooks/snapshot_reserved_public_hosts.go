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
	"fmt"
	"log/slog"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/002-deckhouse/internal/publicdomain"
	"github.com/deckhouse/deckhouse/pkg/log"
)

/*
templates/reserved-public-hosts.yaml reserves every hostname
global.modules.publicDomainTemplate can render, so turning that on in a cluster whose tenants already
serve one of them would start denying their next write. This hook grandfathers what already exists,
once.

Once is the whole point. A snapshot taken again would make the allowlist widen itself: a tenant
claims a hostname, the next snapshot legitimises it, and the reservation is defeated by waiting. So
the hook does not decide by a timestamp or by "has the module been upgraded"; it decides by the
record it produced last time.

That record lives in the d8-reserved-public-hosts ConfigMap in d8-system, the same object the
policies read their parameters from, and Helm is its only writer. The hook reads the grandfather keys
back out of it and puts them in values, Helm renders them from values, and the two agree on a fixed
point instead of arguing over the object -- the flow this repository already uses for a hook that has
to produce something a template owns (go_lib/hooks/tls_certificate/internal_tls.go, where a
self-signed certificate is generated once and then read back out of the Secret Helm renders). Values
are recomputed on every restart and cannot hold the record on their own, which is exactly why it has
to come back from the cluster.

Two consequences worth knowing:

  - Deleting the ConfigMap makes the next converge snapshot again. It is a heritage: deckhouse object
    in d8-system, so that takes the same access as turning the reservation off outright, but it is
    not nothing. Any recorded-state guard can be erased by whoever may erase the record.
  - An operator who edits grandfatheredHosts in the ConfigMap keeps the edit: the hook reads that key
    back, and Helm renders what the hook put in values. That is how the list gets pruned once the
    workload behind an entry is gone.
*/

const (
	reservedPublicHostsBinding   = "reserved_public_hosts_params"
	reservedPublicHostsValuePath = "deckhouse.internal.reservedPublicHosts"

	reservedPublicHostsConfigMap = "d8-reserved-public-hosts"
	reservedPublicHostsNamespace = "d8-system"
	grandfatherRecordedKey       = "grandfatherRecorded"
	grandfatheredHostsKey        = "grandfatheredHosts"
	grandfatherRecordedTrueValue = "true"

	reservedPublicHostsQueue  = "/modules/deckhouse/reserved-public-hosts"
	platformNamespaceHeritage = "deckhouse"

	// The pages a cluster-wide read is taken in. Large enough that a cluster of any ordinary size is
	// one or two requests, small enough that no single response has to be held whole.
	reservedPublicHostsPageLimit = 500
)

var namespaceResource = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}

// hostBearingResources are the kinds that carry a hostname a tenant could claim, and where each of
// them keeps it. The two Gateway API ones may have no CRD in the cluster, which is why the group
// version is checked against discovery before anything is read from it.
var hostBearingResources = []struct {
	gvr   schema.GroupVersionResource
	hosts func(object map[string]interface{}) []string
}{
	{
		gvr:   schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		hosts: hostsFromIngress,
	},
	{
		gvr:   schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"},
		hosts: hostsFromHTTPRoute,
	},
	{
		gvr:   schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "listenersets"},
		hosts: hostsFromListenerSet,
	},
}

// reservedPublicHostsSnapshot is the record, in values and in the ConfigMap alike.
type reservedPublicHostsSnapshot struct {
	// Recorded says the snapshot has been taken, and is the only thing that stops it being taken
	// again. It is a field of its own because an empty Hosts is a legitimate outcome and must not
	// read as "not yet".
	Recorded bool     `json:"recorded"`
	Hosts    []string `json:"hosts"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        reservedPublicHostsQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       reservedPublicHostsBinding,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{reservedPublicHostsNamespace}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{reservedPublicHostsConfigMap}},
			FilterFunc:   filterReservedPublicHostsParams,
		},
	},
}, dependency.WithExternalDependencies(snapshotReservedPublicHosts))

func filterReservedPublicHostsParams(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	data, _, err := unstructured.NestedStringMap(obj.Object, "data")
	if err != nil {
		return nil, fmt.Errorf("read the data of ConfigMap %s: %w", obj.GetName(), err)
	}

	return reservedPublicHostsSnapshot{
		Recorded: data[grandfatherRecordedKey] == grandfatherRecordedTrueValue,
		Hosts:    set.New(strings.Fields(data[grandfatheredHostsKey])...).Slice(),
	}, nil
}

func snapshotReservedPublicHosts(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	recorded, err := sdkobjectpatch.UnmarshalToStruct[reservedPublicHostsSnapshot](input.Snapshots, reservedPublicHostsBinding)
	if err != nil {
		return fmt.Errorf("unmarshal the %s snapshot: %w", reservedPublicHostsBinding, err)
	}

	if len(recorded) > 0 && recorded[0].Recorded {
		// Taken already. Put it back verbatim so that Helm renders what is in the cluster, and read
		// nothing else: this branch is what makes the grandfathering one-time rather than a list
		// that grows every time Deckhouse restarts.
		input.Values.Set(reservedPublicHostsValuePath, recorded[0])
		return nil
	}

	namespace, err := publicdomain.ParseNamespace(input.Values.Get("global.modules.publicDomainTemplate").String())
	if err != nil {
		// No template, or one the global schema would have rejected. Either way nothing is reserved
		// by pattern, so there is nothing to grandfather and no reason to read the cluster. Left
		// unrecorded on purpose: the snapshot belongs to the moment the reservation starts, and a
		// template set later still gets one.
		input.Values.Set(reservedPublicHostsValuePath, reservedPublicHostsSnapshot{Hosts: []string{}})
		return nil
	}

	client, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get the Kubernetes client: %w", err)
	}

	hosts, err := collectTenantHosts(ctx, client, input, namespace)
	if err != nil {
		return err
	}

	input.Logger.Info("recorded the hostnames tenants already serve under the platform's domain",
		slog.Int("count", len(hosts)),
		slog.Any("hosts", hosts))

	input.Values.Set(reservedPublicHostsValuePath, reservedPublicHostsSnapshot{Recorded: true, Hosts: hosts})
	return nil
}

// collectTenantHosts reads every hostname claimed outside the namespaces the platform owns and keeps
// the ones the reservation is about to claim.
//
// The kinds are listed once here rather than watched, because a permanent informer over every Ingress
// in the cluster is a high price for something that runs on one converge, and because two of the
// three kinds may have no CRD at all. Discovery is asked first for the same reason a template asks
// helm_lib_kind_exists: a group version the cluster does not serve has nothing to grandfather.
func collectTenantHosts(ctx context.Context, client k8s.Client, input *go_hook.HookInput, namespace publicdomain.Namespace) ([]string, error) {
	platformNamespaces, err := platformOwnedNamespaces(ctx, client, input)
	if err != nil {
		return nil, err
	}

	hosts := set.New()
	for _, resource := range hostBearingResources {
		if !servesResource(client, resource.gvr, input) {
			continue
		}

		err := eachObject(ctx, client, resource.gvr, func(object *unstructured.Unstructured) {
			if platformNamespaces.Has(object.GetNamespace()) {
				return
			}
			for _, host := range resource.hosts(object.Object) {
				host = publicdomain.NormalizeHost(host)
				if namespace.Covers(host) {
					hosts.Add(host)
				}
			}
		})
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", resource.gvr.String(), err)
		}
	}

	return hosts.Slice(), nil
}

// platformOwnedNamespaces are the namespaces the policies never match, so a hostname served from one
// of them is the platform's own and must not end up in an allowlist.
//
// The label is compared here rather than passed as a selector so that the answer does not depend on
// the API server filtering for us: reading a namespace as a tenant one that is not would grandfather
// a platform hostname, which is the one mistake this must not make.
func platformOwnedNamespaces(ctx context.Context, client k8s.Client, input *go_hook.HookInput) (set.Set, error) {
	namespaces := set.New()
	err := eachObject(ctx, client, namespaceResource, func(object *unstructured.Unstructured) {
		if object.GetLabels()["heritage"] == platformNamespaceHeritage {
			namespaces.Add(object.GetName())
		}
	})
	if err != nil {
		return nil, fmt.Errorf("read the namespaces: %w", err)
	}

	input.Logger.Debug("read the namespaces the platform owns", slog.Int("count", namespaces.Size()))
	return namespaces, nil
}

// servesResource reports whether the cluster serves the group version at all, which is how an absent
// Gateway API is told apart from a call that failed.
func servesResource(client k8s.Client, gvr schema.GroupVersionResource, input *go_hook.HookInput) bool {
	groupVersion := gvr.GroupVersion().String()
	served, err := client.Discovery().ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		input.Logger.Info("skipped a kind the cluster does not serve while recording the hostnames tenants already serve",
			slog.String("resource", gvr.String()),
			log.Err(err))
		return false
	}
	for _, resource := range served.APIResources {
		if resource.Name == gvr.Resource {
			return true
		}
	}

	input.Logger.Info("skipped a kind the cluster does not serve while recording the hostnames tenants already serve",
		slog.String("resource", gvr.String()))
	return false
}

// eachObject walks a resource page by page, so that a cluster-wide read does not pull every object
// into memory at once.
func eachObject(ctx context.Context, client k8s.Client, gvr schema.GroupVersionResource, visit func(*unstructured.Unstructured)) error {
	continueToken := ""
	for {
		page, err := client.Dynamic().Resource(gvr).List(ctx, metav1.ListOptions{
			Limit:    reservedPublicHostsPageLimit,
			Continue: continueToken,
		})
		if err != nil {
			return err
		}

		for i := range page.Items {
			visit(&page.Items[i])
		}

		continueToken = page.GetContinue()
		if continueToken == "" {
			return nil
		}
	}
}

func hostsFromIngress(object map[string]interface{}) []string {
	rules, _, err := unstructured.NestedSlice(object, "spec", "rules")
	if err != nil {
		return nil
	}
	hosts := make([]string, 0, len(rules))
	for _, rule := range rules {
		fields, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		if host, ok := fields["host"].(string); ok {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func hostsFromHTTPRoute(object map[string]interface{}) []string {
	hosts, _, err := unstructured.NestedStringSlice(object, "spec", "hostnames")
	if err != nil {
		return nil
	}
	return hosts
}

func hostsFromListenerSet(object map[string]interface{}) []string {
	listeners, _, err := unstructured.NestedSlice(object, "spec", "listeners")
	if err != nil {
		return nil
	}
	hosts := make([]string, 0, len(listeners))
	for _, listener := range listeners {
		fields, ok := listener.(map[string]interface{})
		if !ok {
			continue
		}
		if host, ok := fields["hostname"].(string); ok {
			hosts = append(hosts, host)
		}
	}
	return hosts
}
