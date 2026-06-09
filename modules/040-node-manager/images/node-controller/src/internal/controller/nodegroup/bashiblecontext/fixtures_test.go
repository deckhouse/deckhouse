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
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/bashiblecontext/names"
)

// The sources the assembly reads. Only the suites that stay with Reconciler and
// Controller use these; go_lib/bashiblecontext carries its own copies for the unit
// tests of the readers themselves, which a test file cannot share across modules.
const (
	clusterConfigSecretName = names.ClusterConfigSecretName
	clusterConfigKey        = names.ClusterConfigKey

	clusterUUIDConfigMapName = names.ClusterUUIDConfigMapName
	clusterUUIDKey           = names.ClusterUUIDKey

	versionInfoCMName = names.VersionInfoCMName

	cloudProviderSecretName      = names.CloudProviderSecretName
	controlPlaneArgsSecretName   = names.ControlPlaneArgsSecretName
	packagesProxyTokenSecretName = names.PackagesProxyTokenSecretName

	bootstrapTokenNGLabel = names.BootstrapTokenNGLabel
	dnsAppLabel           = names.DNSAppLabel
)

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

func endpointSlice(addrs []string, portName string, port int32) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"},
		Endpoints:  []discoveryv1.Endpoint{{Addresses: addrs}},
		Ports: []discoveryv1.EndpointPort{
			{Name: ptr.To(portName), Port: ptr.To(port)},
		},
	}
}

func kubeDNSService(clusterIP string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-dns",
			Namespace: kubeSystemNS,
			Labels:    map[string]string{dnsAppLabel: "kube-dns"},
		},
		Spec: corev1.ServiceSpec{ClusterIP: clusterIP},
	}
}
