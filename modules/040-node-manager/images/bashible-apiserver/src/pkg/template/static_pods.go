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

package template

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type staticPodRequest struct {
	Name              string
	Manifest          string
	CreationTimestamp time.Time
}

func (s *StepsStorage) AddStaticPodRequest(request *NodeStaticPodRequest) {
	klog.Infof("Adding NodeStaticPodRequest %s to context", request.Name)

	stored := staticPodRequest{
		Name:              request.Name,
		Manifest:          request.Spec.Manifest,
		CreationTimestamp: request.CreationTimestamp.Time,
	}

	s.m.Lock()
	defer s.m.Unlock()
	for _, ngPair := range generateNgPairs(request.Spec.NodeGroupSelector.MatchNames) {
		s.staticPodRequests[ngPair] = append(s.staticPodRequests[ngPair], &stored)
	}
}

func (s *StepsStorage) RemoveStaticPodRequest(request *NodeStaticPodRequest) {
	klog.Infof("Removing NodeStaticPodRequest %s from context", request.Name)

	s.m.Lock()
	defer s.m.Unlock()
	for _, ngPair := range generateNgPairs(request.Spec.NodeGroupSelector.MatchNames) {
		stored := s.staticPodRequests[ngPair]
		for i, candidate := range stored {
			if candidate.Name != request.Name {
				continue
			}
			s.staticPodRequests[ngPair] = append(stored[:i], stored[i+1:]...)
			break
		}
	}
}

// staticPodsFor returns this group's requests and the wildcard ones, sorted by
// object name. The pod contest is settled over all stored requests, not just
// this group's, so an object that lost a pod is refused by every group.
func (s *StepsStorage) staticPodsFor(ng string) []*staticPodRequest {
	keyNg := fmt.Sprintf("*:%s", ng)
	wildcard := "*:*"

	s.m.RLock()
	defer s.m.RUnlock()

	accepted := acceptedStaticPods(s.staticPodRequests)

	requests := make([]*staticPodRequest, 0, len(s.staticPodRequests[keyNg])+len(s.staticPodRequests[wildcard]))
	for _, request := range slices.Concat(s.staticPodRequests[keyNg], s.staticPodRequests[wildcard]) {
		if !accepted[request.Name] {
			continue
		}
		requests = append(requests, request)
	}

	slices.SortFunc(requests, func(a, b *staticPodRequest) int {
		return strings.Compare(a.Name, b.Name)
	})

	return requests
}

// acceptedStaticPods names the requests that claimed their pod: oldest first,
// ties by object name, one manifest per namespace/name. Mirrors rejectedNSPRs
// in node-controller (internal/controller/nodeconfig/staticpods.go).
func acceptedStaticPods(stored map[string][]*staticPodRequest) map[string]bool {
	ordered := make([]*staticPodRequest, 0, len(stored))
	seen := make(map[string]bool, len(stored))

	for _, requests := range stored {
		for _, request := range requests {
			if seen[request.Name] {
				continue
			}
			seen[request.Name] = true
			ordered = append(ordered, request)
		}
	}

	slices.SortFunc(ordered, func(a, b *staticPodRequest) int {
		if !a.CreationTimestamp.Equal(b.CreationTimestamp) {
			return a.CreationTimestamp.Compare(b.CreationTimestamp)
		}

		return strings.Compare(a.Name, b.Name)
	})

	accepted := make(map[string]bool, len(ordered))
	claimed := make(map[string]string, len(ordered))

	for _, request := range ordered {
		identity, err := podIdentity(request.Manifest)
		if err != nil {
			klog.Errorf("Skipping NodeStaticPodRequest %s: %s", request.Name, err)
			continue
		}

		if owner, taken := claimed[identity]; taken {
			klog.Errorf("Skipping NodeStaticPodRequest %s: pod %s is already asked for by %s, which is older", request.Name, identity, owner)
			continue
		}

		claimed[identity] = request.Name
		accepted[request.Name] = true
	}

	return accepted
}

// podIdentity reads the namespace/name the manifest claims. Only those two
// fields are decoded: the manifest is run by kubelet's Pod type, not the one
// vendored here, and node-controller's webhook refused anything that is not one.
func podIdentity(manifest string) (string, error) {
	var pod struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	}

	if err := yaml.Unmarshal([]byte(manifest), &pod); err != nil {
		return "", fmt.Errorf("parse manifest: %w", err)
	}

	if pod.Metadata.Name == "" {
		return "", fmt.Errorf("parse manifest: metadata.name is empty")
	}

	return pod.Metadata.Namespace + "/" + pod.Metadata.Name, nil
}
