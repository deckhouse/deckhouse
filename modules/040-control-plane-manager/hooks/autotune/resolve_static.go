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

import (
	"context"
	"errors"
	"fmt"
)

// Terminates the chain. A master too small to spare the percent split leaves no
// budget, and the link fails rather than invent a number.
type staticResolver struct {
	nodes []Node
}

func (r *staticResolver) resolve(_ context.Context, _ resolveDeps, kind resourceKind) (resolvedRequests, error) {
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
	usableMilliCPU, usableBytes, ok := weakestMasterBudget(r.nodes)
	if !ok {
		return 0, degraded(degradedReasonBadNodes, errors.New("no master nodes to derive a budget from"))
	}

	var usable, reserved int64
	switch kind {
	case resourceCPU:
		usable, reserved = usableMilliCPU, everyNodeReservationMilliCPU
	case resourceMemory:
		usable, reserved = usableBytes, everyNodeReservationMemory
	}

	budget := percentOf(usable-reserved, controlPlanePercent)
	if budget <= 0 {
		return 0, degraded(degradedReasonNodesTooSmall, fmt.Errorf(
			"%s: master nodes leave nothing after the every-node reservation (usable=%d, reserved=%d)",
			kind, usable, reserved))
	}
	return budget, nil
}
