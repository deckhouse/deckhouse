/*
Copyright 2023 Flant JSC

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

package kubeconfig

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"
)

// New creates a new Kubeconfig using the cluster name and specified endpoint.
func New(clusterName, endpoint string, caCert []byte, clientKey []byte, clientCert []byte) (*api.Config, error) {
	userName := "capi-controller-manager"
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)

	return &api.Config{
		Clusters: map[string]*api.Cluster{
			clusterName: {
				Server:                   endpoint,
				CertificateAuthorityData: caCert,
			},
		},
		Contexts: map[string]*api.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			userName: {
				ClientKeyData:         clientKey,
				ClientCertificateData: clientCert,
			},
		},
		CurrentContext: contextName,
	}, nil
}

// GenerateSecret returns a Kubernetes secret for the given Cluster and kubeconfig data.
func GenerateSecret(clusterName string, namespace string, data []byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kubeconfig", clusterName),
			Namespace: namespace,
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": clusterName,
			},
		},
		Data: map[string][]byte{
			"value": data,
		},
		Type: "cluster.x-k8s.io/secret",
	}
}
