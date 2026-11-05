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

package scope

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Header names set by kube-aggregator on a proxied scoped read. They must stay
// in sync with k8s.io/apiserver/pkg/registry/generic/scopefilter (proxied.go)
// of the patched kube-apiserver.
const (
	HeaderMode       = "X-Deckhouse-Scope-Mode"
	HeaderNamespaces = "X-Deckhouse-Scope-Namespaces"
)

// Mode is how kube-apiserver let a scoped read through.
type Mode string

const (
	// ModeFiltered: RBAC denied the read in kube-apiserver; the namespaces are
	// the boundary of what may be served at all.
	ModeFiltered Mode = "filtered"
	// ModeNarrowed: RBAC allowed the read; the caller only narrows the view.
	// The extension authorizes the read as usual and then filters.
	ModeNarrowed Mode = "narrowed"
)

// Decision is the restriction of one request: only objects in Namespaces may
// be served. An empty Namespaces is the empty set and allows nothing.
type Decision struct {
	Mode       Mode
	Namespaces sets.Set[string]
}

// Allows reports whether an object in namespace may be served. Objects
// without a namespace (cluster-scoped) are never in the set.
func (d Decision) Allows(namespace string) bool {
	if namespace == "" {
		return false
	}
	return d.Namespaces.Has(namespace)
}

type decisionKey struct{}

// WithDecision returns a context carrying d. The trusted handlers are the only
// production callers; it is exported for tests of storage code.
func WithDecision(ctx context.Context, d Decision) context.Context {
	if d.Namespaces == nil {
		d.Namespaces = sets.New[string]()
	}
	return context.WithValue(ctx, decisionKey{}, d)
}

// FromContext returns the Decision of the request, if a trusted one was
// received. No decision means no restriction.
func FromContext(ctx context.Context) (Decision, bool) {
	d, ok := ctx.Value(decisionKey{}).(Decision)
	return d, ok
}

// isScopeHeader reports whether name is any spelling of one of the scope
// headers: ignoring case, with "_" read as "-". kube-aggregator strips the same
// set of spellings from client requests.
func isScopeHeader(name string) (string, bool) {
	n := strings.ReplaceAll(name, "_", "-")
	for _, k := range []string{HeaderMode, HeaderNamespaces} {
		if strings.EqualFold(n, k) {
			return k, true
		}
	}
	return "", false
}

func hasScopeHeaders(h http.Header) bool {
	for k := range h {
		if _, ok := isScopeHeader(k); ok {
			return true
		}
	}
	return false
}

// stripScopeHeaders deletes every spelling of the scope headers.
func stripScopeHeaders(h http.Header) {
	for k := range h {
		if _, ok := isScopeHeader(k); ok {
			delete(h, k)
		}
	}
}

// parseHeaders reads the Decision from the headers of a trusted request.
//
//   - No mode and no namespaces: no decision.
//   - Mode present, namespaces missing or empty: the empty set.
//   - Namespaces without a mode, more than one value of either header (under
//     any spelling), an unknown mode, an empty entry or a name that is not a
//     DNS label: an error; the request is refused, never served unfiltered.
func parseHeaders(h http.Header) (Decision, bool, error) {
	var modes, namespaces []string
	for k, vv := range h {
		switch canonical, _ := isScopeHeader(k); canonical {
		case HeaderMode:
			modes = append(modes, vv...)
		case HeaderNamespaces:
			namespaces = append(namespaces, vv...)
		}
	}
	if len(modes) == 0 {
		if len(namespaces) > 0 {
			return Decision{}, false, fmt.Errorf("%s without %s", HeaderNamespaces, HeaderMode)
		}
		return Decision{}, false, nil
	}
	if len(modes) > 1 {
		return Decision{}, false, fmt.Errorf("%d values of %s", len(modes), HeaderMode)
	}
	mode := Mode(strings.TrimSpace(modes[0]))
	switch mode {
	case ModeFiltered, ModeNarrowed:
	default:
		return Decision{}, false, fmt.Errorf("unknown %s %q", HeaderMode, modes[0])
	}
	if len(namespaces) > 1 {
		return Decision{}, false, fmt.Errorf("%d values of %s", len(namespaces), HeaderNamespaces)
	}
	set := sets.New[string]()
	if len(namespaces) == 1 && strings.TrimSpace(namespaces[0]) != "" {
		for _, raw := range strings.Split(namespaces[0], ",") {
			name := strings.TrimSpace(raw)
			if name == "" {
				return Decision{}, false, fmt.Errorf("empty entry in %s", HeaderNamespaces)
			}
			if msgs := validation.IsDNS1123Label(name); len(msgs) > 0 {
				return Decision{}, false, fmt.Errorf("invalid namespace name in %s: %s", HeaderNamespaces, strings.Join(msgs, "; "))
			}
			set.Insert(name)
		}
	}
	return Decision{Mode: mode, Namespaces: set}, true, nil
}
