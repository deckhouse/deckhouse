package etcdconfig

import (
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapi"
)

type ImageMeta struct {
	// ImageRepository sets the container registry to pull images from.
	// if not set, the ImageRepository defined in ClusterConfiguration will be used instead.
	ImageRepository string

	// ImageTag allows to specify a tag for the image.
	// In case this value is set, kubeadm does not change automatically the version of the above components during upgrades.
	ImageTag string

	//TODO: evaluate if we need also a ImageName based on user feedbacks
}

type LocalEtcd struct {
	// // ImageMeta allows to customize the container used for etcd
	ImageMeta `json:",inline"`

	// DataDir is the directory etcd will place its data.
	// Defaults to "/var/lib/etcd".
	DataDir string

	// ExtraArgs are extra arguments provided to the etcd binary
	// when run inside a static pod.
	// An argument name in this list is the flag name as it appears on the
	// command line except without leading dash(es). Extra arguments will override existing
	// default arguments. Duplicate extra arguments are allowed.
	ExtraArgs []kubeadmapi.Arg

	// ExtraEnvs is an extra set of environment variables to pass to the control plane component.
	// Environment variables passed using ExtraEnvs will override any existing environment variables, or *_proxy environment variables that kubeadm adds by default.
	// +optional
	ExtraEnvs []kubeadmapi.EnvVar

	// ServerCertSANs sets extra Subject Alternative Names for the etcd server signing cert.
	ServerCertSANs []string
	// PeerCertSANs sets extra Subject Alternative Names for the etcd peer signing cert.
	PeerCertSANs []string
}

type EtcdConfig struct {
	// StaticPodManifest []byte
	ManifestDir     string
	CertificatesDir string
	LocalEtcd       *LocalEtcd

	// ImageRepository sets the container registry to pull images from.
	// If empty, `registry.k8s.io` will be used by default; in case of kubernetes version is a CI build (kubernetes version starts with `ci/`)
	// `gcr.io/k8s-staging-ci-images` will be used as a default for control plane components and for kube-proxy, while `registry.k8s.io`
	// will be used for all the other images.
	ImageRepository string

	KubernetesVersion string

	Timeouts *kubeadmapi.Timeouts
	// Таймаут ожидания готовности etcd после размещения манифеста
	StartupTimeout time.Duration
}
