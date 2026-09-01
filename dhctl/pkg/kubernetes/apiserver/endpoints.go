// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apiserver

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	kubeSystemNamespace      = "kube-system"
	endpointSliceNamespace   = "default"
	endpointSliceName        = "kubernetes"
	endpointSlicePortName    = "https"
	defaultAPIServerPort     = 6443
	kubeAPIServerPodSelector = "component=kube-apiserver,tier=control-plane"
)

// GetEndpoints returns sorted unique API server endpoints in host:port format.
// It merges non-terminating kube-apiserver Pods with the kubernetes EndpointSlice.
// Readiness is intentionally not checked; callers that require ready endpoints
// must use GetReadyEndpoints.
func GetEndpoints(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
) ([]string, error) {
	return getEndpoints(ctx, kubeCl, false)
}

// GetReadyEndpoints returns sorted unique endpoints that are ready to accept
// connections. Terminating and explicitly not-ready endpoints are excluded.
func GetReadyEndpoints(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
) ([]string, error) {
	return getEndpoints(ctx, kubeCl, true)
}

// getEndpoints returns sorted unique API server endpoints in host:port format.
// It merges non-terminating kube-apiserver Pods with the kubernetes EndpointSlice.
// Readiness is intentionally not checked; callers that require ready endpoints
// must apply their own safety filtering.
func getEndpoints(ctx context.Context, kubeCl *client.KubernetesClient, readyOnly bool) ([]string, error) {
	endpoints := make(map[string]struct{})

	pods, err := kubeCl.CoreV1().
		Pods(kubeSystemNamespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: kubeAPIServerPodSelector,
		})
	if err != nil {
		return nil, fmt.Errorf("list kube-apiserver pods: %w", err)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]

		if pod.DeletionTimestamp != nil || pod.Status.PodIP == "" {
			continue
		}

		if readyOnly && !podIsReady(pod.Status.Conditions) {
			continue
		}

		endpoint := net.JoinHostPort(
			pod.Status.PodIP,
			strconv.Itoa(defaultAPIServerPort),
		)
		endpoints[endpoint] = struct{}{}
	}

	endpointSlice, err := kubeCl.DiscoveryV1().
		EndpointSlices(endpointSliceNamespace).
		Get(ctx, endpointSliceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf(
			"read the %s/%s EndpointSlice: %w",
			endpointSliceNamespace,
			endpointSliceName,
			err,
		)
	}

	var ports []int32
	for _, port := range endpointSlice.Ports {
		if port.Name == nil || *port.Name != endpointSlicePortName {
			continue
		}
		if port.Port == nil {
			continue
		}

		ports = append(ports, *port.Port)
	}

	for _, endpoint := range endpointSlice.Endpoints {
		if readyOnly && !endpointIsReady(endpoint.Conditions) {
			continue
		}
		for _, address := range endpoint.Addresses {
			for _, port := range ports {
				hostPort := net.JoinHostPort(
					address,
					strconv.Itoa(int(port)),
				)
				endpoints[hostPort] = struct{}{}
			}
		}
	}

	if len(endpoints) == 0 {
		return nil, errors.New("the cluster reports no apiserver endpoints")
	}

	return slices.Sorted(maps.Keys(endpoints)), nil
}

func podIsReady(conditions []corev1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type != corev1.PodReady {
			continue
		}

		return condition.Status == corev1.ConditionTrue
	}

	return false
}

func endpointIsReady(conditions discoveryv1.EndpointConditions) bool {
	if conditions.Terminating != nil && *conditions.Terminating {
		return false
	}

	// Ready=nil means that readiness is unknown. EndpointSlice consumers
	// interpret an unknown value as ready for backward compatibility.
	return conditions.Ready == nil || *conditions.Ready
}
