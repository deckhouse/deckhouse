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
	"github.com/tidwall/gjson"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/lib/publicdomain"
	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/lib/sharedgateway"
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

When, is the other half. The record is applied only under Template mode, so it is taken when Template
mode first takes effect and not on first converge: a cluster installed on List and switched to
Template a year later gets a snapshot of the day it switched, not one of the day it was installed
that would grandfather none of what appeared in between. Which mode is in force is decided by
publicdomain.EffectiveMode, the same decision the template publishes in the ConfigMap's mode key.

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
// them keeps it. It has to name every kind templates/reserved-public-hosts.yaml writes a policy for:
// a kind the policies deny but this list does not read is denied on its next write with nothing in
// grandfatheredHosts to let it back out, which is the promise the grandfathering makes.
//
// The version is left to discovery rather than pinned. The policies match apiVersions ["*"], so a
// cluster whose Gateway API came from elsewhere and serves only v1beta1 would otherwise have nothing
// recorded while every write to those objects is still denied.
type hostBearingResource struct {
	gr    schema.GroupResource
	hosts func(object map[string]interface{}) []string
	// sharedGateway marks the one resource the Gateway policy exempts an object of by identity, so
	// that the exemption and the record cover the same object and no other.
	sharedGateway bool
}

var hostBearingResources = []hostBearingResource{
	{
		gr:    schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"},
		hosts: hostsFromIngressRules,
	},
	{
		gr:    schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "httproutes"},
		hosts: hostsFromRouteHostnames,
	},
	{
		gr:    schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "grpcroutes"},
		hosts: hostsFromRouteHostnames,
	},
	{
		gr:    schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "tlsroutes"},
		hosts: hostsFromRouteHostnames,
	},
	{
		gr:    schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "listenersets"},
		hosts: hostsFromListenerHostnames,
	},
	{
		gr:            schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "gateways"},
		hosts:         hostsFromListenerHostnames,
		sharedGateway: true,
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
		// Taken already. Put it back so that Helm renders what is in the cluster, and read nothing
		// else: this branch is what makes the grandfathering one-time rather than a list that grows
		// every time Deckhouse restarts.
		input.Values.Set(reservedPublicHostsValuePath, reservedPublicHostsSnapshot{
			Recorded: true,
			Hosts:    hostsFromRecord(recorded[0].Hosts, input),
		})
		return nil
	}

	domainTemplate := input.Values.Get("global.modules.publicDomainTemplate").String()
	configuredMode := input.Values.Get("deckhouse.reservedPublicHosts.mode").String()

	if publicdomain.EffectiveMode(configuredMode, domainTemplate) != publicdomain.ModeTemplate {
		// The record is applied only under Template mode, so recording now would take a snapshot of
		// a moment the reservation it feeds was not in force at -- and a cluster switched to
		// Template a year later would find a record from its List days and grandfather nothing that
		// appeared in between. Left unrecorded, so the snapshot is taken when Template first takes
		// effect. That also covers the cluster with no publicDomainTemplate, and the one whose
		// template the global schema would have rejected: neither reserves anything by pattern, so
		// neither has anything to grandfather or a reason to read the cluster.
		input.Values.Set(reservedPublicHostsValuePath, reservedPublicHostsSnapshot{Hosts: []string{}})
		return nil
	}

	namespace, err := publicdomain.ParseNamespace(domainTemplate)
	if err != nil {
		// Unreachable: EffectiveMode above answers List for every template this cannot parse.
		return fmt.Errorf("derive the reserved namespace: %w", err)
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

// hostsFromRecord spells the record the way the reservation compares hostnames and drops what is not
// a hostname at all.
//
// The header of this file documents editing grandfatheredHosts in the ConfigMap as the way to prune
// an entry, and this value goes straight into deckhouse.internal.reservedPublicHosts.hosts, which
// openapi/values.yaml validates. Without this an operator who types Shop.example.com, leaves a root
// dot or pastes a URL fails values validation and stops the module that renders Deckhouse from
// converging -- from a hand edit to a ConfigMap they were pointed at. The same reasoning the
// template applies to a misspelled excludedServices entry, and more strongly, because this is the
// surface the operator is invited to edit.
//
// A dropped entry is visible on the next converge: it disappears from grandfatheredHosts, and the
// object behind it is denied with a message naming the hostname.
func hostsFromRecord(recorded []string, input *go_hook.HookInput) []string {
	hosts := set.New()
	dropped := make([]string, 0)
	for _, host := range recorded {
		normalized := publicdomain.NormalizeHost(host)
		if !publicdomain.IsHost(normalized) {
			dropped = append(dropped, host)
			continue
		}
		hosts.Add(normalized)
	}

	if len(dropped) > 0 {
		input.Logger.Warn("dropped entries of grandfatheredHosts that are not hostnames",
			slog.Int("count", len(dropped)),
			slog.Any("dropped", dropped))
	}

	return hosts.Slice()
}

// collectTenantHosts reads every hostname claimed outside the namespaces the platform owns and keeps
// the ones the reservation is about to claim.
//
// The kinds are listed once here rather than watched, because a permanent informer over every Ingress
// in the cluster is a high price for something that runs on one converge, and because every kind but
// Ingress may have no CRD at all. Discovery is asked first for the same reason a template asks
// helm_lib_kind_exists: a group version the cluster does not serve has nothing to grandfather.
func collectTenantHosts(ctx context.Context, client k8s.Client, input *go_hook.HookInput, namespace publicdomain.Namespace) ([]string, error) {
	platformNamespaces, err := platformOwnedNamespaces(ctx, client, input)
	if err != nil {
		return nil, err
	}

	platformGateway := configuredSharedGateway(input.Values.Get)

	hosts := set.New()
	for _, resource := range hostBearingResources {
		gvr, served, err := servedVersion(client, resource.gr)
		if err != nil {
			return nil, err
		}
		if !served {
			input.Logger.Info("skipped a resource the cluster does not serve while recording the hostnames tenants already serve",
				slog.String("resource", resource.gr.String()))
			continue
		}

		err = eachObject(ctx, client, gvr, func(object *unstructured.Unstructured) {
			if exemptFromTheRecord(resource, platformNamespaces, platformGateway, object) {
				return
			}
			for _, host := range resource.hosts(object.Object) {
				host = publicdomain.NormalizeHost(host)
				// Only what the allowlist can lift is worth recording: the wildcard form of the
				// namespace is reserved by exact match and stays reserved whatever the allowlist
				// says, so recording it would put an entry in the record that the policies ignore.
				if namespace.PatternCovers(host) {
					hosts.Add(host)
				}
			}
		})
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", gvr.String(), err)
		}
	}

	return hosts.Slice(), nil
}

