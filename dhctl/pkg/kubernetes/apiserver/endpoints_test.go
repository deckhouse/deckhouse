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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func createAPIServerPod(
	t *testing.T,
	kubeCl *client.KubernetesClient,
	name, nodeName, ip string,
	ready corev1.ConditionStatus,
	terminating bool,
) {
	t.Helper()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": "kube-apiserver",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{NodeName: nodeName},
		Status: corev1.PodStatus{
			PodIP: ip,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: ready},
			},
		},
	}
	if terminating {
		pod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}

	_, err := kubeCl.CoreV1().Pods("kube-system").Create(t.Context(), pod, metav1.CreateOptions{})
	require.NoError(t, err)
}

// createDecoyPods adds Ready Pods that must not count as kube-apiserver Pods:
// an etcd Pod in kube-system and an apiserver-labelled Pod in another namespace.
func createDecoyPods(t *testing.T, kubeCl *client.KubernetesClient) {
	t.Helper()

	for _, pod := range []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "etcd-master-etcd-only",
				Namespace: "kube-system",
				Labels:    map[string]string{"component": "etcd", "tier": "control-plane"},
			},
			Spec:   corev1.PodSpec{NodeName: "master-etcd-only"},
			Status: corev1.PodStatus{PodIP: "10.0.0.30"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-master-other-ns",
				Namespace: "default",
				Labels:    map[string]string{"component": "kube-apiserver", "tier": "control-plane"},
			},
			Spec:   corev1.PodSpec{NodeName: "master-other-ns"},
			Status: corev1.PodStatus{PodIP: "10.0.0.31"},
		},
	} {
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		_, err := kubeCl.CoreV1().Pods(pod.Namespace).Create(t.Context(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}
}

func createSlice(
	t *testing.T,
	kubeCl *client.KubernetesClient,
	ports []discoveryv1.EndpointPort,
	endpoints ...discoveryv1.Endpoint,
) {
	t.Helper()

	_, err := kubeCl.DiscoveryV1().EndpointSlices("default").Create(
		t.Context(),
		&discoveryv1.EndpointSlice{
			ObjectMeta:  metav1.ObjectMeta{Name: "kubernetes", Namespace: "default"},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   endpoints,
			Ports:       ports,
		},
		metav1.CreateOptions{},
	)
	require.NoError(t, err)
}

func httpsPort() []discoveryv1.EndpointPort {
	name := "https"
	port := int32(6443)
	return []discoveryv1.EndpointPort{{Name: &name, Port: &port}}
}

func TestGetEndpoints(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	createAPIServerPod(t, kubeCl, "kube-apiserver-master-1", "master-1", "10.0.0.9", corev1.ConditionFalse, false)
	createAPIServerPod(t, kubeCl, "kube-apiserver-terminating", "master-2", "10.0.0.7", corev1.ConditionTrue, true)
	createAPIServerPod(t, kubeCl, "kube-apiserver-no-ip", "master-3", "", corev1.ConditionTrue, false)
	createAPIServerPod(t, kubeCl, "kube-apiserver-v6", "master-4", "fd00::1", corev1.ConditionTrue, false)
	createDecoyPods(t, kubeCl)

	otherName := "metrics"
	otherPort := int32(9443)
	createSlice(
		t,
		kubeCl,
		append(httpsPort(),
			discoveryv1.EndpointPort{Port: &otherPort},
			discoveryv1.EndpointPort{Name: &otherName, Port: &otherPort},
			discoveryv1.EndpointPort{Name: &otherName},
		),
		discoveryv1.Endpoint{Addresses: []string{"10.0.0.1"}},
		discoveryv1.Endpoint{Addresses: []string{"10.0.0.2"}},
	)

	endpoints, err := GetEndpoints(t.Context(), kubeCl)
	require.NoError(t, err)
	require.Equal(
		t,
		[]string{
			"10.0.0.1:6443",
			"10.0.0.2:6443",
			"10.0.0.9:6443",
			"[fd00::1]:6443",
		},
		endpoints,
	)
}

func TestGetEndpointsWithoutSlice(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	createAPIServerPod(t, kubeCl, "kube-apiserver-master-1", "master-1", "10.0.0.2", corev1.ConditionTrue, false)

	_, err := GetEndpoints(t.Context(), kubeCl)
	require.ErrorContains(t, err, "read the default/kubernetes EndpointSlice")
}

func TestGetReadyHostsForNodes(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	createAPIServerPod(t, kubeCl, "kube-apiserver-master-1", "master-1", "10.0.0.2", corev1.ConditionTrue, false)
	createAPIServerPod(t, kubeCl, "kube-apiserver-master-2", "master-2", "fd00::3", corev1.ConditionTrue, false)
	createAPIServerPod(t, kubeCl, "kube-apiserver-master-3", "master-3", "10.0.0.4", corev1.ConditionTrue, false)
	createAPIServerPod(t, kubeCl, "kube-apiserver-not-ready", "master-not-ready", "10.0.0.5", corev1.ConditionFalse, false)
	createAPIServerPod(t, kubeCl, "kube-apiserver-terminating", "master-terminating", "10.0.0.6", corev1.ConditionTrue, true)
	createAPIServerPod(t, kubeCl, "kube-apiserver-no-ip", "master-no-ip", "", corev1.ConditionTrue, false)
	createDecoyPods(t, kubeCl)

	// The slice still lists the NotReady apiserver as ready: readiness must
	// come from the Pod, not from the slice.
	ready := true
	createSlice(
		t,
		kubeCl,
		httpsPort(),
		discoveryv1.Endpoint{Addresses: []string{"10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
		discoveryv1.Endpoint{Addresses: []string{"10.0.0.5"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
		discoveryv1.Endpoint{Addresses: []string{"10.0.0.6"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
	)

	hosts, err := GetReadyHostsForNodes(t.Context(), kubeCl, []string{"master-2", "master-1"})
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.2", "fd00::3"}, hosts)

	for _, nodeName := range []string{
		"master-not-ready",
		"master-terminating",
		"master-no-ip",
		"master-etcd-only",
		"master-other-ns",
		"master-unknown",
	} {
		_, err = GetReadyHostsForNodes(t.Context(), kubeCl, []string{"master-1", nodeName})
		require.ErrorContains(t, err, `control-plane node "`+nodeName+`" has no ready API server endpoint`)
	}

	_, err = GetReadyHostsForNodes(t.Context(), kubeCl, nil)
	require.ErrorContains(t, err, "no control-plane nodes specified")
}
