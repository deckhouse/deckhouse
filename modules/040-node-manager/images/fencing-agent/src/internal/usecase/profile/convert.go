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
	"time"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

// parser collects every violation so an invalid profile is reported whole,
// not one field per restart.
type parser struct {
	errs []error
}

func (p *parser) duration(field, value string) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s: %w", field, err))

		return 0
	}

	// The CRD pattern admits "0ms", and zero timings would disable probing.
	if d <= 0 {
		p.errs = append(p.errs, fmt.Errorf("%s: must be positive, got %q", field, value))

		return 0
	}

	return d
}

func (p *parser) count(field string, value int32) int {
	if value < 1 {
		p.errs = append(p.errs, fmt.Errorf("%s: must be at least 1, got %d", field, value))

		return 0
	}

	return int(value)
}

// lessThan re-checks a CRD CEL invariant of the form "a must be less than b".
func (p *parser) lessThan(fieldA string, a time.Duration, fieldB string, b time.Duration) {
	if a >= b {
		p.errs = append(p.errs, fmt.Errorf("%s %s must be less than %s %s", fieldA, a, fieldB, b))
	}
}

// Convert validates the profile and maps it to the runtime configuration.
// The CRD schema and CEL rules guard admission only: an object may predate
// them, so everything is re-checked here and any error must stop the agent.
// fallback.ttl and evacuation.delay belong to the controller: ttl joins the
// cross-field checks below, delay is not the agent's concern.
func Convert(profile *v1alpha1.FencingSLAProfile) (domain.SLA, error) {
	if profile == nil {
		return domain.SLA{}, errors.New("profile is nil")
	}

	p := &parser{}
	spec := profile.Spec

	sla := domain.SLA{
		Memberlist: domain.MemberlistTuning{
			ProbeInterval:           p.duration("memberlist.probeInterval", spec.Memberlist.ProbeInterval),
			ProbeTimeout:            p.duration("memberlist.probeTimeout", spec.Memberlist.ProbeTimeout),
			SuspicionMult:           p.count("memberlist.suspicionMult", spec.Memberlist.SuspicionMult),
			SuspicionMaxTimeoutMult: p.count("memberlist.suspicionMaxTimeoutMult", spec.Memberlist.SuspicionMaxTimeoutMult),
			IndirectChecks:          p.count("memberlist.indirectChecks", spec.Memberlist.IndirectChecks),
			AwarenessMaxMultiplier:  p.count("memberlist.awarenessMaxMultiplier", spec.Memberlist.AwarenessMaxMultiplier),
			GossipInterval:          p.duration("memberlist.gossipInterval", spec.Memberlist.GossipInterval),
			RetransmitMult:          p.count("memberlist.retransmitMult", spec.Memberlist.RetransmitMult),
			GossipToTheDeadTime:     p.duration("memberlist.gossipToTheDeadTime", spec.Memberlist.GossipToTheDeadTime),
		},
		Fallback: domain.FallbackTuning{
			Heartbeat:  p.duration("fallback.heartbeat", spec.Fallback.Heartbeat),
			APITimeout: p.duration("fallback.kubernetesAPITimeout", spec.Fallback.KubernetesAPITimeout),
		},
		Rejoin: domain.RejoinTuning{
			Interval:    p.duration("rejoin.interval", spec.Rejoin.Interval),
			MaxInterval: p.duration("rejoin.maxInterval", spec.Rejoin.MaxInterval),
		},
		Watchdog: domain.WatchdogTuning{
			FeedInterval: p.duration("watchdog.feedInterval", spec.Watchdog.FeedInterval),
			Timeout:      p.duration("watchdog.timeout", spec.Watchdog.Timeout),
		},
	}

	ttl := p.duration("fallback.ttl", spec.Fallback.TTL)

	if len(p.errs) > 0 {
		return domain.SLA{}, errors.Join(p.errs...)
	}

	// The five CRD CEL invariants, re-checked (see the package comment above).
	p.lessThan("memberlist.probeTimeout", sla.Memberlist.ProbeTimeout, "memberlist.probeInterval", sla.Memberlist.ProbeInterval)
	p.lessThan("fallback.heartbeat", sla.Fallback.Heartbeat, "fallback.ttl", ttl)
	p.lessThan("fallback.kubernetesAPITimeout", sla.Fallback.APITimeout, "fallback.ttl", ttl)

	// Rejoin allows equality: interval == maxInterval is a fixed retry pace.
	if sla.Rejoin.Interval > sla.Rejoin.MaxInterval {
		p.errs = append(p.errs, fmt.Errorf("rejoin.interval %s must not exceed rejoin.maxInterval %s",
			sla.Rejoin.Interval, sla.Rejoin.MaxInterval))
	}

	p.lessThan("watchdog.feedInterval", sla.Watchdog.FeedInterval, "watchdog.timeout", sla.Watchdog.Timeout)

	if len(p.errs) > 0 {
		return domain.SLA{}, errors.Join(p.errs...)
	}

	return sla, nil
}
