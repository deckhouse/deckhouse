package constants

import (
	"path/filepath"
	"time"

	"github.com/go-errors/errors"
	"k8s.io/apimachinery/pkg/util/version"
)

const (
	EtcdAPICallRetryInterval = 500 * time.Millisecond

	// KubernetesAPICallRetryInterval defines how long kubeadm should wait before retrying a failed API operation
	KubernetesAPICallRetryInterval = 500 * time.Millisecond

	// Etcd defines variable used internally when referring to etcd component
	Etcd = "etcd"

	// KubernetesDir is the directory Kubernetes owns for storing various configuration files
	KubernetesDir = "/etc/kubernetes"
	// ManifestsSubDirName defines directory name to store manifests
	ManifestsSubDirName = "manifests"
	// TempDir defines temporary directory for kubeadm
	// should be joined with KubernetesDir.
	TempDir = "tmp"

	//////////////////////////////////////////////////////////////////////////////////

	// EtcdCACertAndKeyBaseName defines etcd's CA certificate and key base name
	EtcdCACertAndKeyBaseName = "etcd/ca"
	// EtcdCACertName defines etcd's CA certificate name
	EtcdCACertName = "etcd/ca.crt"
	// EtcdCAKeyName defines etcd's CA key name
	EtcdCAKeyName = "etcd/ca.key"

	// EtcdServerCertAndKeyBaseName defines etcd's server certificate and key base name
	EtcdServerCertAndKeyBaseName = "etcd/server"
	// EtcdServerCertName defines etcd's server certificate name
	EtcdServerCertName = "etcd/server.crt"
	// EtcdServerKeyName defines etcd's server key name
	EtcdServerKeyName = "etcd/server.key"

	// EtcdListenClientPort defines the port etcd listen on for client traffic
	EtcdListenClientPort = 2379
	// EtcdMetricsPort is the port at which to obtain etcd metrics and health status
	EtcdMetricsPort = 2381

	// EtcdPeerCertAndKeyBaseName defines etcd's peer certificate and key base name
	EtcdPeerCertAndKeyBaseName = "etcd/peer"
	// EtcdPeerCertName defines etcd's peer certificate name
	EtcdPeerCertName = "etcd/peer.crt"
	// EtcdPeerKeyName defines etcd's peer key name
	EtcdPeerKeyName = "etcd/peer.key"

	// EtcdListenPeerPort defines the port etcd listen on for peer traffic
	EtcdListenPeerPort = 2380

	// EtcdHealthcheckClientCertAndKeyBaseName defines etcd's healthcheck client certificate and key base name
	EtcdHealthcheckClientCertAndKeyBaseName = "etcd/healthcheck-client"
	// EtcdHealthcheckClientCertName defines etcd's healthcheck client certificate name
	EtcdHealthcheckClientCertName = "etcd/healthcheck-client.crt"
	// EtcdHealthcheckClientKeyName defines etcd's healthcheck client key name
	EtcdHealthcheckClientKeyName = "etcd/healthcheck-client.key"
	// EtcdHealthcheckClientCertCommonName defines etcd's healthcheck client certificate common name (CN)
	EtcdHealthcheckClientCertCommonName = "kube-etcd-healthcheck-client"

	// APIServerEtcdClientCertAndKeyBaseName defines apiserver's etcd client certificate and key base name
	APIServerEtcdClientCertAndKeyBaseName = "apiserver-etcd-client"
	// APIServerEtcdClientCertName defines apiserver's etcd client certificate name
	APIServerEtcdClientCertName = "apiserver-etcd-client.crt"
	// APIServerEtcdClientKeyName defines apiserver's etcd client key name
	APIServerEtcdClientKeyName = "apiserver-etcd-client.key"
	// APIServerEtcdClientCertCommonName defines apiserver's etcd client certificate common name (CN)
	APIServerEtcdClientCertCommonName = "kube-apiserver-etcd-client"

	// ProbePort is a general named port to be used in pod manifests.
	ProbePort = "probe-port"

	// DefaultEtcdVersion indicates the default etcd version that kubeadm uses
	DefaultEtcdVersion = "3.6.5-0"

	// EtcdAdvertiseClientUrlsAnnotationKey is the annotation key on every etcd pod, describing the
	// advertise client URLs
	EtcdAdvertiseClientUrlsAnnotationKey = "kubeadm.kubernetes.io/etcd.advertise-client-urls"

	// ControlPlaneTier is the value used in the tier label to identify control plane components
	ControlPlaneTier = "control-plane"

	// ControlPlaneComponentHealthCheckTimeout specifies the default control plane component health check timeout
	ControlPlaneComponentHealthCheckTimeout = 4 * time.Minute

	// AllowExperimentalAPI flag can be used to allow experimental / work in progress APIs
	AllowExperimentalAPI = "allow-experimental-api"

	// InitConfigurationKind is the string kind value for the InitConfiguration struct
	InitConfigurationKind = "InitConfiguration"
)

// SupportedEtcdVersion lists officially supported etcd versions with corresponding Kubernetes releases
var SupportedEtcdVersion = map[uint8]string{
	31: "3.5.24-0",
	32: "3.5.24-0",
	33: "3.5.24-0",
	34: "3.6.5-0",
}

// EtcdSupportedVersion returns officially supported version of etcd for a specific Kubernetes release
// If passed version is not in the given list, the function returns the nearest version with a warning
func EtcdSupportedVersion(supportedEtcdVersion map[uint8]string, versionString string) (etcdVersion *version.Version, warning, err error) {
	kubernetesVersion, err := version.ParseSemantic(versionString)
	if err != nil {
		return nil, nil, err
	}
	desiredVersion, etcdStringVersion := uint8(kubernetesVersion.Minor()), ""

	min, max := ^uint8(0), uint8(0)
	for k, v := range supportedEtcdVersion {
		if desiredVersion == k {
			etcdStringVersion = v
			break
		}
		if k < min {
			min = k
		}
		if k > max {
			max = k
		}
	}

	if len(etcdStringVersion) == 0 {
		if desiredVersion < min {
			etcdStringVersion = supportedEtcdVersion[min]
		}
		if desiredVersion > max {
			etcdStringVersion = supportedEtcdVersion[max]
		}
		warning = errors.Errorf("could not find officially supported version of etcd for Kubernetes %s, falling back to the nearest etcd version (%s)",
			versionString, etcdStringVersion)
	}

	etcdVersion, err = version.ParseSemantic(etcdStringVersion)
	if err != nil {
		return nil, nil, err
	}

	return etcdVersion, warning, nil
}

// GetStaticPodFilepath returns the location on the disk where the Static Pod should be present
func GetStaticPodFilepath(componentName, manifestsDir string) string {
	return filepath.Join(manifestsDir, componentName+".yaml")
}

// GetStaticPodDirectory returns the location on the disk where the Static Pod should be present
func GetStaticPodDirectory() string {
	return filepath.Join(KubernetesDir, ManifestsSubDirName)
}
