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

package autotune

// Resolution chain, highest priority first. Every link is asked per resourceKind
// and delegates the kinds it has no answer for, which is what makes a config
// that sets only cpu leave memory to the automatics in the same run.
//
//	cpm ModuleConfig → global ModuleConfig (legacy) → metrics API → static split
//	resolve_moduleconfig.go            resolve_dynamic.go   resolve_static.go

import (
	"context"
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// Millicpu for resourceCPU, bytes for resourceMemory.
type requestsByComponent map[string]int64

type usageByComponent map[string]int64

type resolverName string

const (
	resolverCPMConfig    resolverName = "cpmConfig"
	resolverGlobalConfig resolverName = "globalConfig"
	resolverDynamic      resolverName = "dynamic"
	resolverStaticSplit  resolverName = "staticSplit"
)

type resolvedRequests struct {
	// Every component, always positive: a zero would reach the static-pod manifests
	// as a literal request of 0.
	byComponent requestsByComponent
	resolver    resolverName
	deficit     int64
}

type resolveDeps struct {
	input *go_hook.HookInput
	dc    dependency.Container
}

type requestsResolver interface {
	// An error means "this kind could not be resolved" and nothing else: the caller
	// leaves the measurement untouched, and no link further down gets to answer —
	// falling through would overwrite a hand-set value with an automatic one.
	// Errors carry their degraded-metric reason, see degradedReasonOf.
	resolve(ctx context.Context, deps resolveDeps, kind resourceKind) (resolvedRequests, error)
}

func newRequestsResolverChain(nodes []Node, state autotuneState) requestsResolver {
	static := &staticResolver{nodes: nodes}

	dynamic := &dynamicResolver{
		state:    state,
		nodes:    nodes,
		fallback: static,
	}

	globalConfig := &moduleConfigResolver{
		snapshotName: snapshotGlobalMC,
		name:         resolverGlobalConfig,
		state:        state,
		fallback:     dynamic,
	}

	return &moduleConfigResolver{
		snapshotName: snapshotCPMMC,
		name:         resolverCPMConfig,
		state:        state,
		fallback:     globalConfig,
	}
}

func splitAcrossComponents(budget int64) requestsByComponent {
	requests := make(requestsByComponent, len(controlPlaneComponents))
	for _, comp := range controlPlaneComponents {
		requests[comp] = percentOf(budget, componentMeta[comp].percent)
	}
	return requests
}

type degradedError struct {
	reason string
	err    error
}

func (e *degradedError) Error() string { return e.err.Error() }
func (e *degradedError) Unwrap() error { return e.err }

func degraded(reason string, err error) error {
	return &degradedError{reason: reason, err: err}
}

func degradedReasonOf(err error) string {
	var degradedErr *degradedError
	if errors.As(err, &degradedErr) {
		return degradedErr.reason
	}
	return degradedReasonReadThrough
}
