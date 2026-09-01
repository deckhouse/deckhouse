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

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestGetEndpoints(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	_, err := kubeCl.CoreV1().Pods("kube-system").Create(
		t.Context(),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-master-1",
				Namespace: "kube-system",
				Labels: map[string]string{
					"component": "kube-apiserver",
					"tier":      "control-plane",
				},
			},
			Status: corev1.PodStatus{
				PodIP: "10.0.0.2",
			},
		},
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	portName := "https"
	port := int32(6443)

	_, err = kubeCl.DiscoveryV1().
		EndpointSlices("default").
		Create(
			t.Context(),
			&discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubernetes",
					Namespace: "default",
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{Addresses: []string{"10.0.0.1"}},
					{Addresses: []string{"10.0.0.2"}},
				},
				Ports: []discoveryv1.EndpointPort{
					{Name: &portName, Port: &port},
				},
			},
			metav1.CreateOptions{},
		)
	require.NoError(t, err)

	endpoints, err := GetEndpoints(t.Context(), kubeCl)
	require.NoError(t, err)
	require.Equal(
		t,
		[]string{
			"10.0.0.1:6443",
			"10.0.0.2:6443",
		},
		endpoints,
	)
}

func TestGetReadyEndpoints(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	createPod := func(
		name string,
		ip string,
		ready corev1.ConditionStatus,
	) {
		t.Helper()

		_, err := kubeCl.CoreV1().
			Pods("kube-system").
			Create(
				t.Context(),
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: "kube-system",
						Labels: map[string]string{
							"component": "kube-apiserver",
							"tier":      "control-plane",
						},
					},
					Status: corev1.PodStatus{
						PodIP: ip,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: ready,
							},
						},
					},
				},
				metav1.CreateOptions{},
			)
		require.NoError(t, err)
	}

	createPod(
		"kube-apiserver-ready",
		"10.0.0.1",
		corev1.ConditionTrue,
	)
	createPod(
		"kube-apiserver-not-ready",
		"10.0.0.2",
		corev1.ConditionFalse,
	)

	portName := "https"
	port := int32(6443)
	ready := true
	notReady := false
	terminating := true

	_, err := kubeCl.DiscoveryV1().
		EndpointSlices("default").
		Create(
			t.Context(),
			&discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubernetes",
					Namespace: "default",
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.3"},
						Conditions: discoveryv1.EndpointConditions{
							Ready: &ready,
						},
					},
					{
						Addresses: []string{"10.0.0.4"},
						Conditions: discoveryv1.EndpointConditions{
							Ready: &notReady,
						},
					},
					{
						// Ready=nil means that readiness is unknown.
						// EndpointSlice consumers treat it as ready.
						Addresses: []string{"10.0.0.5"},
					},
					{
						Addresses: []string{"10.0.0.6"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       &ready,
							Terminating: &terminating,
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{Name: &portName, Port: &port},
				},
			},
			metav1.CreateOptions{},
		)
	require.NoError(t, err)

	endpoints, err := GetReadyEndpoints(t.Context(), kubeCl)
	require.NoError(t, err)
	require.Equal(
		t,
		[]string{
			"10.0.0.1:6443",
			"10.0.0.3:6443",
			"10.0.0.5:6443",
		},
		endpoints,
	)
}

func TestGetReadyHostsForNodes(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	createNode := func(name, internalIP string) {
		t.Helper()

		_, err := kubeCl.CoreV1().Nodes().Create(
			t.Context(),
			&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: internalIP,
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
		require.NoError(t, err)
	}

	createReadyPod := func(name, nodeName, ip string) {
		t.Helper()

		_, err := kubeCl.CoreV1().
			Pods("kube-system").
			Create(
				t.Context(),
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: "kube-system",
						Labels: map[string]string{
							"component": "kube-apiserver",
							"tier":      "control-plane",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: nodeName,
					},
					Status: corev1.PodStatus{
						PodIP: ip,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				metav1.CreateOptions{},
			)
		require.NoError(t, err)
	}

	createNode("master-0", "10.0.0.1")
	createNode("master-1", "10.0.0.2")
	createNode("master-2", "10.0.0.3")

	createReadyPod(
		"kube-apiserver-master-1",
		"master-1",
		"10.0.0.2",
	)
	createReadyPod(
		"kube-apiserver-master-2",
		"master-2",
		"10.0.0.3",
	)

	portName := "https"
	port := int32(6443)
	ready := true

	_, err := kubeCl.DiscoveryV1().
		EndpointSlices("default").
		Create(
			t.Context(),
			&discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubernetes",
					Namespace: "default",
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.2"},
						Conditions: discoveryv1.EndpointConditions{
							Ready: &ready,
						},
					},
					{
						Addresses: []string{"10.0.0.3"},
						Conditions: discoveryv1.EndpointConditions{
							Ready: &ready,
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{Name: &portName, Port: &port},
				},
			},
			metav1.CreateOptions{},
		)
	require.NoError(t, err)

	hosts, err := GetReadyHostsForNodes(
		t.Context(),
		kubeCl,
		[]string{"master-2", "master-1"},
	)
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.2", "10.0.0.3"}, hosts)

	_, err = GetReadyHostsForNodes(
		t.Context(),
		kubeCl,
		[]string{"master-0"},
	)
	require.ErrorContains(
		t,
		err,
		`control-plane node "master-0" has no ready API server endpoint`,
	)
}
