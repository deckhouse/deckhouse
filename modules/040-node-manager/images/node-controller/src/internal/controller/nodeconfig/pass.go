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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// pass memoises what one reconcile pass reads: an all-nodes pass renders every
// node from the same cluster state, and every read goes straight to the API
// server because the manager's cache is too old for these decisions.
type pass struct {
	// inputs is what reading the cluster-wide render inputs produced, keyed by
	// Kubernetes version: the ~6 uncached reads readClusterInputs performs are
	// done once per distinct version rather than once per node.
	inputs map[string]clusterInputsResult
	// derivedVersion is the Kubernetes version derived from cluster state, read
	// once per pass: derived_status computes it from the cluster configuration
	// and the control plane alone, so every group gets the same answer.
	derivedVersion *versionResult
	// groups holds the NodeGroup of each node rendered, keyed by group name: one
	// Get per group per pass instead of one per node (a cached Get deep-copies
	// status.instanceClass with it). nil is a group that does not exist.
	groups map[string]*v1.NodeGroup
	// rollouts is each group's remaining rollout budget, keyed by group name:
	// one NodeConfig listing per group per pass instead of one per node.
	rollouts map[string]*rolloutBudget
}

// clusterInputsResult is one attempt at reading the cluster-wide inputs, kept
// whether it succeeded or not: these inputs fail identically for every node,
// so the failure is memoised as much as the answer.
type clusterInputsResult struct {
	inputs clusterInputs
	err    error
}

// versionResult is one attempt at deriving the cluster's Kubernetes version,
// kept whether it succeeded or not, for the same reason as the inputs above:
// the answer is a property of the cluster, not of the node being rendered.
type versionResult struct {
	version string
	err     error
}

func newPass() *pass {
	return &pass{
		inputs:   map[string]clusterInputsResult{},
		groups:   map[string]*v1.NodeGroup{},
		rollouts: map[string]*rolloutBudget{},
	}
}

// kubernetesVersion answers which version the group's nodes run: the cluster's
// derived answer, computed once per pass, or the group's own status when the
// derivation has none.
func (r *Reconciler) kubernetesVersion(ctx context.Context, ng *v1.NodeGroup, p *pass) (string, error) {
	if p.derivedVersion == nil {
		version, err := deriveKubernetesVersion(ctx, r.derived, ng)
		p.derivedVersion = &versionResult{version: version, err: err}
	}
	if p.derivedVersion.err != nil {
		return "", p.derivedVersion.err
	}
	return cmp.Or(p.derivedVersion.version, ng.Status.KubernetesVersion), nil
}

func (r *Reconciler) nodeGroup(ctx context.Context, name string, p *pass) (*v1.NodeGroup, error) {
	if ng, ok := p.groups[name]; ok {
		return ng, nil
	}
	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			p.groups[name] = nil
			return nil, nil
		}
		return nil, fmt.Errorf("get NodeGroup %s: %w", name, err)
	}
	p.groups[name] = ng
	return ng, nil
}

// clusterInputs returns the render inputs for the group's Kubernetes version,
// reading them once per version per pass and serving the rest from the pass.
func (r *Reconciler) clusterInputs(ctx context.Context, ng *v1.NodeGroup, p *pass) (clusterInputs, error) {
	version, err := r.kubernetesVersion(ctx, ng, p)
	if err != nil {
		return clusterInputs{}, err
	}
	if result, ok := p.inputs[version]; ok {
		return result.inputs, result.err
	}
	in, err := r.sources.readClusterInputs(ctx, version)
	p.inputs[version] = clusterInputsResult{inputs: in, err: err}
	return in, err
}
