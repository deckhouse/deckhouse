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

package nodeconfig

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/registry/apis/upstreams"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// Both kinds are read as unstructured, the way the registry module's RegistryConfig is: the
// types are not in this controller's scheme, and every manager of the tests would need them
// there for a typed watch to start.
var (
	registryUpstreamGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "RegistryUpstream"}
	moduleSourceGVK     = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleSource"}
)

// registryRoutes is the other registries every node routes on its own, arbitrated once
// per pass: the contest is cluster-wide, as the requests' contests are.
type registryRoutes struct {
	// Routes are the accepted entries of spec.registry.upstreams, ordered by match.
	Routes []internalv1alpha1.RegistryUpstream
	// Verdicts are the arbitration's answer for each RegistryUpstream, by name.
	Verdicts map[string]upstreams.Verdict
	// Accepted names the accepted RegistryUpstream objects in arbitration order.
	Accepted []string
	// Specs are the accepted objects' specs, by name, for what a status says beside the verdict.
	Specs map[string]registryv1alpha1.RegistryUpstreamSpec
	// Dropped says, by ModuleSource name, why its route is not on the nodes. There is no
	// status to write it to, so it is logged.
	Dropped map[string]string
	// Sources names, by lower-cased registry host, the ModuleSource whose authority bashible
	// writes into registry.d for that host, whether or not its route is in Routes.
	Sources map[string]string
}

// moduleSourceCA is the part of a ModuleSource a node needs: the certificate authority
// of the registry its modules are downloaded from.
type moduleSourceCA struct {
	Name    string
	Created time.Time
	Host    string
	Scheme  string
	CA      string
}

// readRegistryRoutes reads the objects the routes come from and arbitrates them against
// the registry the nodes pull from. A cluster without one of the kinds has none of
// those objects, which is an answer rather than a failure.
//
// An object that cannot be read is left out and logged, rather than failing the read:
// every render of every node reads the routes, and one malformed object written by
// anyone must not stop them all. Left out, it gets no verdict, so no status is written
// to it either.
func (s *sourceReader) readRegistryRoutes(ctx context.Context, primary *internalv1alpha1.Registry) (registryRoutes, error) {
	logger := log.FromContext(ctx)

	objects, err := listOptional(ctx, s.Reader, registryUpstreamGVK)
	if err != nil {
		return registryRoutes{}, fmt.Errorf("list RegistryUpstreams: %w", err)
	}
	registryUpstreams := make([]registryv1alpha1.RegistryUpstream, 0, len(objects))
	for i := range objects {
		upstream, err := toRegistryUpstream(&objects[i])
		if err != nil {
			routeNotices.report(logger, "RegistryUpstream/"+objects[i].GetName(), "the object is left out of the routes: "+err.Error())
			continue
		}
		routeNotices.clear("RegistryUpstream/" + objects[i].GetName())
		registryUpstreams = append(registryUpstreams, upstream)
	}

	objects, err = listOptional(ctx, s.Reader, moduleSourceGVK)
	if err != nil {
		return registryRoutes{}, fmt.Errorf("list ModuleSources: %w", err)
	}
	sources := make([]moduleSourceCA, 0, len(objects))
	for i := range objects {
		source, ok, err := readModuleSourceCA(&objects[i])
		if err != nil {
			routeNotices.report(logger, "ModuleSource/"+objects[i].GetName(), "the object is left out of the routes: "+err.Error())
			continue
		}
		if ok {
			sources = append(sources, source)
		}
	}

	primaryUpstream, err := s.readArbitrationPrimary(ctx, primary)
	if err != nil {
		return registryRoutes{}, err
	}
	routes := arbitrateRoutes(primaryUpstream, registryUpstreams, sources)
	for i := range sources {
		key := "ModuleSource/" + sources[i].Name
		if reason, dropped := routes.Dropped[sources[i].Name]; dropped {
			routeNotices.report(logger, key, "the route of the registry certificate authority is not on the nodes: "+reason)
			continue
		}
		routeNotices.clear(key)
	}
	return routes, nil
}

