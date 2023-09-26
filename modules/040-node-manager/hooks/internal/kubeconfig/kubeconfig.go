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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd/api"
)

// New creates a new Kubeconfig using the cluster name and specified endpoint.
func New(clusterName, endpoint string, caCert []byte, token string) (*api.Config, error) {
	userName := fmt.Sprintf("%s-admin", clusterName)
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
				Token: token,
			},
		},
		CurrentContext: contextName,
	}, nil
}

// GenerateSecret returns a Kubernetes secret for the given Cluster and kubeconfig data.
func GenerateSecret(cluster *unstructured.Unstructured, data []byte) *corev1.Secret {
	return GenerateSecretWithOwner(cluster, data, metav1.OwnerReference{
		APIVersion: cluster.GetAPIVersion(),
		Kind:       "Cluster",
		Name:       cluster.GetName(),
		UID:        cluster.GetUID(),
	})
}

// GenerateSecretWithOwner returns a Kubernetes secret for the given Cluster name, namespace, kubeconfig data, and ownerReference.
func GenerateSecretWithOwner(cluster *unstructured.Unstructured, data []byte, owner metav1.OwnerReference) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kubeconfig", cluster.GetName()),
			Namespace: cluster.GetNamespace(),
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": cluster.GetName(),
			},
			OwnerReferences: []metav1.OwnerReference{
				owner,
			},
		},
		Data: map[string][]byte{
			"value": data,
		},
		Type: "cluster.x-k8s.io/secret",
	}
}

func GenerateSecretForServiceAccountToken(cluster *unstructured.Unstructured, serviceAccountName string) *corev1.Secret {
	return generateSecretForServiceAccountToken(cluster, serviceAccountName, metav1.OwnerReference{
		APIVersion: cluster.GetAPIVersion(),
		Kind:       "Cluster",
		Name:       cluster.GetName(),
		UID:        cluster.GetUID(),
	})
}

func generateSecretForServiceAccountToken(cluster *unstructured.Unstructured, serviceAccountName string, owner metav1.OwnerReference) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kubeconfig-token", cluster.GetName()),
			Namespace: cluster.GetNamespace(),
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": cluster.GetName(),
			},
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": serviceAccountName,
			},
			OwnerReferences: []metav1.OwnerReference{
				owner,
			},
		},
		Type: "kubernetes.io/service-account-token",
	}
}
