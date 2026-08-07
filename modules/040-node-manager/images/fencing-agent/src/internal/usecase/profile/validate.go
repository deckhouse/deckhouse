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

package profile

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

// validator collects every violation so an invalid profile is reported whole,
// not one field per restart.
type validator struct {
	errs []error
}

func (v *validator) positive(field string, d metav1.Duration) {
	// The CRD pattern admits "0ms", and zero timings would disable probing.
	if d.Duration <= 0 {
		v.errs = append(v.errs, fmt.Errorf("%s: must be positive, got %s", field, d.Duration))
	}
}

func (v *validator) atLeastOne(field string, value int32) {
	if value < 1 {
		v.errs = append(v.errs, fmt.Errorf("%s: must be at least 1, got %d", field, value))
	}
}

// lessThan re-checks a CRD CEL invariant of the form "a must be less than b".
func (v *validator) lessThan(fieldA string, a metav1.Duration, fieldB string, b metav1.Duration) {
	if a.Duration >= b.Duration {
		v.errs = append(v.errs, fmt.Errorf("%s %s must be less than %s %s", fieldA, a.Duration, fieldB, b.Duration))
	}
}

// Validate re-checks the profile the agent is about to run on. The CRD schema
// and CEL rules guard admission only: an object may predate them, so every
// value is checked here and any error must stop the agent.
// evacuation.delay belongs to the controller and is not the agent's concern;
// fallback.ttl is checked because the agent's own timings must fit inside it.
func Validate(profile *v1alpha1.FencingSLAProfile) error {
	if profile == nil {
		return errors.New("profile is nil")
	}

	v := &validator{}
	spec := profile.Spec

	v.positive("memberlist.probeInterval", spec.Memberlist.ProbeInterval)
	v.positive("memberlist.probeTimeout", spec.Memberlist.ProbeTimeout)
	v.atLeastOne("memberlist.suspicionMult", spec.Memberlist.SuspicionMult)
	v.atLeastOne("memberlist.suspicionMaxTimeoutMult", spec.Memberlist.SuspicionMaxTimeoutMult)
	v.atLeastOne("memberlist.indirectChecks", spec.Memberlist.IndirectChecks)
	v.atLeastOne("memberlist.awarenessMaxMultiplier", spec.Memberlist.AwarenessMaxMultiplier)
	v.positive("memberlist.gossipInterval", spec.Memberlist.GossipInterval)
	v.atLeastOne("memberlist.retransmitMult", spec.Memberlist.RetransmitMult)
	v.positive("memberlist.gossipToTheDeadTime", spec.Memberlist.GossipToTheDeadTime)
	v.positive("fallback.heartbeat", spec.Fallback.Heartbeat)
	v.positive("fallback.ttl", spec.Fallback.TTL)
	v.positive("fallback.kubernetesAPITimeout", spec.Fallback.KubernetesAPITimeout)
	v.positive("rejoin.interval", spec.Rejoin.Interval)
	v.positive("rejoin.maxInterval", spec.Rejoin.MaxInterval)
	v.positive("watchdog.feedInterval", spec.Watchdog.FeedInterval)
	v.positive("watchdog.timeout", spec.Watchdog.Timeout)

	if len(v.errs) > 0 {
		return errors.Join(v.errs...)
	}

	// The five CRD CEL invariants, re-checked (see the function comment above).
	v.lessThan("memberlist.probeTimeout", spec.Memberlist.ProbeTimeout, "memberlist.probeInterval", spec.Memberlist.ProbeInterval)
	v.lessThan("fallback.heartbeat", spec.Fallback.Heartbeat, "fallback.ttl", spec.Fallback.TTL)
	v.lessThan("fallback.kubernetesAPITimeout", spec.Fallback.KubernetesAPITimeout, "fallback.ttl", spec.Fallback.TTL)

	// Rejoin allows equality: interval == maxInterval is a fixed retry pace.
	if spec.Rejoin.Interval.Duration > spec.Rejoin.MaxInterval.Duration {
		v.errs = append(v.errs, fmt.Errorf("rejoin.interval %s must not exceed rejoin.maxInterval %s",
			spec.Rejoin.Interval.Duration, spec.Rejoin.MaxInterval.Duration))
	}

	v.lessThan("watchdog.feedInterval", spec.Watchdog.FeedInterval, "watchdog.timeout", spec.Watchdog.Timeout)

	if len(v.errs) > 0 {
		return errors.Join(v.errs...)
	}

	return nil
}
