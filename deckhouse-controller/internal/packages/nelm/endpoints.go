// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nelm

import (
	"cmp"
	"encoding/json"
	"io"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
)

const (
	ingressKind        = "Ingress"
	httpRouteKind      = "HTTPRoute"
	listenerSetKind    = "ListenerSet"
	serviceAccountKind = "ServiceAccount"

	// gatewayGroup is the API group of the Gateway API, through which the alb
	// module publishes applications: HTTPRoutes attached to ListenerSets.
	gatewayGroup = "gateway.networking.k8s.io"

	// kubernetesAPIURL is the endpoint of a ServiceAccount: the in-cluster
	// address of the Kubernetes API, which its token is used against.
	kubernetesAPIURL = "https://kubernetes.default.svc/"

	// annotationEndpointDescription marks an Ingress, an HTTPRoute or a
	// ServiceAccount in the application chart as an application endpoint and
	// holds its description; the hosts and paths of that object, or the
	// Kubernetes API address for a ServiceAccount, are reflected in status.urls.
	annotationEndpointDescription = "packages.deckhouse.io/application-endpoint-description"
)

// endpointManifest is the part of a rendered document every endpoint kind
// shares; the spec is decoded once the kind is known.
type endpointManifest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec json.RawMessage `json:"spec"`
}

// ingressSpec is a minimal projection of an Ingress spec, just enough to build
// URLs from rules and TLS hosts.
type ingressSpec struct {
	TLS []struct {
		Hosts []string `json:"hosts"`
	} `json:"tls"`
	Rules []struct {
		Host string `json:"host"`
		HTTP struct {
			Paths []struct {
				Path string `json:"path"`
			} `json:"paths"`
		} `json:"http"`
	} `json:"rules"`
}

// httpRouteSpec is a minimal projection of an HTTPRoute spec: the listeners it
// attaches to, its hostnames and the paths it matches.
type httpRouteSpec struct {
	ParentRefs []parentRef `json:"parentRefs"`
	Hostnames  []string    `json:"hostnames"`
	Rules      []struct {
		Matches []struct {
			Path *struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"path"`
		} `json:"matches"`
	} `json:"rules"`
}

type parentRef struct {
	Group       string `json:"group"`
	Kind        string `json:"kind"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	SectionName string `json:"sectionName"`
	Port        int32  `json:"port"`
}

// listener is a ListenerSet listener, as far as the URLs of its routes need it.
type listener struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}

// listenerSet is a ListenerSet rendered by the same templates as the routes.
type listenerSet struct {
	name      string
	namespace string
	listeners []listener
}

// extractEndpointURLs collects application endpoints from a rendered
// multi-document YAML manifest. An endpoint is built for every host/path pair
// of networking.k8s.io Ingress and gateway.networking.k8s.io HTTPRoute
// resources annotated with packages.deckhouse.io/application-endpoint-description
// (any value except "false"), and for every annotated ServiceAccount, whose
// endpoint is the in-cluster address of the Kubernetes API. The annotation
// value becomes the endpoint description ("true" means no description). The
// scheme is https when the host is covered by spec.tls of the Ingress or by an
// HTTPS listener the HTTPRoute attaches to, http otherwise. The result is
// deduplicated and sorted; undecodable documents are skipped.
func extractEndpointURLs(renderedManifests string) []observed.URL {
	manifests := decodeEndpointManifests(renderedManifests)
	listenerSets := collectListenerSets(manifests)

	var urls []observed.URL
	for _, m := range manifests {
		description, ok := endpointDescription(m)
		if !ok {
			continue
		}

		switch {
		case m.Kind == ingressKind && strings.HasPrefix(m.APIVersion, "networking.k8s.io/"):
			urls = append(urls, ingressURLs(m, description)...)
		case m.Kind == httpRouteKind && strings.HasPrefix(m.APIVersion, gatewayGroup+"/"):
			urls = append(urls, httpRouteURLs(m, description, listenerSets)...)
		case m.Kind == serviceAccountKind && m.APIVersion == "v1":
			urls = append(urls, observed.URL{URL: kubernetesAPIURL, Description: description})
		}
	}

	slices.SortFunc(urls, func(a, b observed.URL) int {
		return cmp.Or(cmp.Compare(a.URL, b.URL), cmp.Compare(a.Description, b.Description))
	})

	return slices.Compact(urls)
}

// decodeEndpointManifests splits a rendered multi-document manifest into
// documents. An HTTPRoute can come before the ListenerSet it attaches to, so
// the whole manifest is decoded before any URL is built.
func decodeEndpointManifests(renderedManifests string) []endpointManifest {
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(renderedManifests), 4096)

	var manifests []endpointManifest
	for {
		m := new(endpointManifest)
		if err := dec.Decode(m); err != nil {
			if err == io.EOF {
				return manifests
			}
			// Skip empty or undecodable documents (e.g. standalone '---',
			// documents whose metadata does not match the projection).
			continue
		}
		manifests = append(manifests, *m)
	}
}

// endpointDescription reports whether a manifest is an application endpoint and
// returns its description: the annotation value, "true" meaning none.
func endpointDescription(m endpointManifest) (string, bool) {
	value, set := m.Metadata.Annotations[annotationEndpointDescription]
	if !set || value == "false" {
		return "", false
	}

	if value == "true" {
		return "", true
	}

	return value, true
}

