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

package agent

import (
	"context"
	"slices"
	"sync/atomic"
	"time"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/decision"
)

// cpStale is the age of the last control plane read after which the check
// of the control plane does not work.
const cpStale = 10 * time.Second

// ready tells whether the agent pod is ready.
func ready(reason domain.Reason, cpWorking bool, peers []domain.Peer, members int) bool {
	if !cpWorking {
		return false
	}

	// Without a compatible peer (the first install, a new protocol) the agent
	// does not wait for a member.
	if members == 0 && slices.ContainsFunc(peers, func(p domain.Peer) bool { return p.Compatible }) {
		return false
	}

	switch reason {
	case domain.ReasonArmed, domain.ReasonGroupDisabled,
		domain.ReasonMaintenance, domain.ReasonNoEvacuationTarget, domain.ReasonKernelPanicMismatch,
		domain.ReasonWatchdogDeviceMissing:
		return true
	default:
		return false
	}
}

// cpView passes each control plane state to the decision engine and to the
// observers, and keeps the last state for the watchdog and the readiness.
type cpView struct {
	out       chan domain.CPState
	observers []func(domain.CPState)
	// gate changes the state for the decision engine only. It can be nil.
	gate func(domain.CPState) domain.CPState
	last atomic.Pointer[domain.CPState]
}

func newCPView(observers ...func(domain.CPState)) *cpView {
	return &cpView{out: make(chan domain.CPState), observers: observers}
}

func (v *cpView) Updates() <-chan domain.CPState {
	return v.out
}

// run passes the states of src until ctx ends.
func (v *cpView) run(ctx context.Context, src decision.ControlPlane) {
	for {
		var state domain.CPState

		select {
		case <-ctx.Done():
			return
		case state = <-src.Updates():
		}

		v.last.Store(&state)

		for _, observe := range v.observers {
			observe(state)
		}

		if v.gate != nil {
			state = v.gate(state)
		}

		select {
		case <-ctx.Done():
			return
		case v.out <- state:
		}
	}
}

// working tells whether the check of the control plane gives reads.
func (v *cpView) working() bool {
	last := v.last.Load()

	return last != nil && time.Since(last.ReadAt) < cpStale
}
