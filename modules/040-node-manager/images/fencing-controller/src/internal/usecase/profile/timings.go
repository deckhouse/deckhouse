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
	"time"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/domain/fsm"
)

const (
	// lossConfirmation is the time after controlPlaneLostAfter in which the
	// agent can still feed the watchdog: the agent declares the control plane
	// lost after two reads of the probes, one read in each 100ms.
	lossConfirmation = 200 * time.Millisecond

	// silentMargin covers two delays: the agent can see its object a short
	// time after the controller, and its first heartbeat can come late.
	silentMargin = 900 * time.Millisecond
)

// timings takes the values the controller acts on: controlPlaneLostAfter,
// watchdog.timeout, watchdog.margin, heartbeat.interval and
// heartbeat.apiTimeout. Everything else in the profile configures the agent and
// is not read here. The schema of the CRD checks the values.
//
// A silent node fed the watchdog for controlPlaneLostAfter + lossConfirmation
// after its last sign of life at most. Then the watchdog stops the node in
// watchdog.timeout, and watchdog.margin covers the inaccuracy of the timers.
func timings(profile *v1alpha1.FencingSLAProfile) fsm.Params {
	spec := profile.Spec

	return fsm.Params{
		HeartbeatTTL:           2*spec.Heartbeat.Interval.Duration + spec.Heartbeat.APITimeout.Duration,
		AttestedNodeTaintAfter: spec.Watchdog.Timeout.Duration + spec.Watchdog.Margin.Duration,
		HeartbeatInterval:      spec.Heartbeat.Interval.Duration,
		SilentNodeTaintAfter: spec.ControlPlaneLostAfter.Duration + lossConfirmation +
			spec.Watchdog.Timeout.Duration + spec.Watchdog.Margin.Duration + silentMargin,
	}
}
