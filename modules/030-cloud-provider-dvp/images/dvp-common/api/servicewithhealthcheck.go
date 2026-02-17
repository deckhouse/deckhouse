package api

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

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
		target := int32(0)
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
			"probes":              probes,
			"timeoutSeconds":      int64(1),
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
		// targetPort
		if p.TargetPort.Type == intstr.Int {
			m["targetPort"] = p.TargetPort.IntVal
		} else if p.TargetPort.StrVal != "" {
			m["targetPort"] = p.TargetPort.StrVal
		}
		out = append(out, m)
	}
	return out
}
