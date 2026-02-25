package kubeconfig

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	"github.com/go-errors/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CertProvider interface {
	NotAfter() time.Time
	CACert() *x509.Certificate
	CAKey() crypto.Signer
}

type diskCertProvider struct {
	certStartTime  time.Time
	validityPeriod *metav1.Duration
	caCert         *x509.Certificate
	caKey          crypto.Signer
}

func newDiskCertProvider(certificatesDir string, validityPeriod *metav1.Duration) (*diskCertProvider, error) {
	caCert, caKey, err := pki.TryLoadCertAndKeyFromDisk(certificatesDir, constants.CACertAndKeyBaseName)
	if os.IsNotExist(errors.Unwrap(err)) {
		return nil, fmt.Errorf("the CA files do not exist in %s: %w", certificatesDir, err)
	}
	if err != nil {
		return nil, fmt.Errorf("the CA files couldn't be loaded from %s: %w", certificatesDir, err)
	}

	return &diskCertProvider{
		certStartTime:  time.Now().UTC(),
		validityPeriod: validityPeriod,
		caCert:         caCert,
		caKey:          caKey,
	}, nil
}

func (cp *diskCertProvider) NotAfter() time.Time {
	return cp.certStartTime.Add(cp.validityPeriod.Duration)
}

func (cp *diskCertProvider) CACert() *x509.Certificate {
	return cp.caCert
}

func (cp *diskCertProvider) CAKey() crypto.Signer {
	return cp.caKey
}