// exemptFromTheRecord reports whether an object is one the policies never deny, which is what makes
// it one the record has nothing to grandfather for.
//
// The shared Gateway is exempt on both sides for the same reason twice over: its hostnames need no
// exception, since the Gateway policy leaves that object alone whatever it claims; and a listener
// hostname on it that the pattern covers would otherwise be written into allowedHosts, where it
// would free a hostname the platform itself serves for every tenant and every other kind. That is
// the one mistake this hook must not make.
func exemptFromTheRecord(resource hostBearingResource, platformNamespaces set.Set, platformGateway sharedgateway.Ref, object *unstructured.Unstructured) bool {
	if platformNamespaces.Has(object.GetNamespace()) {
		return true
	}
	return resource.sharedGateway && platformGateway.Is(object.GetNamespace(), object.GetName())
}

// configuredSharedGateway is the Gateway the platform's own routes attach to, read the way
// helm_lib_module_gateway reads it, so that the object the hook skips is the object
// templates/reserved-public-hosts.yaml exempts. The module level is asked for as well although the
// deckhouse module has no gatewayAPIGateway setting of its own, because the precedence is the
// helper's and not this hook's to shorten.
func configuredSharedGateway(value func(path string) gjson.Result) sharedgateway.Ref {
	named := func(path string) *sharedgateway.Ref {
		gateway := value(path)
		if !gateway.Exists() {
			return nil
		}
		return &sharedgateway.Ref{
			Namespace: gateway.Get("namespace").String(),
			Name:      gateway.Get("name").String(),
		}
	}

	return sharedgateway.Resolve(
		named("deckhouse.gatewayAPIGateway"),
		named("global.modules.gatewayAPIGateway"),
		named("global.discovery.gatewayAPIDefaultGateway"),
	)
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

// servedVersion resolves the version the cluster serves a resource under, the way a template asks
// helm_lib_kind_exists over the capability strings: the group has to be there and the resource has
// to appear in one of its versions, but which version that is stays the cluster's business. The
// policies match apiVersions ["*"], so a version pinned here would leave a cluster serving only
// gateway.networking.k8s.io/v1beta1 with nothing recorded while every write is still denied.
//
// A group the cluster does not serve is absent and reported as such, which is how an uninstalled
// Gateway API is skipped. A discovery call that fails is returned as an error instead, because
// reading a failure as "no such CRD" would silently record nothing and grandfather nobody.
func servedVersion(client k8s.Client, gr schema.GroupResource) (schema.GroupVersionResource, bool, error) {
	groups, err := client.Discovery().ServerGroups()
	if err != nil {
		return schema.GroupVersionResource{}, false, fmt.Errorf("read the API groups the cluster serves: %w", err)
	}

	for _, group := range groups.Groups {
		if group.Name != gr.Group {
			continue
		}

		for _, groupVersion := range versionsPreferredFirst(group) {
			served, err := client.Discovery().ServerResourcesForGroupVersion(groupVersion)
			if apierrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				return schema.GroupVersionResource{}, false, fmt.Errorf("read the resources of %s: %w", groupVersion, err)
			}

			for _, resource := range served.APIResources {
				if resource.Name != gr.Resource {
					continue
				}
				parsed, err := schema.ParseGroupVersion(groupVersion)
				if err != nil {
					return schema.GroupVersionResource{}, false, fmt.Errorf("parse the group version %q: %w", groupVersion, err)
				}
				return parsed.WithResource(gr.Resource), true, nil
			}
		}
	}

	return schema.GroupVersionResource{}, false, nil
}

// versionsPreferredFirst orders the versions of a group the way a client should try them: whichever
// the API server prefers, then the rest, so that a resource served by several versions is read
// through the one the cluster itself would pick.
func versionsPreferredFirst(group metav1.APIGroup) []string {
	versions := make([]string, 0, len(group.Versions)+1)
	if group.PreferredVersion.GroupVersion != "" {
		versions = append(versions, group.PreferredVersion.GroupVersion)
	}
	for _, version := range group.Versions {
		if version.GroupVersion != group.PreferredVersion.GroupVersion {
			versions = append(versions, version.GroupVersion)
		}
	}
	return versions
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

func hostsFromIngressRules(object map[string]interface{}) []string {
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

// hostsFromRouteHostnames reads spec.hostnames, which HTTPRoute, GRPCRoute and TLSRoute all carry
// under that name and shape.
func hostsFromRouteHostnames(object map[string]interface{}) []string {
	hosts, _, err := unstructured.NestedStringSlice(object, "spec", "hostnames")
	if err != nil {
		return nil
	}
	return hosts
}

// hostsFromListenerHostnames reads spec.listeners[].hostname, which ListenerSet and Gateway both
// carry under that name and shape.
func hostsFromListenerHostnames(object map[string]interface{}) []string {
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
