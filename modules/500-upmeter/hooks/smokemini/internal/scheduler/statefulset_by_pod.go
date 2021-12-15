/*
Copyright 2021 Flant JSC

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

package scheduler

import (
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

type selectByPod struct {
	pods              []snapshot.Pod
	disruptionAllowed bool
}

func (s *selectByPod) Select(state State) (string, error) {
	// Find absent pod
	pods := set.New()
	for _, p := range s.pods {
		pods.Add(p.Index)
	}
	for x := range state {
		// Absent pod that had been already scheduled, should be fixed
		if !pods.Has(x) && state[x].scheduled() {
			return x, nil
		}
	}

	// Find pods that stuck, otherwise PDB will block StatefulSet updates. There can be race between
	// node assignment by the hook and node cordoning by some extra factors (e.g. autoscaling or node
	// upgrade). It can lead to infinitely pending pod.
	pendingThreshold := time.Now().Add(-time.Minute)
	for _, pod := range s.pods {
		tooLong := pod.Created.Before(pendingThreshold)
		if !pod.Ready && tooLong {
			return pod.Index, nil
		}
	}

	if !s.disruptionAllowed {
		// Aborting to save available pods
		return "", fmt.Errorf("%w: no disruption allowed", ErrSkip)
	}

	// Find sts which was not moved for the longest time. Oldest pod cannot be younger than N-1 crontab
	// periods. Since we have a fixed number of smoke-mini pods (N=5), and we could have just
	// rescheduled a pod, the oldest one should be at least 4 minutes old.
	threshold := time.Now().Add(-4 * time.Minute)
	var (
		x   string
		err = errNext
	)
	for _, pod := range s.pods {
		if pod.Created.Before(threshold) {
			threshold = pod.Created
			x, err = pod.Index, nil
		}
	}
	return x, err
}