// readArbitrationPrimary is the registry the arbitration reserves: the one the nodes pull
// from, with the mirrors the registry module's configuration gives it. The mirrors make
// the verdicts the ones registry-controller reaches from the same configuration.
func (s *sourceReader) readArbitrationPrimary(ctx context.Context, primary *internalv1alpha1.Registry) (*registryv1alpha1.Upstream, error) {
	if primary == nil {
		return nil, nil
	}
	upstream := &registryv1alpha1.Upstream{Endpoint: registryv1alpha1.Endpoint{Host: primary.Address}}

	config := &unstructured.Unstructured{}
	config.SetAPIVersion(registryConfigAPIVersion)
	config.SetKind(registryConfigKind)
	err := s.Reader.Get(ctx, types.NamespacedName{Name: registryConfigName}, config)
	switch {
	case apierrors.IsNotFound(err), apierrors.IsForbidden(err), apimeta.IsNoMatchError(err), runtime.IsNotRegisteredError(err):
		return upstream, nil
	case err != nil:
		return nil, fmt.Errorf("read the registry configuration %s: %w", registryConfigName, err)
	}
	mirrors, _, _ := unstructured.NestedSlice(config.Object, "spec", "primary", "upstream", "mirrors")
	for _, mirror := range mirrors {
		if fields, ok := mirror.(map[string]any); ok {
			if host, _ := fields["host"].(string); host != "" {
				upstream.Mirrors = append(upstream.Mirrors, registryv1alpha1.Endpoint{Host: host})
			}
		}
	}
	return upstream, nil
}

// listOptional lists every object of a kind; no such kind in the cluster is no objects.
func listOptional(ctx context.Context, reader client.Reader, gvk schema.GroupVersionKind) ([]unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	err := reader.List(ctx, list)
	switch {
	case apimeta.IsNoMatchError(err), runtime.IsNotRegisteredError(err):
		return nil, nil
	case err != nil:
		return nil, err
	}
	return list.Items, nil
}

func toRegistryUpstream(obj *unstructured.Unstructured) (registryv1alpha1.RegistryUpstream, error) {
	var upstream registryv1alpha1.RegistryUpstream
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &upstream); err != nil {
		return upstream, fmt.Errorf("read RegistryUpstream %s: %w", obj.GetName(), err)
	}
	return upstream, nil
}

// readModuleSourceCA takes a ModuleSource's registry the way bashible does for the nodes it
// configures (bashible-apiserver, ModuleSource.getRegistry): the host is the first segment
// of spec.registry.repo, and a source with no certificate authority has nothing to route.
func readModuleSourceCA(obj *unstructured.Unstructured) (moduleSourceCA, bool, error) {
	fields := map[string]string{}
	for _, field := range []string{"repo", "ca", "scheme"} {
		value, _, err := unstructured.NestedString(obj.Object, "spec", "registry", field)
		if err != nil {
			return moduleSourceCA{}, false, fmt.Errorf("read ModuleSource %s: %w", obj.GetName(), err)
		}
		fields[field] = value
	}
	host, _, _ := strings.Cut(fields["repo"], "/")
	if host == "" || strings.TrimSpace(fields["ca"]) == "" {
		return moduleSourceCA{}, false, nil
	}
	return moduleSourceCA{
		Name:    obj.GetName(),
		Created: obj.GetCreationTimestamp().Time,
		Host:    strings.ToLower(host),
		Scheme:  strings.ToUpper(fields["scheme"]),
		CA:      fields["ca"],
	}, true, nil
}

