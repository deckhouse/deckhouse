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

// Package names holds the object names and keys shared by the bashible context
// assembly in go_lib/bashiblecontext and the node-controller code that drives it.
// They live in their own package so neither side has to repeat a literal, and so
// bashiblecontext itself keeps the API surface it had while it was still part of
// the node-controller image.
package names

const (
	// SecretName, SecretNamespace and SecretInputKey identify the assembled bashible
	// context: node-controller writes it and bashible-apiserver reads it.
	SecretName      = "bashible-apiserver-context"
	SecretNamespace = "d8-cloud-instance-manager"
	SecretInputKey  = "input.yaml"

	CloudInstanceManagerNS = "d8-cloud-instance-manager"
	KubeSystemNS           = "kube-system"
	VersionInfoCMNS        = "d8-system"

	APIProxyCertSecretName       = "kubernetes-api-proxy-discovery-cert"
	CloudProviderSecretName      = "d8-node-manager-cloud-provider"
	ClusterConfigSecretName      = "d8-cluster-configuration"
	ControlPlaneArgsSecretName   = "d8-control-plane-manager-control-plane-arguments"
	PackagesProxyTokenSecretName = "registry-packages-proxy-token"

	ClusterUUIDConfigMapName = "d8-cluster-uuid"
	VersionInfoCMName        = "d8-deckhouse-version-info"

	ClusterConfigKey = "cluster-configuration.yaml"
	ClusterUUIDKey   = "cluster-uuid"

	BootstrapTokenNGLabel = "node-manager.deckhouse.io/node-group"
	DNSAppLabel           = "k8s-app"

	KubernetesEndpointSliceNS   = "default"
	KubernetesEndpointSliceName = "kubernetes"
)
