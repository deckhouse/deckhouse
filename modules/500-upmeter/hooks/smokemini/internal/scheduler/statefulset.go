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
	"errors"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

var (
	// ErrSkip is the legal abortion of scheduling
	ErrSkip = fmt.Errorf("scheduling skipped")

	// errNext lets one step in pipeline to pass the control to the next step
	errNext = fmt.Errorf("next step")
)

func NewStatefulSetSelector(nodes []snapshot.Node, storageClass string, pods []snapshot.Pod, disruptionAllowed bool) IndexSelectorPipe {
	xSel := IndexSelectorPipe{
		&selectByNode{nodes: nodes},
		&selectByStorageClass{storageClass: storageClass},
		&selectByPod{pods: pods, disruptionAllowed: disruptionAllowed},
	}
	return xSel
}

type IndexSelector interface {
	Select(State) (string, error)
}

// IndexSelectorPipe is the sequential wrapper for other sts selectors. The result is returned from the
// first successful selection or abortion error. Selection is ignored on next error.
type IndexSelectorPipe []IndexSelector

func (s IndexSelectorPipe) Select(state State) (string, error) {
	for _, s := range s {
		x, err := s.Select(state)
		if errors.Is(err, errNext) {
			continue
		}
		return x, err
	}
	return "", ErrSkip
}

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
		notRunning := pod.Phase != v1.PodRunning
		tooLong := pod.Created.Before(pendingThreshold)
		if notRunning && tooLong {
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

type selectByStorageClass struct {
	storageClass string
}

func (s *selectByStorageClass) Select(state State) (string, error) {
	// Find sts with outdated storage class
	for x, sts := range state {
		if sts.StorageClass != s.storageClass {
			return x, nil
		}
	}
	return "", errNext
}

type selectByNode struct {
	nodes []snapshot.Node
}

func (s *selectByNode) Select(state State) (string, error) {
	// Not deployed sts
	for x, sts := range state {
		if sts.Node == "" {
			return x, nil
		}
	}

	// Collect nodes
	allNodes := set.New()
	unschedNodes := set.New()
	for _, node := range s.nodes {
		allNodes.Add(node.Name)
		if !node.Schedulable {
			unschedNodes.Add(node.Name)
		}
	}

	// Find sts placed on non-existent node
	for x, sts := range state {
		if !allNodes.Has(sts.Node) {
			return x, nil
		}
	}

	// Find sts placed on unavailable node
	for x, sts := range state {
		if unschedNodes.Has(sts.Node) {
			return x, nil
		}
	}

	return "", errNext
}
