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

package helper

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodService struct {
	client client.Client
}

func NewPodService(client client.Client) *PodService {
	return &PodService{
		client: client,
	}
}

func (s *PodService) ListByLabels(
	ctx context.Context,
	namespace string,
	labels map[string]string,
) ([]corev1.Pod, error) {
	var podList corev1.PodList
	err := s.client.List(
		ctx,
		&podList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	)
	if err != nil {
		return nil, err
	}

	// oldest first
	sort.SliceStable(podList.Items, func(i, j int) bool {
		return podList.Items[i].CreationTimestamp.Time.Before(
			podList.Items[j].CreationTimestamp.Time,
		)
	})

	return podList.Items, nil
}
