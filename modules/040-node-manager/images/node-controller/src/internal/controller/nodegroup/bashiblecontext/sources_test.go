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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, discoveryv1.AddToScheme(scheme))
	return scheme
}

func newService(t *testing.T, objs ...client.Object) *Service {
	t.Helper()
	c := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objs...).Build()
	return &Service{Client: c}
}

func secret(ns, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Data:       data,
	}
}

func configMap(ns, name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Data:       data,
	}
}

func TestReadPackagesProxyToken(t *testing.T) {
	s := newService(t, secret(cloudInstanceManagerNS, packagesProxyTokenSecretName, map[string][]byte{
		"token": []byte("tok-123"),
	}))
	assert.Equal(t, "tok-123", s.readPackagesProxyToken(context.Background()))
}

func TestReadPackagesProxyToken_Absent(t *testing.T) {
	s := newService(t)
	assert.Equal(t, "", s.readPackagesProxyToken(context.Background()))
}

func TestReadControlPlaneArguments(t *testing.T) {
	s := newService(t, secret(kubeSystemNS, controlPlaneArgsSecretName, map[string][]byte{
		"arguments.json":    []byte(`{"nodeMonitorGracePeriod":40}`),
		"featureGates.json": []byte(`{"kubelet":["A","B"]}`),
	}))
	got := s.readControlPlaneArguments(context.Background())
	require.True(t, got.present)
	require.NotNil(t, got.updateFrequency)
	assert.Equal(t, float64(10), *got.updateFrequency) // round(40/4)
	assert.Equal(t, []string{"A", "B"}, got.kubeletFeatureGate)
}

func TestReadControlPlaneArguments_ZeroGracePeriodOmitsFrequency(t *testing.T) {
	s := newService(t, secret(kubeSystemNS, controlPlaneArgsSecretName, map[string][]byte{
		"arguments.json":    []byte(`{"nodeMonitorGracePeriod":0}`),
		"featureGates.json": []byte(`{}`),
	}))
	got := s.readControlPlaneArguments(context.Background())
	require.True(t, got.present)
	assert.Nil(t, got.updateFrequency)
	assert.Equal(t, []string{}, got.kubeletFeatureGate) // nil kubelet -> []
}

func TestReadControlPlaneArguments_Absent(t *testing.T) {
	s := newService(t)
	assert.False(t, s.readControlPlaneArguments(context.Background()).present)
}

func TestReadAPIServerProxyCerts(t *testing.T) {
	s := newService(t, secret(kubeSystemNS, apiProxyCertSecretName, map[string][]byte{
		"crt": []byte("CERT"),
		"key": []byte("KEY"),
	}))
	got := s.readAPIServerProxyCerts(context.Background())
	require.True(t, got.present)
	assert.Equal(t, "CERT", got.crt)
	assert.Equal(t, "KEY", got.key)
}

func TestReadKubernetesCA(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(path, []byte("CA-PEM"), 0o600))
	s := &Service{RootCAFile: path}
	assert.Equal(t, "CA-PEM", s.readKubernetesCA())
}

func TestReadKubernetesCA_MissingFile(t *testing.T) {
	s := &Service{RootCAFile: filepath.Join(t.TempDir(), "nope.crt")}
	assert.Equal(t, "", s.readKubernetesCA())
}

func bootstrapTokenSecret(name, ng, id, sec string, created time.Time, expireIn time.Duration) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         kubeSystemNS,
			Name:              name,
			Labels:            map[string]string{bootstrapTokenNGLabel: ng},
			CreationTimestamp: metav1.NewTime(created),
		},
		Type: corev1.SecretTypeBootstrapToken,
		Data: map[string][]byte{
			"token-id":     []byte(id),
			"token-secret": []byte(sec),
			"expiration":   []byte(time.Now().Add(expireIn).Format(time.RFC3339)),
		},
	}
}

func TestReadBootstrapTokens_NewestPerNG(t *testing.T) {
	now := time.Now()
	s := newService(t,
		bootstrapTokenSecret("bootstrap-token-old", "worker", "aaaaaa", "old", now.Add(-2*time.Hour), 4*time.Hour),
		bootstrapTokenSecret("bootstrap-token-new", "worker", "bbbbbb", "new", now, 4*time.Hour),
		bootstrapTokenSecret("bootstrap-token-exp", "worker", "cccccc", "exp", now.Add(time.Minute), -time.Minute),
		bootstrapTokenSecret("bootstrap-token-sys", "system", "dddddd", "sys", now, 4*time.Hour),
	)
	got := s.readBootstrapTokens(context.Background())
	assert.Equal(t, map[string]string{
		"worker": "bbbbbb.new",
		"system": "dddddd.sys",
	}, got)
}

func endpointSlice(addrs []string, portName string, port int32) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"},
		Endpoints:  []discoveryv1.Endpoint{{Addresses: addrs}},
		Ports: []discoveryv1.EndpointPort{
			{Name: ptr.To(portName), Port: ptr.To(port)},
		},
	}
}

func apiserverPod(name, ip string, ready bool) *corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kubeSystemNS,
			Name:      name,
			Labels:    map[string]string{"component": "kube-apiserver", "tier": "control-plane"},
		},
		Status: corev1.PodStatus{
			PodIP:      ip,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}},
		},
	}
}

func TestReadEndpoints_UnionSortedSplit(t *testing.T) {
	s := newService(t,
		apiserverPod("kube-apiserver-1", "10.0.0.2", true),
		apiserverPod("kube-apiserver-2", "10.0.0.1", true),
		apiserverPod("kube-apiserver-3", "10.0.0.9", false), // not ready -> excluded
		endpointSlice([]string{"10.0.0.1"}, "https", 6443),   // duplicate of pod 2
	)
	got := s.readEndpoints(context.Background())

	assert.Equal(t, []string{"10.0.0.1:6443", "10.0.0.2:6443"}, got.apiserverEndpoints)
	require.Len(t, got.clusterMasterEndpoints, 2)
	assert.Equal(t, map[string]interface{}{
		"address":                "10.0.0.1",
		"kubeApiPort":            6443,
		"rppServerPort":          packagesProxyPort,
		"rppBootstrapServerPort": packagesProxyBootstrapPort,
	}, got.clusterMasterEndpoints[0])
}

func TestReadCloudProvider(t *testing.T) {
	s := newService(t, secret(kubeSystemNS, cloudProviderSecretName, map[string][]byte{
		"type":             []byte(`"yandex"`),
		"machineClassKind": []byte(`"YandexMachineClass"`),
	}))
	got := s.readCloudProvider(context.Background())
	assert.Equal(t, "yandex", got["type"])
	assert.Equal(t, "YandexMachineClass", got["machineClassKind"])
}

func TestReadCloudProvider_AbsentReturnsNil(t *testing.T) {
	s := newService(t)
	assert.Nil(t, s.readCloudProvider(context.Background()))
}
