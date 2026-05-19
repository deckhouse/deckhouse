/*
Copyright 2025 Flant JSC

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

// This file contains helper logic for ServiceWithHealthchecks support.
// It detects whether the ServiceWithHealthchecks API is available in the cluster,
// caches the detection result, builds unstructured ServiceWithHealthchecks objects,
// and converts Kubernetes Service fields into the corresponding unstructured spec.
package api

import (
	"fmt"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
)

const swhcCacheTTL = 5 * time.Minute

type swhcCache struct {
	mu          sync.Mutex
	cachedUntil time.Time
	cachedOK    bool
}

func (lb *LoadBalancerService) shouldUseSWHC() bool {
	now := time.Now()

	lb.swhcCache.mu.Lock()
	if !lb.swhcCache.cachedUntil.IsZero() && now.Before(lb.swhcCache.cachedUntil) {
		ok := lb.swhcCache.cachedOK
		lb.swhcCache.mu.Unlock()
		return ok
	}
	lb.swhcCache.mu.Unlock()

	ok, err := detectSWHCResource(lb.Service)
	if err != nil {
		klog.V(4).InfoS("shouldUseSWHC: detection failed, assuming SWHC unavailable", "err", err)
		return false
	}

	lb.swhcCache.mu.Lock()
	lb.swhcCache.cachedUntil = now.Add(swhcCacheTTL)
	lb.swhcCache.cachedOK = ok
	lb.swhcCache.mu.Unlock()

	return ok
}

func detectSWHCResource(svc *Service) (bool, error) {
	res, err := svc.clientset.Discovery().ServerResourcesForGroupVersion("network.deckhouse.io/v1alpha1")
	if err != nil {
		if isSWHCUnsupportedErr(err) {
			return false, nil
		}
		return false, err
	}

	for _, r := range res.APIResources {
		if r.Name == "servicewithhealthchecks" {
			return true, nil
		}
	}
	return false, nil
}

func isSWHCUnsupportedErr(err error) bool {
	if err == nil {
		return false
	}
	if k8serrors.IsNotFound(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no matches for kind") ||
		strings.Contains(msg, "could not find the requested resource")
}

func newServiceWithHealthchecksUnstructured(namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("network.deckhouse.io/v1alpha1")
	u.SetKind("ServiceWithHealthchecks")
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

func setServiceWithHealthchecksSpec(
	u *unstructured.Unstructured,
	ports []v1.ServicePort,
	externalTrafficPolicy v1.ServiceExternalTrafficPolicyType,
	selector map[string]string,
	externalIPs []string,
	loadBalancerClass *string,
	loadBalancerIP string,
) error {
	probes := make([]any, 0, len(ports))
	for _, p := range ports {
		var target int64
		if p.TargetPort.Type == intstr.Int {
			target = int64(p.TargetPort.IntVal)
		} else {
			target = int64(p.Port)
		}
		probes = append(probes, map[string]any{
			"mode": "TCP",
			"tcp": map[string]any{
				"targetPort": target,
			},
		})
	}

	selectorAny := make(map[string]any, len(selector))
	for k, v := range selector {
		selectorAny[k] = v
	}

	spec := map[string]any{
		"type":                  string(v1.ServiceTypeLoadBalancer),
		"ports":                 servicePortsToUnstructured(ports),
		"selector":              selectorAny,
		"externalTrafficPolicy": string(externalTrafficPolicy),
		"healthcheck": map[string]any{
			"initialDelaySeconds": int64(10),
			"periodSeconds":       int64(10),
			"timeoutSeconds":      int64(1),
			"probes":              probes,
		},
	}

	if len(externalIPs) > 0 {
		externalIPsAny := make([]any, len(externalIPs))
		for i, ip := range externalIPs {
			externalIPsAny[i] = ip
		}
		spec["externalIPs"] = externalIPsAny
	}
	if loadBalancerClass != nil {
		spec["loadBalancerClass"] = *loadBalancerClass
	}
	if loadBalancerIP != "" {
		spec["loadBalancerIP"] = loadBalancerIP
	}

	if err := unstructured.SetNestedMap(u.Object, spec, "spec"); err != nil {
		return fmt.Errorf("set SWHC spec: %w", err)
	}
	return nil
}

func servicePortsToUnstructured(ports []v1.ServicePort) []any {
	out := make([]any, 0, len(ports))
	for _, p := range ports {
		m := map[string]any{
			"port":     int64(p.Port),
			"protocol": string(p.Protocol),
		}
		if p.Name != "" {
			m["name"] = p.Name
		}
		if p.AppProtocol != nil {
			m["appProtocol"] = *p.AppProtocol
		}
		if p.TargetPort.Type == intstr.Int {
			m["targetPort"] = int64(p.TargetPort.IntVal)
		} else if p.TargetPort.StrVal != "" {
			m["targetPort"] = p.TargetPort.StrVal
		}
		out = append(out, m)
	}
	return out
}
