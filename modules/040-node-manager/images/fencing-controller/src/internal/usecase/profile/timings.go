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

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/domain/fsm"
)

// timings takes the two values the controller acts on: fallback.ttl as
// FallbackTTL and evacuation.delay as EvacuationDelay. Everything else in the
// profile configures the agent and is not read here.
//
// Both values are re-checked, because the CRD schema and its CEL rules guard
// admission only: an object may predate them, and a non-positive timing would
// either drop the fallback protection or evacuate with no delay at all.
func timings(profile *v1alpha1.FencingSLAProfile) (fsm.Params, error) {
	if profile == nil {
		return fsm.Params{}, errors.New("profile is nil")
	}

	var errs []error

	ttl := profile.Spec.Fallback.TTL.Duration
	if ttl <= 0 {
		errs = append(errs, fmt.Errorf("fallback.ttl: must be positive, got %s", ttl))
	}

	delay := profile.Spec.Evacuation.Delay.Duration
	if delay <= 0 {
		errs = append(errs, fmt.Errorf("evacuation.delay: must be positive, got %s", delay))
	}

	if len(errs) > 0 {
		return fsm.Params{}, errors.Join(errs...)
	}

	return fsm.Params{FallbackTTL: ttl, EvacuationDelay: delay}, nil
}