// arbitrateRoutes settles the contest. The RegistryUpstream objects are arbitrated by the
// registry module's rules alone (upstreams.Arbitrate), so a verdict never depends on who
// arbitrates. The ModuleSource authorities come after them, oldest first: a source whose
// registry an accepted object routes gives way to it, a source naming the primary adds
// nothing, since the primary carries its own authority, a source whose registry or
// authority a node would refuse is dropped, and the sources share what is left of the
// budget the objects were held to.
func arbitrateRoutes(
	primary *registryv1alpha1.Upstream, registryUpstreams []registryv1alpha1.RegistryUpstream, sources []moduleSourceCA,
) registryRoutes {
	result := upstreams.Arbitrate(primary, registryUpstreams)
	routes := make([]internalv1alpha1.RegistryUpstream, 0, len(result.Accepted)+len(sources))
	specs := make(map[string]registryv1alpha1.RegistryUpstreamSpec, len(result.Accepted))
	routedBy := make(map[string]string, len(result.Accepted))

	byName := make(map[string]*registryv1alpha1.RegistryUpstream, len(registryUpstreams))
	for i := range registryUpstreams {
		byName[registryUpstreams[i].Name] = &registryUpstreams[i]
	}
	for _, name := range result.Accepted {
		upstream := byName[name]
		specs[name] = *upstream.Spec.DeepCopy()
		// Lower-cased: the match names the node's directory, and a host differing only
		// in case is the same registry.
		match := upstreams.NormalizeHost(upstream.Spec.Match)
		routedBy[match] = name
		routes = append(routes, internalv1alpha1.RegistryUpstream{
			RequestedBy: internalv1alpha1.ObjectOwner{Kind: internalv1alpha1.ObjectOwnerKindRegistryUpstream, Name: name},
			Match:       match,
			Upstream: internalv1alpha1.RegistryUpstreamEndpoints{
				RegistryEndpoint: nodeEndpoint(upstream.Spec.Upstream.Endpoint),
				Mirrors:          nodeEndpoints(upstream.Spec.Upstream.Mirrors),
			},
		})
	}

	slices.SortStableFunc(sources, func(a, b moduleSourceCA) int {
		return cmp.Or(a.Created.Compare(b.Created), strings.Compare(a.Name, b.Name))
	})
	reserved := upstreams.ReservedHosts(primary)
	taken := map[string]bool{}
	dropped := map[string]string{}
	byHost := map[string]string{}
	count, size := len(result.Accepted), result.SpecBytes
	for _, source := range sources {
		host := upstreams.NormalizeHost(source.Host)
		if _, known := byHost[host]; !known {
			byHost[host] = source.Name
		}
		if _, primaryHost := reserved[host]; primaryHost || taken[host] {
			continue
		}
		if owner, routed := routedBy[host]; routed {
			dropped[source.Name] = fmt.Sprintf("RegistryUpstream %s routes %s", owner, host)
			continue
		}
		endpoint := registryv1alpha1.Endpoint{Scheme: registryv1alpha1.Scheme(source.Scheme), Host: host, CA: source.CA}
		// Checked as an object's route is, since it goes into the same list: the NodeConfig
		// schema, and nodelet's loader after it, would refuse the document of every node.
		if err := validateSourceRoute(&endpoint); err != nil {
			dropped[source.Name] = err.Error()
			continue
		}
		cost := upstreams.SpecSize(&registryv1alpha1.RegistryUpstreamSpec{Match: host, Upstream: registryv1alpha1.Upstream{Endpoint: endpoint}})
		if count >= upstreams.MaxRoutes || size+cost > upstreams.MaxTotalSpecBytes {
			dropped[source.Name] = fmt.Sprintf("its %d bytes do not fit into the %d bytes and %d routes a node takes, "+
				"%d bytes and %d routes of which the routes before it take", cost, upstreams.MaxTotalSpecBytes, upstreams.MaxRoutes, size, count)
			continue
		}
		taken[host] = true
		count, size = count+1, size+cost
		routes = append(routes, internalv1alpha1.RegistryUpstream{
			RequestedBy: internalv1alpha1.ObjectOwner{Kind: internalv1alpha1.ObjectOwnerKindModuleSource, Name: source.Name},
			Match:       host,
			Upstream: internalv1alpha1.RegistryUpstreamEndpoints{RegistryEndpoint: internalv1alpha1.RegistryEndpoint{
				Scheme: source.Scheme,
				Host:   host,
				CA:     source.CA,
			}},
		})
	}

	// By match, so that the order of the listing never reorders a node's spec.
	slices.SortFunc(routes, func(a, b internalv1alpha1.RegistryUpstream) int {
		return strings.Compare(a.Match, b.Match)
	})
	return registryRoutes{
		Routes: routes, Verdicts: result.Verdicts, Accepted: result.Accepted, Specs: specs, Dropped: dropped, Sources: byHost,
	}
}

// validateSourceRoute checks a ModuleSource's registry the way the arbitration checks an
// object's: the ModuleSource schema leaves its repository and authority unchecked.
func validateSourceRoute(endpoint *registryv1alpha1.Endpoint) error {
	if err := upstreams.ValidateMatch(endpoint.Host); err != nil {
		return fmt.Errorf("its registry: %w", err)
	}
	if err := upstreams.ValidateEndpoint(endpoint); err != nil {
		return fmt.Errorf("its registry: %w", err)
	}
	return nil
}

func nodeEndpoints(endpoints []registryv1alpha1.Endpoint) []internalv1alpha1.RegistryEndpoint {
	if len(endpoints) == 0 {
		return nil
	}
	out := make([]internalv1alpha1.RegistryEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		out = append(out, nodeEndpoint(endpoint))
	}
	return out
}