// collectListenerSets reads the listeners of the rendered ListenerSets: the
// scheme of an HTTPRoute, and its hosts when it names none, come from them.
func collectListenerSets(manifests []endpointManifest) []listenerSet {
	var sets []listenerSet
	for _, m := range manifests {
		if m.Kind != listenerSetKind || !strings.HasPrefix(m.APIVersion, gatewayGroup+"/") {
			continue
		}

		var spec struct {
			Listeners []listener `json:"listeners"`
		}
		if err := json.Unmarshal(m.Spec, &spec); err != nil {
			continue
		}

		sets = append(sets, listenerSet{name: m.Metadata.Name, namespace: m.Metadata.Namespace, listeners: spec.Listeners})
	}

	return sets
}

// ingressURLs builds an endpoint for every host/path pair of the Ingress rules.
func ingressURLs(m endpointManifest, description string) []observed.URL {
	var spec ingressSpec
	if err := json.Unmarshal(m.Spec, &spec); err != nil {
		return nil
	}

	tlsHosts := make(map[string]struct{})
	for _, tls := range spec.TLS {
		for _, host := range tls.Hosts {
			tlsHosts[host] = struct{}{}
		}
	}

	var urls []observed.URL
	for _, rule := range spec.Rules {
		if rule.Host == "" {
			continue
		}

		scheme := "http"
		if _, ok := tlsHosts[rule.Host]; ok {
			scheme = "https"
		}

		paths := rule.HTTP.Paths
		if len(paths) == 0 {
			urls = append(urls, observed.URL{URL: scheme + "://" + rule.Host + "/", Description: description})
			continue
		}

		for _, p := range paths {
			urls = append(urls, observed.URL{URL: scheme + "://" + rule.Host + rootedPath(p.Path), Description: description})
		}
	}

	return urls
}

// httpRouteURLs builds an endpoint for every hostname/path pair of an HTTPRoute,
// the way the alb module publishes an application. A route without hostnames
// takes those of the listeners it attaches to. The scheme is https when one of
// those listeners is an HTTPS listener for the host. Like the TLS of an
// Ingress, it is read from the same templates: only a listener of a rendered
// ListenerSet counts.
func httpRouteURLs(m endpointManifest, description string, listenerSets []listenerSet) []observed.URL {
	var spec httpRouteSpec
	if err := json.Unmarshal(m.Spec, &spec); err != nil {
		return nil
	}

	listeners := attachedListeners(m.Metadata.Namespace, spec.ParentRefs, listenerSets)

	hosts := spec.Hostnames
	if len(hosts) == 0 {
		for _, l := range listeners {
			if l.Hostname != "" {
				hosts = append(hosts, l.Hostname)
			}
		}
	}

	paths := httpRoutePaths(spec)

	var urls []observed.URL
	for _, host := range hosts {
		scheme := "http"
		if slices.ContainsFunc(listeners, func(l listener) bool { return l.Protocol == "HTTPS" && listenerAccepts(l, host) }) {
			scheme = "https"
		}

		for _, path := range paths {
			urls = append(urls, observed.URL{URL: scheme + "://" + host + path, Description: description})
		}
	}

	return urls
}

// httpRoutePaths lists the paths of an HTTPRoute that make a URL: the value of
// every PathPrefix and Exact match. A route without rules, a rule without
// matches and a match without a path match every request, which is the root; a
// RegularExpression match names no single path and is skipped.
func httpRoutePaths(spec httpRouteSpec) []string {
	if len(spec.Rules) == 0 {
		return []string{"/"}
	}

	var paths []string
	for _, rule := range spec.Rules {
		if len(rule.Matches) == 0 {
			paths = append(paths, "/")
			continue
		}

		for _, match := range rule.Matches {
			if match.Path == nil {
				paths = append(paths, "/")
				continue
			}

			if match.Path.Type == "RegularExpression" {
				continue
			}

			paths = append(paths, rootedPath(match.Path.Value))
		}
	}

	return paths
}

// attachedListeners returns the listeners of the rendered ListenerSets an
// HTTPRoute attaches to: every listener of a ListenerSet its parentRefs name,
// narrowed down by sectionName and port. A Gateway parent, or a ListenerSet the
// templates do not render, adds none: its listeners are unknown here.
func attachedListeners(namespace string, refs []parentRef, listenerSets []listenerSet) []listener {
	var attached []listener
	for _, ref := range refs {
		if ref.Kind != listenerSetKind || (ref.Group != "" && ref.Group != gatewayGroup) {
			continue
		}

		// A parentRef without a namespace points to the route's own, and a rendered
		// object without one lands in the release namespace, so only two different
		// namespaces, both set, tell the objects apart.
		refNamespace := cmp.Or(ref.Namespace, namespace)
		for _, set := range listenerSets {
			if set.name != ref.Name || (refNamespace != "" && set.namespace != "" && set.namespace != refNamespace) {
				continue
			}

			for _, l := range set.listeners {
				if (ref.SectionName != "" && l.Name != ref.SectionName) || (ref.Port != 0 && l.Port != ref.Port) {
					continue
				}
				attached = append(attached, l)
			}
		}
	}

	return attached
}

// listenerAccepts reports whether a listener serves a host: a listener without a
// hostname serves every host, a wildcard one every host below its domain.
func listenerAccepts(l listener, host string) bool {
	if l.Hostname == "" || l.Hostname == host {
		return true
	}

	suffix, wildcard := strings.CutPrefix(l.Hostname, "*")
	return wildcard && strings.HasSuffix(host, suffix)
}

// rootedPath makes a path absolute: an empty path is the root.
func rootedPath(path string) string {
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}

	return path
}
