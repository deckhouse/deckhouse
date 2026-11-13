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

package decision

import (
	"time"

	"fencing-agent/internal/config"
)

// Params are the decision parameters from the profile and from the common timings.
type Params struct {
	ControlPlaneLostAfter time.Duration
	// WitnessFresherBy is how much fresher the probe of the answering peer must be.
	WitnessFresherBy time.Duration
	// RoundTimeout is the timeout of a round. ConfirmTimeout is the timeout of the repeated question.
	RoundTimeout   time.Duration
	ConfirmTimeout time.Duration
}

// ParamsFrom takes the parameters from the applied profile and the poll
// timeouts from the common timings. For a Disabled group the applied profile
// is the memberlist profile.
func ParamsFrom(s config.Settings) Params {
	p, t := s.Profile, s.Config.CommonTimings
	return Params{
		ControlPlaneLostAfter: p.ControlPlaneLostAfter.Duration,
		WitnessFresherBy:      p.Poll.WitnessFresherBy.Duration,
		RoundTimeout:          t.RoundTimeout.Duration,
		ConfirmTimeout:        t.ConfirmTimeout.Duration,
	}
}