// nodeEndpoint copies an endpoint field by field. auth.secretRef has no counterpart: the
// RegistryUpstream CRD does not serve it, and a node could not resolve it.
func nodeEndpoint(endpoint registryv1alpha1.Endpoint) internalv1alpha1.RegistryEndpoint {
	out := internalv1alpha1.RegistryEndpoint{
		Scheme: string(endpoint.Scheme),
		Host:   endpoint.Host,
		Path:   endpoint.Path,
		CA:     endpoint.CA,
	}
	if auth := endpoint.Auth; auth != nil && (auth.Username != "" || auth.Password != "" || auth.Auth != "") {
		out.Auth = &internalv1alpha1.RegistryEndpointAuth{Username: auth.Username, Password: auth.Password, Auth: auth.Auth}
	}
	return out
}

// nodeRegistry is the registry block of an Engine node: the primary, and the routes unless
// the registry agent owns the node's registry.d — the agent routes the same objects then,
// from the registry module's own arbitration.
func nodeRegistry(in clusterInputs, agentOwns bool) *internalv1alpha1.Registry {
	if in.Registry == nil {
		return nil
	}
	registry := *in.Registry
	registry.Upstreams = nil
	if agentOwns {
		return &registry
	}
	registry.Upstreams = slices.Clone(in.RegistryRoutes.Routes)
	return &registry
}

// mutableNodeRegistry is the registry block of a node bashible configures: the address of
// the primary, which nodelet keeps every route from taking over, and the routes.
//
// Nothing else of the primary: bashible writes it into registry.d, and nodelet downloads
// through the packages proxy (spec.registryPackagesProxy is always set on such a node), so
// the path, the authority and above all the credentials would only be copied into one more
// object every node can read.
//
// The routes are there whoever owns registry.d. Whether the registry agent does is a fact
// of the node — bashible puts the agent's _default into its state there — and nodelet,
// which sees it, leaves registry.d to the agent then. The module's intent read here would
// be a beat ahead of the node on the way in and a beat behind on the way out.
//
// No ModuleSource route: step 030 writes those authorities there itself, and nodelet
// refuses a directory bashible records as its own.
func mutableNodeRegistry(in clusterInputs) *internalv1alpha1.Registry {
	if in.Registry == nil {
		return nil
	}
	registry := &internalv1alpha1.Registry{Address: in.Registry.Address}
	for _, route := range in.RegistryRoutes.Routes {
		if route.RequestedBy.Kind == internalv1alpha1.ObjectOwnerKindModuleSource {
			continue
		}
		registry.Upstreams = append(registry.Upstreams, route)
	}
	return registry
}

// noticeLog logs what is wrong with an object once, and again only when it changes: the
// routes are read on every pass, and the same line on every pass would bury the change.
type noticeLog struct {
	mu   sync.Mutex
	last map[string]string
}

var routeNotices = &noticeLog{last: map[string]string{}}

func (n *noticeLog) report(logger logr.Logger, object, message string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.last[object] == message {
		return
	}
	n.last[object] = message
	logger.Info("registry route not rendered", "object", object, "reason", message)
}

func (n *noticeLog) clear(object string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.last, object)
}

// reconcileRegistryUpstreamStatuses tells each RegistryUpstream whether it took effect,
// while this controller is the one that arbitrates them. Under the registry agent the
// registry module's controller writes these statuses, and two writers would flip them.
//
// Read live, as the renders read: the manager's cache holds one Secret of d8-system,
// registry-bashible-config (common.CacheOptions), and deckhouse-registry is not it.
func (r *Reconciler) reconcileRegistryUpstreamStatuses(ctx context.Context, logger logr.Logger) error {
	sources := r.sources
	if sources == nil {
		sources = &sourceReader{Reader: r.Client}
	}
	agent, err := sources.readRegistryAgentMode(ctx)
	if err != nil {
		return err
	}
	if agent {
		return nil
	}
	primary, _, err := sources.readRegistry(ctx)
	if err != nil {
		return err
	}
	routes, err := sources.readRegistryRoutes(ctx, primary)
	if err != nil {
		return err
	}
	notes, err := sources.nodeNotes(ctx, primary, routes)
	if err != nil {
		return err
	}

	for name, verdict := range routes.Verdicts {
		if err := r.patchRegistryUpstreamVerdict(ctx, name, verdict, notes[name]); err != nil {
			logger.V(1).Info("cannot write the status of a RegistryUpstream", "registryUpstream", name, "error", err.Error())
			return fmt.Errorf("write the status of RegistryUpstream %s: %w", name, err)
		}
	}
	return nil
}

