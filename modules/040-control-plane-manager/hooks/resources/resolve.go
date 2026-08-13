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

package resources

// Resolution chain for per-component control-plane requests, highest priority
// first:
//
//	cpm ModuleConfig → global ModuleConfig (legacy location) → metrics API → static percent split
//
//	resolve_moduleconfig.go    resolve_dynamic.go    resolve_static.go
//
// Every link is asked per resourceKind, and a link with no answer for a kind
// delegates to the next one. That is what makes partial configuration work
// without any merging by the caller: resourcesRequests.cpu set and memory left
// out resolves cpu from the config and memory from the automatics, in the same
// hook run.
//
// The chain answers with a map; it persists nothing. Committing the answer into
// autotuneState — through autotuneState.commit, so the anti-flap LastChange
// timestamps survive — and writing the ConfigMap is the hook's job.

import (
	"context"
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// requestsByComponent is a resolved request per control-plane component:
// millicpu for resourceCPU, bytes for resourceMemory.
type requestsByComponent map[string]int64

// usageByComponent is measured usage from the metrics API, in the same units. A
// separate type from requestsByComponent on purpose: the two are trivially
// swappable at a call site — decide() takes both — and mean entirely different
// things.
type usageByComponent map[string]int64

// requestsSource names the link that produced an answer. It is part of the
// result because callers act on it: the global ModuleConfig location is
// deprecated, and answering from it has to raise an alert.
type requestsSource string

const (
	sourceCPMConfig    requestsSource = "cpmConfig"
	sourceGlobalConfig requestsSource = "globalConfig"
	sourceDynamic      requestsSource = "dynamic"
	sourceStaticSplit  requestsSource = "staticSplit"
)

type resolvedRequests struct {
	// byComponent holds every component in controlPlaneComponents, always with a
	// positive value: a zero would reach the ConfigMap, and from there the
	// static-pod manifests, as a literal request of 0.
	byComponent requestsByComponent
	source      requestsSource
	// deficit > 0 means a raise was measured but did not fit the headroom left on
	// the weakest master, by that much. Only the dynamic link sets it; it exists
	// to be reported as autotuneMetricName by the caller.
	deficit int64
}

// resolveContext carries the ambient plumbing a link needs to do I/O. It is the
// same for every link, straight from the hook invocation. Anything computed or
// cached (nodes, state, headroom) is one link's own opinion about how it
// resolves its answer, not something every link shares, so it lives on that
// link's struct instead.
type resolveContext struct {
	input *go_hook.HookInput
	dc    dependency.Container
}

type requestsResolver interface {
	// resolve answers for a single resourceKind, delegating to the next link when
	// it has no answer of its own.
	//
	// ctx is a parameter rather than a resolveContext field on purpose: a
	// context.Context is passed explicitly to the call that needs it, never held.
	//
	// An error means "this kind could not be resolved" and nothing more: the
	// caller must leave the measurement exactly as it is — no ConfigMap write, and
	// no falling through to a lower-priority link. Falling through would silently
	// replace a value the user set by hand with an automatic one, which is the
	// opposite of what a broken override should do. Every error carries its
	// degraded-metric reason, see degradedReasonOf.
	resolve(ctx context.Context, rctx resolveContext, kind resourceKind) (resolvedRequests, error)
}

// newRequestsResolverChain wires the chain. staticResolver terminates it, so no
// link is ever left calling a nil fallback.
//
// nodes and state are read from the snapshots once, by the hook, and injected:
// the hook needs the nodes anyway — no master nodes means a managed control
// plane, the one case where the ConfigMap legitimately does not exist — and a
// link loading them for itself would only duplicate that.
func newRequestsResolverChain(nodes []Node, state autotuneState) requestsResolver {
	static := &staticResolver{nodes: nodes}

	dynamic := &dynamicResolver{
		state:    state,
		nodes:    nodes,
		fallback: static,
	}

	globalConfig := &moduleConfigResolver{
		snapshotName: snapshotGlobalMC,
		source:       sourceGlobalConfig,
		state:        state,
		fallback:     dynamic,
	}

	return &moduleConfigResolver{
		snapshotName: snapshotCPMMC,
		source:       sourceCPMConfig,
		state:        state,
		fallback:     globalConfig,
	}
}

// splitAcrossComponents turns one budget into per-component requests by the
// fixed percent split (componentMeta[comp].percent, 45/35/10/10). Used by every
// link that resolves a single number instead of measuring components one by one.
func splitAcrossComponents(budget int64) requestsByComponent {
	requests := make(requestsByComponent, len(controlPlaneComponents))
	for _, comp := range controlPlaneComponents {
		requests[comp] = fallbackSplit(budget, componentMeta[comp].percent)
	}
	return requests
}

// degradedError couples a failure with the reason label of
// autotuneDegradedMetricName. It lets a link fail without also logging the
// failure and setting the metric itself — the caller does both, once, at the
// point where it decides what a failed kind means.
type degradedError struct {
	reason string
	err    error
}

func (e *degradedError) Error() string { return e.err.Error() }
func (e *degradedError) Unwrap() error { return e.err }

func degraded(reason string, err error) error {
	return &degradedError{reason: reason, err: err}
}

// degradedReasonOf returns the reason to report for err, falling back to
// degradedReasonReadThrough for an error that carries none.
func degradedReasonOf(err error) string {
	var degradedErr *degradedError
	if errors.As(err, &degradedErr) {
		return degradedErr.reason
	}
	return degradedReasonReadThrough
}
