package kubeconfig

import (
	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Options struct {
	OutDir                    string
	ClusterName               string
	CertificatesDir           string
	LocalAPIEndpoint          string
	ControlPlaneEndpoint      string
	NodeNamePath              string
	EncryptionAlgorithm       constants.EncryptionAlgorithmType
	CertificateValidityPeriod *metav1.Duration
	CertProvider              CertProvider
}

func buildOptions(options ...Option) (opt *Options, err error) {
	opt.enrichWithOptions(options...)

	if err = opt.setDefaultsIfNeeded(); err != nil {
		return nil, err
	}

	return opt, nil
}

func (opt *Options) enrichWithOptions(options ...Option) {
	for _, option := range options {
		option(opt)
	}
}

func (opt *Options) setDefaultsIfNeeded() error {
	var err error

	if opt.ClusterName == "" {
		opt.ClusterName = DefaultClusterName
	}

	if opt.CertificatesDir == "" {
		opt.CertificatesDir = DefaultCertificatesDir
	}

	if opt.EncryptionAlgorithm == "" {
		opt.EncryptionAlgorithm = constants.EncryptionAlgorithmRSA2048
	}

	if opt.CertProvider == nil {
		opt.CertProvider, err = newDiskCertProvider(opt.CertificatesDir, opt.CertificateValidityPeriod)

		if err != nil {
			return err
		}
	}

	return nil
}

type Option func(*Options)

// WithClusterName is an option to set the cluster name.
func WithClusterName(clusterName string) Option {
	return func(o *Options) {
		o.ClusterName = clusterName
	}
}
