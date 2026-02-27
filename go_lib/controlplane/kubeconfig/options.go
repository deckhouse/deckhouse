package kubeconfig

import (
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	"strings"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type options struct {
	// core options
	OutDir                    string
	ClusterName               string
	CertificatesDir           string
	LocalAPIEndpoint          string
	ControlPlaneEndpoint      string
	EncryptionAlgorithm       constants.EncryptionAlgorithmType
	CertificateValidityPeriod *metav1.Duration
	CertProvider              CertProvider

	// specific options
	NodeName string
}

func prepareCoreOptions(opts ...option) (*options, error) {
	var err error
	opt := &options{}

	for _, option := range opts {
		option(opt)
	}

	if opt.OutDir == "" {
		opt.OutDir = DefaultOutDir
	}

	if opt.ClusterName == "" {
		opt.ClusterName = DefaultClusterName
	}

	if opt.CertificatesDir == "" {
		opt.CertificatesDir = DefaultCertificatesDir
	}

	if opt.LocalAPIEndpoint == "" {
		opt.LocalAPIEndpoint = DefaultLocalAPIEndpoint
	}

	if opt.ControlPlaneEndpoint == "" {
		controlPlaneEndpoint, err := os.ReadFile(constants.DiscoveredNodeIPPath)
		if err != nil || len(controlPlaneEndpoint) == 0 {
			return nil, fmt.Errorf("failed to read discovered control plane endpoint: %w", err)
		}

		opt.ControlPlaneEndpoint = fmt.Sprintf("https://%s:6443", strings.TrimSpace(string(controlPlaneEndpoint)))
	}

	if opt.EncryptionAlgorithm == "" {
		opt.EncryptionAlgorithm = constants.EncryptionAlgorithmRSA2048
	}

	if opt.CertProvider == nil {
		opt.CertProvider, err = newDefaultCertProvider(opt.CertificatesDir, opt.CertificateValidityPeriod)

		if err != nil {
			return nil, err
		}
	}

	return opt, nil
}

func (opt *options) ensureNodeNameProvided() error {
	if opt.NodeName == "" {
		name, err := os.ReadFile(constants.DiscoveredNodeNamePath)
		if err != nil || len(name) == 0 {
			return fmt.Errorf("failed to read discovered node name: %w", err)
		}
		opt.NodeName = strings.TrimSpace(string(name))
	}

	return nil
}

type option func(*options)

// WithOutDir is an option to set the output directory.
func WithOutDir(outDir string) option {
	return func(o *options) {
		o.OutDir = outDir
	}
}

// WithClusterName is an option to set the cluster name.
func WithClusterName(clusterName string) option {
	return func(o *options) {
		o.ClusterName = clusterName
	}
}

// WithCertificatesDir is an option to set the certificates directory.
func WithCertificatesDir(certificatesDir string) option {
	return func(o *options) {
		o.CertificatesDir = certificatesDir
	}
}

// WithLocalAPIEndpoint is an option to set the local API endpoint.
func WithLocalAPIEndpoint(localAPIEndpoint string) option {
	return func(o *options) {
		o.LocalAPIEndpoint = localAPIEndpoint
	}
}

// WithControlPlaneEndpoint is an option to set the control plane endpoint.
func WithControlPlaneEndpoint(controlPlaneEndpoint string) option {
	return func(o *options) {
		o.ControlPlaneEndpoint = controlPlaneEndpoint
	}
}

// WithNodeNamePath is an option to set the node name path.
func WithNodeNamePath(nodeName string) option {
	return func(o *options) {
		o.NodeName = nodeName
	}
}

// WithEncryptionAlgorithm is an option to set the encryption algorithm.
func WithEncryptionAlgorithm(encryptionAlgorithm constants.EncryptionAlgorithmType) option {
	return func(o *options) {
		o.EncryptionAlgorithm = encryptionAlgorithm
	}
}

// WithCertificateValidityPeriod is an option to set the certificate validity period.
func WithCertificateValidityPeriod(certificateValidityPeriod *metav1.Duration) option {
	return func(o *options) {
		o.CertificateValidityPeriod = certificateValidityPeriod
	}
}

// WithCertProvider is an option to set the certificate provider.
func WithCertProvider(certProvider CertProvider) option {
	return func(o *options) {
		o.CertProvider = certProvider
	}
}

type CertProvider interface {
	NotAfter() time.Time
	CACert() *x509.Certificate
	CAKey() crypto.Signer
}

type defaultCertProvider struct {
	certStartTime  time.Time
	validityPeriod *metav1.Duration
	caCert         *x509.Certificate
	caKey          crypto.Signer
}

func newDefaultCertProvider(certificatesDir string, validityPeriod *metav1.Duration) (*defaultCertProvider, error) {
	caCert, caKey, err := pki.TryLoadCertAndKeyFromDisk(certificatesDir, constants.CACertAndKeyBaseName)
	if os.IsNotExist(errors.Unwrap(err)) {
		return nil, fmt.Errorf("the CA files do not exist in %s: %w", certificatesDir, err)
	}
	if err != nil {
		return nil, fmt.Errorf("the CA files couldn't be loaded from %s: %w", certificatesDir, err)
	}

	return &defaultCertProvider{
		certStartTime:  time.Now().UTC(),
		validityPeriod: validityPeriod,
		caCert:         caCert,
		caKey:          caKey,
	}, nil
}

func (cp *defaultCertProvider) NotAfter() time.Time {
	return cp.certStartTime.Add(cp.validityPeriod.Duration)
}

func (cp *defaultCertProvider) CACert() *x509.Certificate {
	return cp.caCert
}

func (cp *defaultCertProvider) CAKey() crypto.Signer {
	return cp.caKey
}
