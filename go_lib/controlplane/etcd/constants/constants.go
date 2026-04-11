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

package constants

import "time"

const (
	// Etcd defines variable used internally when referring to etcd component
	Etcd = "etcd"

	// KubernetesDir is the directory Kubernetes owns for storing various configuration files
	KubernetesDir = "/etc/kubernetes"

	// EtcdCACertName defines etcd's CA certificate name
	EtcdCACertName = "etcd/ca.crt"

	// EtcdListenClientPort defines the port etcd listen on for client traffic
	EtcdListenClientPort = 2379

	// EtcdListenPeerPort defines the port etcd listen on for peer traffic
	EtcdListenPeerPort = 2380

	// EtcdHealthcheckClientCertName defines etcd's healthcheck client certificate name
	EtcdHealthcheckClientCertName = "etcd/healthcheck-client.crt"
	// EtcdHealthcheckClientKeyName defines etcd's healthcheck client key name
	EtcdHealthcheckClientKeyName = "etcd/healthcheck-client.key"

	EtcdHealthyCheckInterval = 5 * time.Second
	EtcdHealthyCheckRetries  = 8

	// EtcdAdvertiseClientUrlsAnnotationKey is the annotation key on every etcd pod, describing the
	// advertise client URLs
	EtcdAdvertiseClientUrlsAnnotationKey = "control-plane-manager.deckhouse.io/etcd.advertise-client-urls"

	// LegacyEtcdAdvertiseClientUrlsAnnotationKey is the kubeadm-era annotation key.
	// Used as fallback during migration from kubeadm-managed to CPM-managed manifests.
	LegacyEtcdAdvertiseClientUrlsAnnotationKey = "kubeadm.kubernetes.io/etcd.advertise-client-urls"

	// EtcdAPICallTimeout specifies how much time to wait for completion of requests against the etcd API.
	EtcdAPICallTimeout = 2 * time.Minute

	// EtcdAPICallRetryInterval defines how long etcd should wait before retrying a failed API operation
	EtcdAPICallRetryInterval = 500 * time.Millisecond

	// KubernetesAPICallRetryInterval defines how long kubeadm should wait before retrying a failed API operation
	KubernetesAPICallRetryInterval = 500 * time.Millisecond

	// ControlPlaneTier is the value used in the tier label to identify control plane components
	ControlPlaneTier = "control-plane"

	// AdminKubeConfigFileName defines name for the kubeconfig aimed to be used by the admin of the cluster
	AdminKubeConfigFileName = "admin.conf"
)

// KubernetesAPICallTimeout is the maximum time to wait for a Kubernetes API call to complete.
// Declared as a variable (not const) so tests can override it with a shorter duration.
var KubernetesAPICallTimeout = 5 * time.Minute
