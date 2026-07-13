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

package bashiblecontext

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	apiserverPort              = 6443
	packagesProxyPort          = 4219
	packagesProxyBootstrapPort = 4282
)

type endpoints struct {
	apiserverEndpoints     []string
	clusterMasterEndpoints []map[string]interface{}
}

func (s *Service) readEndpoints(ctx context.Context) endpoints {
	set := make(map[string]struct{})

	pods := &corev1.PodList{}
	if err := s.Client.List(ctx, pods,
		client.InNamespace(kubeSystemNS),
		client.MatchingLabels{"component": "kube-apiserver", "tier": "control-plane"},
	); err == nil {
		for i := range pods.Items {
			pod := &pods.Items[i]
			if !podReady(pod) {
				continue
			}
			set[fmt.Sprintf("%s:%d", pod.Status.PodIP, apiserverPort)] = struct{}{}
		}
	}

	slice := &discoveryv1.EndpointSlice{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: "default", Name: "kubernetes"}, slice); err == nil {
		var ports []int32
		for _, port := range slice.Ports {
			if port.Name != nil && *port.Name == "https" && port.Port != nil {
				ports = append(ports, *port.Port)
			}
		}
		for _, endpoint := range slice.Endpoints {
			for _, addr := range endpoint.Addresses {
				for _, port := range ports {
					set[net.JoinHostPort(addr, strconv.Itoa(int(port)))] = struct{}{}
				}
			}
		}
	}

	delete(set, "")

	list := make([]string, 0, len(set))
	for ep := range set {
		list = append(list, ep)
	}
	sort.Strings(list)

	res := endpoints{
		apiserverEndpoints:     list,
		clusterMasterEndpoints: make([]map[string]interface{}, 0, len(list)),
	}
	for _, ep := range list {
		address, port, err := net.SplitHostPort(ep)
		if err != nil {
			continue
		}
		kubeAPIPort, err := strconv.Atoi(port)
		if err != nil {
			continue
		}
		res.clusterMasterEndpoints = append(res.clusterMasterEndpoints, map[string]interface{}{
			"address":                address,
			"kubeApiPort":            kubeAPIPort,
			"rppServerPort":          packagesProxyPort,
			"rppBootstrapServerPort": packagesProxyBootstrapPort,
		})
	}
	return res
}

func podReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
