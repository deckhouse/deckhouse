package constants

import (
	"path/filepath"
	"time"
)

const (
	// EtcdAPICallRetryInterval defines how long etcd should wait before retrying a failed API operation
	EtcdAPICallRetryInterval = 500 * time.Millisecond

	// KubernetesAPICallRetryInterval defines how long kubeadm should wait before retrying a failed API operation
	KubernetesAPICallRetryInterval = 500 * time.Millisecond

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

	// EtcdAdvertiseClientUrlsAnnotationKey is the annotation key on every etcd pod, describing the
	// advertise client URLs
	// CHANGE TO control-plane-manager.deckhouse.io/etcd.advertise-client-urls
	EtcdAdvertiseClientUrlsAnnotationKey = "kubeadm.kubernetes.io/etcd.advertise-client-urls"

	// ControlPlaneTier is the value used in the tier label to identify control plane components
	ControlPlaneTier = "control-plane"

	// AdminKubeConfigFileName defines name for the kubeconfig aimed to be used by the admin of the cluster
	AdminKubeConfigFileName = "admin.conf"
)

// GetStaticPodFilepath returns the location on the disk where the Static Pod should be present
func GetStaticPodFilepath(componentName, manifestsDir string) string {
	return filepath.Join(manifestsDir, componentName+".yaml")
}
