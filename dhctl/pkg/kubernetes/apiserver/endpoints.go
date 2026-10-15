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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	endpointSliceNamespace   = "default"
	endpointSliceName        = "kubernetes"
	endpointSlicePortName    = "https"
	kubeAPIServerPodPort     = 6443
	kubeAPIServerPodSelector = "component=kube-apiserver,tier=control-plane"
)

// GetEndpoints returns sorted unique API server endpoints in host:port format.
// It merges non-terminating kube-apiserver Pods with the kubernetes EndpointSlice.
// Readiness is intentionally not checked.
// Mirrors sourceReader.readAPIServerEndpoints in
// modules/040-node-manager/images/node-controller/src/internal/controller/nodeconfig/sources.go.
func GetEndpoints(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
) ([]string, error) {
	endpoints := make(map[string]struct{})

	pods, err := listAPIServerPods(ctx, kubeCl)
	if err != nil {
		return nil, err
	}

	for i := range pods {
		endpoint := net.JoinHostPort(
			pods[i].Status.PodIP,
			strconv.Itoa(kubeAPIServerPodPort),
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

// GetReadyHostsForNodes returns sorted unique PodIPs of the Ready
// kube-apiserver Pods running on the specified nodes. Readiness comes from
// the Pods only: the kubernetes EndpointSlice is kept by the lease reconciler
// and still lists an apiserver whose Pod is NotReady. node-controller writes
// the same PodIPs to apiserverEndpoints of the bootstrap Secrets, so callers
// can wait for these hosts to appear there.
func GetReadyHostsForNodes(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	nodeNames []string,
) ([]string, error) {
	if len(nodeNames) == 0 {
		return nil, errors.New("no control-plane nodes specified")
	}

	pods, err := listAPIServerPods(ctx, kubeCl)
	if err != nil {
		return nil, err
	}

	hostsByNode := make(map[string][]string, len(nodeNames))
	for i := range pods {
		pod := &pods[i]
		if podIsReady(pod.Status.Conditions) {
			hostsByNode[pod.Spec.NodeName] = append(hostsByNode[pod.Spec.NodeName], pod.Status.PodIP)
		}
	}

	hosts := make(map[string]struct{}, len(nodeNames))
	for _, nodeName := range nodeNames {
		nodeHosts := hostsByNode[nodeName]
		if len(nodeHosts) == 0 {
			return nil, fmt.Errorf(
				"control-plane node %q has no ready API server endpoint",
				nodeName,
			)
		}

		for _, host := range nodeHosts {
			hosts[host] = struct{}{}
		}
	}

	return slices.Sorted(maps.Keys(hosts)), nil
}

// listAPIServerPods returns non-terminating kube-apiserver Pods that have an IP.
func listAPIServerPods(ctx context.Context, kubeCl *client.KubernetesClient) ([]corev1.Pod, error) {
	pods, err := kubeCl.CoreV1().
		Pods(metav1.NamespaceSystem).
		List(ctx, metav1.ListOptions{
			LabelSelector: kubeAPIServerPodSelector,
		})
	if err != nil {
		return nil, fmt.Errorf("list kube-apiserver pods: %w", err)
	}

	return slices.DeleteFunc(pods.Items, func(pod corev1.Pod) bool {
		return pod.DeletionTimestamp != nil || pod.Status.PodIP == ""
	}), nil
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