// nodeNotes says, by name, which accepted objects some bashible nodes do not apply, and why.
//
// A note, not a verdict: the object is applied everywhere else, and the verdict has to be
// the one registry-controller reaches, which knows nothing of the nodes.
func (s *sourceReader) nodeNotes(
	ctx context.Context, primary *internalv1alpha1.Registry, routes registryRoutes,
) (map[string]string, error) {
	if len(routes.Accepted) == 0 {
		return nil, nil
	}
	// The module's node configuration is what sets config_path for containerd v1 (step 032,
	// .registry.registryModuleEnable): it exists in Managed mode and in the configurable
	// modes of the previous implementation, and is withdrawn when the module is disabled or
	// in Unmanaged mode.
	moduleConfig, err := s.readConfigurationSecret(ctx, d8SystemNS, registryBashibleConfigSecret, registryBashibleConfigKey)
	if err != nil {
		return nil, err
	}
	containerdV1, containerdV2, err := s.bashibleContainerdGroups(ctx)
	if err != nil {
		return nil, err
	}

	notes := map[string]string{}
	add := func(name, note string) {
		notes[name] = strings.TrimSpace(notes[name] + " " + note)
	}
	readRegistryD := containerdV2
	if moduleConfig != nil {
		readRegistryD = slices.Sorted(slices.Values(append(slices.Clone(containerdV2), containerdV1...)))
	} else {
		for name, note := range containerdV1Notes(primary, routes, containerdV1) {
			add(name, note)
		}
	}
	for name, note := range moduleSourceNotes(routes, readRegistryD) {
		add(name, note)
	}
	return notes, nil
}

// containerdV1Notes covers the containerd v1 nodes while the module configures no node: such
// a node reads its mirrors inline from config.toml (step 032 sets no config_path), and
// bashible-apiserver renders there what fits (pkg/template/registry/upstreams.go,
// addUpstreams). Every other runtime reads registry.d, where nodelet routes them all.
//
// The rule is bashible-apiserver's, in its order: oldest first, an object with a path on
// any address is left out, since inline mirrors cannot rewrite a repository, and so is
// one naming a registry configured already, by itself or before it, since config.toml keys
// credentials and the authority by the registry and holds one table per registry.
func containerdV1Notes(primary *internalv1alpha1.Registry, routes registryRoutes, groups []string) map[string]string {
	if len(groups) == 0 {
		return nil
	}
	// The hosts of the cluster's own registry, and the registries configured for them.
	// Without the module both are the address of deckhouse-registry.
	hosts := map[string]bool{upstreams.NormalizeHost(upstreams.InClusterHost): true}
	configured := map[string]bool{}
	if primary != nil {
		hosts[upstreams.NormalizeHost(primary.Address)] = true
		configured[upstreams.NormalizeHost(primary.Address)] = true
	}
	notes := map[string]string{}
	for _, name := range routes.Accepted {
		spec := routes.Specs[name]
		endpoints := spec.Upstream.Endpoints()
		reason := ""
		match := upstreams.NormalizeHost(spec.Match)
		switch {
		case hosts[match]:
			reason = fmt.Sprintf("registry %q is configured there already", match)
		case slices.ContainsFunc(endpoints, func(e registryv1alpha1.Endpoint) bool { return strings.Trim(e.Path, "/") != "" }):
			reason = "containerd v1 reads its mirrors from config.toml there, which cannot rewrite a repository path"
		default:
			own := map[string]bool{}
			for _, endpoint := range endpoints {
				host := upstreams.NormalizeHost(endpoint.Host)
				if configured[host] || own[host] {
					reason = fmt.Sprintf("registry %q is configured there already, and containerd v1 holds one configuration per registry", host)
					break
				}
				own[host] = true
			}
		}
		if reason != "" {
			notes[name] = fmt.Sprintf("Not applied on the containerd v1 nodes of NodeGroup %s: %s.", strings.Join(groups, ", "), reason)
			continue
		}
		for _, endpoint := range endpoints {
			configured[upstreams.NormalizeHost(endpoint.Host)] = true
		}
	}
	return notes
}

