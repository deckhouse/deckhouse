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

// Package proxy serves the container runtime's registry requests.
//
// The runtime sends every pull here, whatever registry it was for, and names the
// original registry in the `ns` query parameter. Deciding what to do with it is this
// package's job: use the in-cluster cache, fall back to the upstream, send it to an
// additional upstream with its credentials, or pass it through untouched.
//
// Being on the path of every pull on the node has a consequence worth stating: a
// registry nobody configured must still work. Images of workloads that have nothing
// to do with Deckhouse are pulled through here too, and refusing them would break the
// node rather than the feature.
package proxy

import (
	"fmt"
	"strings"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// Kind says which rule produced a decision, for logging and for metrics.
type Kind string

const (
	// KindPrimary is the image set of Deckhouse itself: the cache, with the upstream
	// behind it while the cache is still filling.
	KindPrimary Kind = "Primary"

	// KindRoute is an additional upstream declared by a module, an administrator or a
	// user.
	KindRoute Kind = "Route"

	// KindPassThrough is a registry nobody configured. Forwarded as it is, because the
	// agent sits on the path of every pull and blocking the unconfigured ones would
	// break workloads that never asked to be involved.
	KindPassThrough Kind = "PassThrough"
)

// Target is one attempt at satisfying a request.
type Target struct {
	// Name identifies the target in logs: a backend name, a route match, or the
	// registry itself when passing through.
	Name string

	// Scheme and Host are where to send the request.
	Scheme registryv1alpha1.Scheme
	Host   string

	// Path is the request path to use, already rewritten for this target.
	Path string

	// CA verifies the target, empty meaning the system trust store.
	CA string

	// Auth are the credentials for the target, if any.
	Auth *registryv1alpha1.Auth
}

// URL is the absolute request URL for this target.
func (t *Target) URL() string {
	scheme := t.Scheme
	if scheme == "" {
		scheme = registryv1alpha1.SchemeHTTPS
	}
	return fmt.Sprintf("%s://%s%s", scheme.Lower(), t.Host, t.Path)
}

// Decision is the ordered list of attempts for one request.
type Decision struct {
	Kind    Kind
	Targets []Target
}

// Resolve works out where a request should go.
//
// A pure function of the request and the layout, with no client and no network: the
// agent is on the path of every image pull on the node, so getting this wrong breaks
// the node, and it has to be exhaustively checkable without one.
//
// `self` is the agent's own endpoint. A request naming it would be the agent asking
// itself, and that has to be refused rather than looped.
func Resolve(
	namespace, requestPath string, spec *registryv1alpha1.RegistryNodeSpec, self string,
) (Decision, error) {
	if equalHost(namespace, self) {
		return Decision{}, fmt.Errorf("the request names the agent itself as its registry, which would loop")
	}
	if spec == nil {
		return Decision{}, fmt.Errorf("no node layout")
	}

	repository, remainder, err := splitAPIPath(requestPath)
	if err != nil {
		return Decision{}, err
	}

	// No namespace at all is a process on this node fetching through the agent, rather than
	// the container runtime pulling.
	//
	// The runtime always names the original registry, because the agent is never the registry
	// that was asked for — it is put in the runtime's path by a drop-in, so the registry it
	// meant to reach has to travel alongside the request. A process has nothing to be
	// redirected from: it dialled the agent deliberately, and what it means by that is the one
	// image set the agent exists to serve.
	//
	// The Deckhouse controller does exactly this. It fetches the release channel and its
	// module sources through the agent, which is what makes a change of registry reach it the
	// same way it reaches the runtime — the alternative being a second copy of the registry
	// address kept somewhere for it, and the reason that is not an alternative is that nothing
	// would keep the copy current.
	if namespace == "" || equalHost(namespace, constant.Host) {
		return primaryDecision(spec, repository, remainder)
	}

	for i := range spec.AdditionalRoutes {
		route := &spec.AdditionalRoutes[i]
		if equalHost(route.Match, namespace) {
			return routeDecision(route, repository, remainder), nil
		}
	}

	// Unconfigured, and therefore forwarded untouched. No credentials are added: the
	// agent has none for it, and inventing any would send this node's secrets to a
	// registry the cluster never vetted.
	return Decision{
		Kind: KindPassThrough,
		Targets: []Target{{
			Name:   namespace,
			Scheme: registryv1alpha1.SchemeHTTPS,
			Host:   namespace,
			Path:   requestPath,
		}},
	}, nil
}

// primaryDecision maps a request for the in-cluster address onto the backends, in
// the order the controller put them in: the cache first, the upstream only behind it.
func primaryDecision(
	spec *registryv1alpha1.RegistryNodeSpec, repository, remainder string,
) (Decision, error) {
	if len(spec.Backends) == 0 {
		return Decision{}, fmt.Errorf("no backend for the primary image set")
	}

	// The cluster refers to Deckhouse images under a fixed prefix. Each backend
	// serves them under its own, so the prefix is swapped per target — which is the
	// whole reason the upstream address can change without every image reference
	// changing with it.
	//
	// The prefix is the whole repository as often as it is a parent of one, and both
	// have to be swapped. Every image of an embedded module is `<base>@<digest>`, where
	// the base ends at this prefix — so the repository is exactly `system/deckhouse`,
	// with nothing under it. Trimming only `system/deckhouse/` left those untouched and
	// then put the backend's prefix in front of them, and the request went out naming
	// `system/deckhouse/system/deckhouse`, which no registry has ever heard of. It
	// affected every image the platform runs, and nothing noticed for as long as no pod
	// image named the in-cluster address at all.
	trimmed := trimPrefixPath(repository, strings.Trim(constant.Path, "/"))

	decision := Decision{Kind: KindPrimary}
	for i := range spec.Backends {
		backend := &spec.Backends[i]

		decision.Targets = append(decision.Targets, Target{
			Name:   string(backend.Name),
			Scheme: backend.Scheme,
			Host:   backend.Host,
			Path:   apiPath(joinRepository(backend.Path, trimmed), remainder),
			CA:     backend.CA,
			Auth:   backend.Auth,
		})
	}
	return decision, nil
}

// routeDecision maps a request for an intercepted host onto its upstream and mirrors.
func routeDecision(route *registryv1alpha1.Route, repository, remainder string) Decision {
	decision := Decision{Kind: KindRoute}

	for _, source := range append([]registryv1alpha1.Endpoint{route.Endpoint}, route.Mirrors...) {
		decision.Targets = append(decision.Targets, Target{
			Name:   route.Match,
			Scheme: source.Scheme,
			Host:   source.Host,
			// The image is named after the intercepted host, so the repository has to be
			// prefixed with where it actually lives in the upstream.
			Path: apiPath(joinRepository(source.Path, repository), remainder),
			CA:   source.CA,
			Auth: source.Auth,
		})
	}
	return decision
}

// splitAPIPath separates the repository from the rest of a registry API path.
//
// The shape is "/v2/<repository>/<verb>/<reference>", where the repository itself may
// contain slashes, so it is found by locating the verb rather than by counting
// segments.
func splitAPIPath(requestPath string) (string, string, error) {
	rest, found := strings.CutPrefix(requestPath, "/v2/")
	if !found {
		return "", "", fmt.Errorf("%q is not a registry API path", requestPath)
	}

	for _, verb := range []string{"/manifests/", "/blobs/", "/tags/", "/referrers/"} {
		if index := strings.LastIndex(rest, verb); index > 0 {
			return rest[:index], rest[index:], nil
		}
	}

	return "", "", fmt.Errorf("%q names no repository operation", requestPath)
}

// trimPrefixPath removes a repository prefix, whether it is the whole repository or a
// parent of it.
//
// Written out rather than being a `TrimPrefix`, because the two cases it has to tell apart
// look alike: `system/deckhouse` is the prefix itself and becomes empty, while
// `system/deckhouse-extra` merely starts with the same letters and must be left alone.
func trimPrefixPath(repository, prefix string) string {
	if repository == prefix {
		return ""
	}
	if under, found := strings.CutPrefix(repository, prefix+"/"); found {
		return under
	}
	return repository
}

// apiPath rebuilds a registry API path.
func apiPath(repository, remainder string) string {
	return "/v2/" + repository + remainder
}

// joinRepository puts a repository under a prefix, tolerating either being empty or
// carrying stray separators, since both come from user-written configuration.
func joinRepository(prefix, repository string) string {
	prefix = strings.Trim(prefix, "/")
	repository = strings.Trim(repository, "/")

	switch {
	case prefix == "":
		return repository
	case repository == "":
		return prefix
	default:
		return prefix + "/" + repository
	}
}

// equalHost compares registry hosts the way DNS does, since a name that differs only
// in case is the same registry and must not miss its route.
func equalHost(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}
