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

// Package fsm selects the phase of one FencingFailedNodeState. It does not read
// or write objects.
package fsm

import (
	"time"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// Params are the profile timings of one incident. They are not written to the
// object.
type Params struct {
	// Profile is the profile of the timings. It is empty for an unarmed node.
	Profile v1alpha1.ProfileName
	// Armed is spec.armed of the incident. An unarmed node is never proven.
	Armed bool
	// HeartbeatTTL is 2 * heartbeat.interval + heartbeat.apiTimeout of the
	// profile. The incident stays in FallbackAlive for this time after the
	// controller sees a heartbeat.
	HeartbeatTTL time.Duration
	// AttestedNodeTaintAfter is watchdog.timeout + watchdog.margin of the
	// profile.
	AttestedNodeTaintAfter time.Duration
	// HeartbeatInterval is heartbeat.interval of the profile.
	HeartbeatInterval time.Duration
	// SilentNodeTaintAfter is the time after the last sign of life of a node
	// without an attestation. After this time the watchdog stopped the node.
	SilentNodeTaintAfter time.Duration
}

// Observations are the moments when the controller saw the evidence of one
// incident. All deadlines start at these moments. The times that the writers
// put into the object do not start a deadline.
type Observations struct {
	// Incident is when the controller saw the object first.
	Incident time.Time
	// Heartbeat is when the controller saw the last new heartbeat of the node.
	// It is zero if the controller did not see a heartbeat.
	Heartbeat time.Time
	// Attestation is when the controller saw the attestation of the witness.
	// It is zero if the controller did not see an attestation.
	Attestation time.Time
}

// AliveAfterAttestation tells if the controller saw a heartbeat of the node
// after it saw the attestation. Then the node is alive, and the attestation
// does not prove the failure. The controller does not know the order of a
// heartbeat and an attestation that it saw in the same reconcile, for example
// in the first reconcile of a new leader. Such a heartbeat is after the
// attestation: the node can be alive.
func (o Observations) AliveAfterAttestation() bool {
	if o.Attestation.IsZero() || o.Heartbeat.IsZero() {
		return false
	}

	return !o.Heartbeat.Before(o.Attestation)
}

// Next returns the phase of the incident at now. It also returns the time after
// which the phase can change without a new observation. Zero means that only a
// new observation can change the phase.
//
// A new incident is Suspected. The incident is FallbackAlive while the last
// heartbeat that the controller saw is younger than HeartbeatTTL. After that,
// the incident is Suspected again.
//
// The incident of an armed node is Proven when AttestedNodeTaintAfter passed
// after the controller saw the attestation, and the node was not alive after
// it. A heartbeat before the attestation does not stop the proof. Without such
// an attestation, the incident of an armed node is Proven when
// SilentNodeTaintAfter passed after the last sign of life.
func Next(
	phase v1alpha1.FencingFailedNodeStatePhase,
	seen Observations,
	params Params,
	now time.Time,
) (v1alpha1.FencingFailedNodeStatePhase, time.Duration) {
	switch phase {
	case "", v1alpha1.PhaseSuspected, v1alpha1.PhaseFallbackAlive:
	default:
		// The controller moves a proven incident to eviction itself.
		return phase, 0
	}

	proofAfter, provable := ProvenAfter(seen, params, now)
	if provable && proofAfter <= 0 {
		return v1alpha1.PhaseProven, 0
	}

	if !provable {
		proofAfter = 0
	}

	if !seen.Heartbeat.IsZero() {
		if left := seen.Heartbeat.Add(params.HeartbeatTTL).Sub(now); left > 0 {
			if proofAfter > 0 {
				left = min(left, proofAfter)
			}

			return v1alpha1.PhaseFallbackAlive, left
		}
	}

	return v1alpha1.PhaseSuspected, proofAfter
}

// ProvenAfter returns the time after which the observations prove the failure
// of the node: the earlier of AttestedAfter and SilentAfter. Zero or less means
// that the failure is proven at now. The flag is false when these observations
// cannot prove the failure.
func ProvenAfter(seen Observations, params Params, now time.Time) (time.Duration, bool) {
	attested, byAttestation := AttestedAfter(seen, params, now)
	silent, bySilence := SilentAfter(seen, params, now)

	switch {
	case byAttestation && bySilence:
		return min(attested, silent), true
	case byAttestation:
		return attested, true
	default:
		return silent, bySilence
	}
}

// AttestedAfter returns the time after which the attestation proves the
// failure of the node. The flag is false when the node is not armed, the
// controller did not see an attestation, or the node was alive after it.
func AttestedAfter(seen Observations, params Params, now time.Time) (time.Duration, bool) {
	if !params.Armed || seen.Attestation.IsZero() || seen.AliveAfterAttestation() {
		return 0, false
	}

	return seen.Attestation.Add(params.AttestedNodeTaintAfter).Sub(now), true
}

// SilentAfter returns the time after which the silence of the node proves its
// failure. The last sign of life is the later of two moments: when the
// controller saw the object first, and when it saw the last heartbeat plus
// HeartbeatInterval. The flag is false when the node is not armed or the
// controller did not see the object.
func SilentAfter(seen Observations, params Params, now time.Time) (time.Duration, bool) {
	if !params.Armed || seen.Incident.IsZero() {
		return 0, false
	}

	alive := seen.Incident
	if !seen.Heartbeat.IsZero() {
		alive = later(alive, seen.Heartbeat.Add(params.HeartbeatInterval))
	}

	return alive.Add(params.SilentNodeTaintAfter).Sub(now), true
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}

	return a
}
