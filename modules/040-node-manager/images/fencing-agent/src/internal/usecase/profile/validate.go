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
)

// The probe values of kubernetes-api-proxy. They are the same on every node and
// are not profile fields. The own probe of the control plane uses them too.
const (
	proxyMaxInterval = 1200 * time.Millisecond // 1 s with 20 % jitter
	proxyTimeout     = 500 * time.Millisecond
	// MaxProbeAge is both the longest interval between two probes and the
	// largest age of a probe when no probe is lost. After a round that lost the
	// probes to all upstreams, the age can be up to 2.3 s for up to 0.6 s.
	MaxProbeAge = proxyMaxInterval + proxyTimeout
)

// witnessMargin is the slack for the delay of one message. The lower bound of
// controlPlaneLostAfter keeps it.
const witnessMargin = 100 * time.Millisecond

// Validate checks the profile that the agent is about to run on: the values and
// the inequalities between them. The CRD schema and CEL rules guard only
// admission, and an object may predate them. The agent applies no profile that
// fails here.
func Validate(p *v1alpha1.FencingSLAProfile) error {
	if p == nil {
		return errors.New("profile is nil")
	}

	spec := p.Spec
	ml := spec.Memberlist

	var errs []error

	for _, d := range []struct {
		field string
		value time.Duration
	}{
		{"controlPlaneLostAfter", spec.ControlPlaneLostAfter.Duration},
		{"memberlist.probeInterval", ml.ProbeInterval.Duration},
		{"memberlist.probeTimeout", ml.ProbeTimeout.Duration},
		{"memberlist.gossipInterval", ml.GossipInterval.Duration},
		{"memberlist.deadNodeReclaimTime", ml.DeadNodeReclaimTime.Duration},
		{"memberlist.gossipToTheDeadTime", ml.GossipToTheDeadTime.Duration},
		{"rejoin.interval", spec.Rejoin.Interval.Duration},
		{"eviction.writerWaveInterval", spec.Eviction.WriterWaveInterval.Duration},
		{"poll.witnessFresherBy", spec.Poll.WitnessFresherBy.Duration},
		{"watchdog.timeout", spec.Watchdog.Timeout.Duration},
		{"watchdog.feedInterval", spec.Watchdog.FeedInterval.Duration},
		{"heartbeat.interval", spec.Heartbeat.Interval.Duration},
		{"heartbeat.apiTimeout", spec.Heartbeat.APITimeout.Duration},
	} {
		// The CRD pattern admits "0ms", and a zero timing would disable a check.
		if d.value <= 0 {
			errs = append(errs, fmt.Errorf("%s: must be positive, got %s", d.field, d.value))
		}
	}

	for _, n := range []struct {
		field string
		value int32
	}{
		{"memberlist.suspicionMult", ml.SuspicionMult},
		{"memberlist.suspicionMaxTimeoutMult", ml.SuspicionMaxTimeoutMult},
		{"memberlist.indirectChecks", ml.IndirectChecks},
		{"memberlist.awarenessMaxMultiplier", ml.AwarenessMaxMultiplier},
		{"memberlist.gossipNodes", ml.GossipNodes},
		{"memberlist.retransmitMult", ml.RetransmitMult},
	} {
		if n.value < 1 {
			errs = append(errs, fmt.Errorf("%s: must be at least 1, got %d", n.field, n.value))
		}
	}

	// The inequalities mean nothing for a zero value.
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	var (
		controlPlaneLostAfter = spec.ControlPlaneLostAfter.Duration
		witnessFresherBy      = spec.Poll.WitnessFresherBy.Duration
		watchdogTimeout       = spec.Watchdog.Timeout.Duration
		feedInterval          = spec.Watchdog.FeedInterval.Duration
		heartbeatInterval     = spec.Heartbeat.Interval.Duration
		apiTimeout            = spec.Heartbeat.APITimeout.Duration
	)

	check := func(ok bool, format string, args ...any) {
		if !ok {
			errs = append(errs, fmt.Errorf(format, args...))
		}
	}

	// The proxy does not retry a failed probe.
	minLostAfterByProbe := proxyMaxInterval + proxyTimeout
	check(controlPlaneLostAfter >= minLostAfterByProbe,
		"controlPlaneLostAfter %s must be at least %s, the longest probe interval plus the probe timeout of kubernetes-api-proxy",
		controlPlaneLostAfter, minLostAfterByProbe)

	minLostAfterByWitness := witnessFresherBy + MaxProbeAge + witnessMargin
	check(controlPlaneLostAfter >= minLostAfterByWitness, "controlPlaneLostAfter %s must be at least poll.witnessFresherBy + %s = %s",
		controlPlaneLostAfter, MaxProbeAge+witnessMargin, minLostAfterByWitness)

	check(MaxProbeAge < witnessFresherBy && witnessFresherBy < controlPlaneLostAfter-MaxProbeAge,
		"poll.witnessFresherBy %s must be more than %s and less than controlPlaneLostAfter - %s = %s",
		witnessFresherBy, MaxProbeAge, MaxProbeAge, controlPlaneLostAfter-MaxProbeAge)

	check(4*feedInterval <= watchdogTimeout, "watchdog.feedInterval %s must be at most a quarter of watchdog.timeout %s", feedInterval, watchdogTimeout)
	check(watchdogTimeout%time.Second == 0, "watchdog.timeout must be a whole number of seconds, got %s", watchdogTimeout)
	check(ml.SuspicionMult >= 3, "memberlist.suspicionMult must be at least 3, got %d", ml.SuspicionMult)
	check(2*ml.ProbeTimeout.Duration <= ml.ProbeInterval.Duration, "memberlist.probeTimeout %s must be at most half of memberlist.probeInterval %s",
		ml.ProbeTimeout.Duration, ml.ProbeInterval.Duration)
	check(heartbeatInterval+apiTimeout <= controlPlaneLostAfter, "heartbeat.interval + heartbeat.apiTimeout %s must be at most controlPlaneLostAfter %s",
		heartbeatInterval+apiTimeout, controlPlaneLostAfter)
	check(3*heartbeatInterval <= controlPlaneLostAfter, "heartbeat.interval %s must be at most a third of controlPlaneLostAfter %s",
		heartbeatInterval, controlPlaneLostAfter)

	return errors.Join(errs...)
}
