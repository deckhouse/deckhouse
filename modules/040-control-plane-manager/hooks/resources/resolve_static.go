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

import (
	"context"
	"errors"
	"fmt"
)

// staticResolver terminates the chain with the legacy percent split: the weakest
// master's effective resources, minus the reservation the rest of the node needs
// anyway, carved down to controlPlanePercent.
//
// A master too small to give that up leaves no budget. Rather than invent a
// number, the link fails: the caller then leaves the measurement alone, and with
// no ConfigMap entry the templates apply their own fallback.
type staticResolver struct {
	nodes []Node
}

func (r *staticResolver) resolve(_ context.Context, _ resolveContext, kind resourceKind) (resolvedRequests, error) {
	budget, err := r.splitBudget(kind)
	if err != nil {
		return resolvedRequests{}, err
	}
	return resolvedRequests{
		byComponent: splitAcrossComponents(budget),
		source:      sourceStaticSplit,
	}, nil
}

func (r *staticResolver) splitBudget(kind resourceKind) (int64, error) {
	effectiveMilliCPU, effectiveBytes, ok := minMasterNodeBudget(r.nodes)
	if !ok {
		return 0, degraded(degradedReasonBadNodes, errors.New("no master nodes to derive a budget from"))
	}

	var effective, reserved int64
	switch kind {
	case resourceCPU:
		effective, reserved = effectiveMilliCPU, configEveryNodeMilliCPU
	case resourceMemory:
		effective, reserved = effectiveBytes, configEveryNodeMemory
	}

	budget := (effective - reserved) * controlPlanePercent / 100
	if budget <= 0 {
		return 0, degraded(degradedReasonNodesTooSmall, fmt.Errorf(
			"%s: master nodes leave nothing after the every-node reservation (effective=%d, reserved=%d)",
			kind, effective, reserved))
	}
	return budget, nil
}