// moduleSourceNotes covers the bashible nodes whose runtime reads registry.d: step 030
// writes there the directory of every ModuleSource registry with an authority, and nodelet
// leaves a directory bashible records alone, so an object routing such a registry is not
// applied there. An Engine node gets the object's route in place of the source's.
func moduleSourceNotes(routes registryRoutes, groups []string) map[string]string {
	if len(groups) == 0 || len(routes.Sources) == 0 {
		return nil
	}
	notes := map[string]string{}
	for _, name := range routes.Accepted {
		match := upstreams.NormalizeHost(routes.Specs[name].Match)
		if source, configured := routes.Sources[match]; configured {
			notes[name] = fmt.Sprintf("Not applied on the nodes of NodeGroup %s, which bashible configures: "+
				"ModuleSource %s configures registry %q there.", strings.Join(groups, ", "), source, match)
		}
	}
	return notes
}

// bashibleContainerdGroups names the groups bashible runs containerd v1 and containerd v2
// on: their own cri.type, or the cluster's defaultCRI for a group that sets none, as
// bashible reads it.
func (s *sourceReader) bashibleContainerdGroups(ctx context.Context) ([]string, []string, error) {
	groups := &v1.NodeGroupList{}
	if err := s.Reader.List(ctx, groups); err != nil {
		return nil, nil, fmt.Errorf("list NodeGroups: %w", err)
	}
	clusterConfig, err := s.readConfigurationSecret(ctx, kubeSystemNS, clusterConfigSecretName, clusterConfigKey)
	if err != nil {
		return nil, nil, err
	}
	defaultCRI, _ := clusterConfig["defaultCRI"].(string)
	if defaultCRI == "" {
		defaultCRI = string(v1.CRITypeContainerd)
		// CSE builds no containerd v1 package, so its implicit default cannot be v1.
		if nodecommon.IsCSEEdition() {
			defaultCRI = string(v1.CRITypeContainerdV2)
		}
	}

	var containerdV1, containerdV2 []string
	for i := range groups.Items {
		ng := &groups.Items[i]
		if !IsMutable(ng) {
			continue
		}
		cri := defaultCRI
		if ng.Spec.CRI != nil && ng.Spec.CRI.Type != "" {
			cri = string(ng.Spec.CRI.Type)
		}
		switch cri {
		case string(v1.CRITypeContainerd):
			containerdV1 = append(containerdV1, ng.Name)
		case string(v1.CRITypeContainerdV2):
			containerdV2 = append(containerdV2, ng.Name)
		}
	}
	slices.Sort(containerdV1)
	slices.Sort(containerdV2)
	return containerdV1, containerdV2, nil
}

// patchRegistryUpstreamVerdict writes one verdict the way the registry module's controller
// does (layout.patchUpstreamVerdicts), so the object reads the same whoever arbitrated it.
//
// note is added to the message of an accepted object: what some nodes do not apply.
func (r *Reconciler) patchRegistryUpstreamVerdict(ctx context.Context, name string, verdict upstreams.Verdict, note string) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(registryUpstreamGVK)
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	upstream, err := toRegistryUpstream(obj)
	if err != nil {
		return err
	}

	condition := metav1.Condition{
		Type:               registryv1alpha1.ConditionAccepted,
		ObservedGeneration: upstream.Generation,
		Reason:             verdict.Reason,
	}
	if verdict.Accepted {
		condition.Status = metav1.ConditionTrue
		condition.Message = "The upstream is routed on every node."
		if note != "" {
			condition.Message = "The upstream is routed. " + note
		}
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Message = verdict.Conflict
	}

	status := upstream.Status
	unchanged := status.Accepted == verdict.Accepted &&
		status.Conflict == verdict.Conflict &&
		status.ObservedGeneration == upstream.Generation
	status.Accepted = verdict.Accepted
	status.Conflict = verdict.Conflict
	status.ObservedGeneration = upstream.Generation
	changed := apimeta.SetStatusCondition(&status.Conditions, condition)
	if unchanged && !changed {
		return nil
	}

	// Spelled out rather than marshalled from the type: its fields are omitempty, and a merge
	// patch that leaves out `accepted: false` or an empty conflict leaves the old value there.
	var conflict any
	if status.Conflict != "" {
		conflict = status.Conflict
	}
	patch, err := json.Marshal(map[string]any{"status": map[string]any{
		"accepted":           status.Accepted,
		"conflict":           conflict,
		"observedGeneration": status.ObservedGeneration,
		"conditions":         status.Conditions,
	}})
	if err != nil {
		return err
	}
	return r.Client.Status().Patch(ctx, obj, client.RawPatch(types.MergePatchType, patch))
}
