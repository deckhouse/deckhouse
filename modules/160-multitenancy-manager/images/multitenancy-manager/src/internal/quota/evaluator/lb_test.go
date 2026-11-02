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

package evaluator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func service(class string, annotations map[string]string) *corev1.Service {
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "team", Annotations: annotations},
		Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer}}
	if class != "" {
		svc.Spec.LoadBalancerClass = ptr.To(class)
	}
	return svc
}

func TestLoadBalancerUsage(t *testing.T) {
	n := func(v int64) resource.Quantity { return *resource.NewQuantity(v, resource.DecimalSI) }
	tests := []struct {
		name     string
		svc      *corev1.Service
		defaults string
		want     corev1.ResourceList
	}{
		{name: "a class of its own", svc: service("l2-a", nil),
			want: corev1.ResourceList{LBClassServices("l2-a"): n(1), LBClassAddresses("l2-a"): n(1), LBAddresses: n(1)}},
		{name: "the legacy annotation of the class", svc: service("", map[string]string{"network.deckhouse.io/metal-load-balancer-class": "l2-b"}),
			defaults: "l2-default", want: corev1.ResourceList{LBClassServices("l2-b"): n(1), LBClassAddresses("l2-b"): n(1), LBAddresses: n(1)}},
		{name: "the default class", svc: service("", nil), defaults: "l2-default",
			want: corev1.ResourceList{LBClassServices("l2-default"): n(1), LBClassAddresses("l2-default"): n(1), LBAddresses: n(1)}},
		{name: "a cloud load balancer without a class", svc: service("", nil),
			want: corev1.ResourceList{LBClassServices("default"): n(1), LBClassAddresses("default"): n(1), LBAddresses: n(1)}},
		{name: "several addresses by count", svc: service("l2-a", map[string]string{"network.deckhouse.io/l2-load-balancer-external-ips-count": "3"}),
			want: corev1.ResourceList{LBClassServices("l2-a"): n(1), LBClassAddresses("l2-a"): n(3), LBAddresses: n(3)}},
		{name: "a list of addresses without a count takes one", svc: service("l2-a", map[string]string{"network.deckhouse.io/load-balancer-ips": "10.0.0.1, 10.0.0.2"}),
			want: corev1.ResourceList{LBClassServices("l2-a"): n(1), LBClassAddresses("l2-a"): n(1), LBAddresses: n(1)}},
		{name: "a list of addresses with a count", svc: service("l2-a", map[string]string{"network.deckhouse.io/load-balancer-ips": "10.0.0.1, 10.0.0.2",
			"network.deckhouse.io/l2-load-balancer-external-ips-count": "2"}),
			want: corev1.ResourceList{LBClassServices("l2-a"): n(1), LBClassAddresses("l2-a"): n(2), LBAddresses: n(2)}},
		{name: "a pool of the BGP mode takes one address", svc: service("", map[string]string{"network.deckhouse.io/load-balancer-pool": "bgp-1",
			"network.deckhouse.io/l2-load-balancer-external-ips-count": "3"}),
			want: corev1.ResourceList{LBClassServices("default"): n(1), LBClassAddresses("default"): n(1), LBAddresses: n(1), LBPoolAddresses("bgp-1"): n(1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadBalancer{DefaultClass: tt.defaults, Classes: map[string]bool{"l2-a": true, "l2-b": true, "l2-default": true}}.Usage(tt.svc)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadBalancerUsageOfOtherServices(t *testing.T) {
	svc := service("l2-a", nil)
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	got, err := LoadBalancer{}.Usage(svc)
	require.NoError(t, err)
	assert.Empty(t, got)

}

// A count of addresses that is not a number above one takes one address, as the metallb module
// gives it: the Service is still counted, and no Service fails to evaluate.
func TestLoadBalancerUsageOfAnUnreadableCount(t *testing.T) {
	for _, value := range []string{"0", "-1", "abc", " 3", ""} {
		got, err := LoadBalancer{}.Usage(service("l2-a", map[string]string{"network.deckhouse.io/l2-load-balancer-external-ips-count": value}))
		require.NoError(t, err, value)
		addresses := got[LBAddresses]
		assert.Equal(t, int64(1), addresses.Value(), value)
	}
}

// The class is the one the metallb module serves the Service with: the class bound in the status
// first, a named class only when it exists, else the default one.
func TestLoadBalancerClassAsMetalLB(t *testing.T) {
	classes := map[string]bool{"l2-a": true, "l2-b": true, "l2-default": true}
	bound := func(svc *corev1.Service, class string) *corev1.Service {
		svc.Status.Conditions = []metav1.Condition{{Type: "network.deckhouse.io/load-balancer-class", Status: metav1.ConditionTrue, Message: class}}
		return svc
	}
	tests := []struct {
		name string
		svc  *corev1.Service
		lb   LoadBalancer
		want string
	}{
		{name: "an existing class of the spec", svc: service("l2-a", map[string]string{"network.deckhouse.io/metal-load-balancer-class": "l2-b"}),
			lb: LoadBalancer{DefaultClass: "l2-default", Classes: classes}, want: "l2-a"},
		{name: "an existing class of the annotation", svc: service("", map[string]string{"network.deckhouse.io/metal-load-balancer-class": "l2-b"}),
			lb: LoadBalancer{DefaultClass: "l2-default", Classes: classes}, want: "l2-b"},
		{name: "a class that does not exist takes the default one", svc: service("", map[string]string{"network.deckhouse.io/metal-load-balancer-class": "nosuch"}),
			lb: LoadBalancer{DefaultClass: "l2-default", Classes: classes}, want: "l2-default"},
		{name: "the class bound in the status wins", svc: bound(service("", map[string]string{"network.deckhouse.io/metal-load-balancer-class": "l2-b"}), "l2-a"),
			lb: LoadBalancer{DefaultClass: "l2-default", Classes: classes}, want: "l2-a"},
		{name: "a class of another controller without a default class", svc: service("other", nil),
			lb: LoadBalancer{Classes: classes}, want: "other"},
		{name: "a class that does not exist without a default class", svc: service("", map[string]string{"network.deckhouse.io/metal-load-balancer-class": "nosuch"}),
			lb: LoadBalancer{Classes: classes}, want: "default"},
		{name: "the class of the spec taken as it is when the classes are not served", svc: service("other", map[string]string{"network.deckhouse.io/metal-load-balancer-class": "x"}),
			lb: LoadBalancer{}, want: "other"},
		{name: "the annotation not read when the classes are not served", svc: service("", map[string]string{"network.deckhouse.io/metal-load-balancer-class": "x"}),
			lb: LoadBalancer{}, want: "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.lb.Usage(tt.svc)
			require.NoError(t, err)
			assert.Contains(t, got, LBClassServices(tt.want))
		})
	}
}

func TestIsLBKey(t *testing.T) {
	for _, key := range []corev1.ResourceName{LBAddresses, LBClassServices("a"), LBClassAddresses("a"), LBPoolAddresses("p")} {
		assert.True(t, IsLBKey(key), key)
	}
	for _, key := range []corev1.ResourceName{"services.loadbalancers", "a.loadbalancerclass.network.deckhouse.io/ports", ".loadbalancerpool.network.deckhouse.io/addresses"} {
		assert.False(t, IsLBKey(key), key)
	}
}

// Every load balancer counts its own addresses, also the ones that share an address under one key.
func TestSumLoadBalancers(t *testing.T) {
	shared := map[string]string{"network.deckhouse.io/load-balancer-shared-ip-key": "k"}
	a := *service("l2", shared)
	b := *service("l2", shared)
	b.Name = "web-udp"
	other := *service("l2", shared)
	other.Namespace = "other"
	plain := *service("l2", nil)
	plain.Name = "dns"
	clusterIP := *service("l2", nil)
	clusterIP.Spec.Type = corev1.ServiceTypeClusterIP

	got := LoadBalancer{}.SumLoadBalancers([]corev1.Service{a, b, other, plain, clusterIP})
	services := got[LBClassServices("l2")]
	addresses := got[LBAddresses]
	assert.Equal(t, int64(4), services.Value())
	assert.Equal(t, int64(4), addresses.Value(), "web and web-udp count an address each though they share one key")
}

// A trimmed Service counts and shares like the whole one and keeps nothing else.
func TestTrimService(t *testing.T) {
	svc := service("l2-a", map[string]string{"network.deckhouse.io/l2-load-balancer-external-ips-count": "2",
		"kubectl.kubernetes.io/last-applied-configuration": "{...}"})
	svc.Labels = map[string]string{"app": "web"}
	svc.Spec.Ports = []corev1.ServicePort{{Name: "http", Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8080), NodePort: 30080}}
	svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
	svc.Spec.Selector = map[string]string{"app": "web"}
	svc.Status.Conditions = []metav1.Condition{{Type: "network.deckhouse.io/load-balancer-class", Status: metav1.ConditionTrue, Message: "l2-a", Reason: "Bound"}}

	out, err := TrimService(svc)
	require.NoError(t, err)
	trimmed := out.(*corev1.Service)
	lb := LoadBalancer{Classes: map[string]bool{"l2-a": true}}
	want, err := lb.Usage(svc)
	require.NoError(t, err)
	got, err := lb.Usage(trimmed)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.NotContains(t, trimmed.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	assert.Empty(t, trimmed.Labels)
	assert.Empty(t, trimmed.Spec.Ports)
	assert.Empty(t, trimmed.Spec.Selector)

	clusterIP := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "db"}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{{Port: 5432}}, Selector: map[string]string{"app": "db"}}}
	out, err = TrimService(clusterIP)
	require.NoError(t, err)
	assert.Empty(t, out.(*corev1.Service).Spec.Ports, "a Service of another type keeps only its type")
}
