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

package api

import (
	"context"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	swhcMu          sync.Mutex
	swhcCachedUntil time.Time
	swhcCachedOK    bool
)

func shouldUseSWHC(ctx context.Context, svc *Service) (bool, error) {
	now := time.Now()

	swhcMu.Lock()
	if !swhcCachedUntil.IsZero() && now.Before(swhcCachedUntil) {
		ok := swhcCachedOK
		swhcMu.Unlock()
		return ok, nil
	}
	swhcMu.Unlock()

	ok, err := detectSWHCResource(ctx, svc)

	swhcMu.Lock()
	swhcCachedUntil = now.Add(5 * time.Minute)
	swhcCachedOK = ok
	swhcMu.Unlock()

	return ok, err
}

func detectSWHCResource(ctx context.Context, svc *Service) (bool, error) {
	discovery := svc.clientset.Discovery()
	res, err := discovery.ServerResourcesForGroupVersion("network.deckhouse.io/v1alpha1")
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "the server could not find the requested resource") {
			return false, nil
		}
		if strings.Contains(msg, "could not find the requested resource") {
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
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no matches for kind") {
		return true
	}
	if strings.Contains(msg, "could not find the requested resource") {
		return true
	}
	if strings.Contains(msg, "the server could not find the requested resource") {
		return true
	}
	if strings.Contains(msg, "servicewithhealthchecks") && strings.Contains(msg, "not found") {
		return true
	}
	return false
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
) {
	probes := make([]map[string]any, 0, len(ports))
	for _, p := range ports {
		var target int32
		if p.TargetPort.Type == intstr.Int {
			target = p.TargetPort.IntVal
		} else {
			target = p.Port
		}
		probes = append(probes, map[string]any{
			"mode": "TCP",
			"tcp": map[string]any{
				"targetPort": target,
			},
		})
	}

	spec := map[string]any{
		"type":                  string(v1.ServiceTypeLoadBalancer),
		"ports":                 servicePortsToUnstructured(ports),
		"selector":              selector,
		"externalTrafficPolicy": string(externalTrafficPolicy),
		"healthcheck": map[string]any{
			"initialDelaySeconds": int64(10),
			"periodSeconds":       int64(10),
			"timeoutSeconds":      int64(1),
			"probes":              probes,
		},
	}

	if len(externalIPs) > 0 {
		spec["externalIPs"] = externalIPs
	}
	if loadBalancerClass != nil {
		spec["loadBalancerClass"] = *loadBalancerClass
	}
	if loadBalancerIP != "" {
		spec["loadBalancerIP"] = loadBalancerIP
	}

	_ = unstructured.SetNestedMap(u.Object, spec, "spec")
}

func servicePortsToUnstructured(ports []v1.ServicePort) []any {
	out := make([]any, 0, len(ports))
	for _, p := range ports {
		m := map[string]any{
			"port":     p.Port,
			"protocol": string(p.Protocol),
		}
		if p.Name != "" {
			m["name"] = p.Name
		}
		if p.AppProtocol != nil {
			m["appProtocol"] = *p.AppProtocol
		}
		if p.TargetPort.Type == intstr.Int {
			m["targetPort"] = p.TargetPort.IntVal
		} else if p.TargetPort.StrVal != "" {
			m["targetPort"] = p.TargetPort.StrVal
		}
		out = append(out, m)
	}
	return out
}
