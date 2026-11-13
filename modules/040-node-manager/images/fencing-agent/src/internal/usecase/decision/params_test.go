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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/config"
)

func TestParamsFrom(t *testing.T) {
	d := func(v time.Duration) metav1.Duration { return metav1.Duration{Duration: v} }
	// The poll timeouts are the common timings of the ConfigMap, not of the profile.
	set := config.Settings{
		Config: config.Config{CommonTimings: config.CommonTimings{RoundTimeout: d(150 * ms), ConfirmTimeout: d(600 * ms), FastReadAfter: d(1200 * ms)}},
		Profile: v1alpha1.FencingSLAProfileSpec{
			ControlPlaneLostAfter: d(4150 * ms),
			Poll:                  v1alpha1.FencingSLAProfilePoll{WitnessFresherBy: d(2350 * ms)},
		},
	}
	if got := ParamsFrom(set); got != critical.params {
		t.Errorf("ParamsFrom = %+v, want %+v", got, critical.params)
	}
}
