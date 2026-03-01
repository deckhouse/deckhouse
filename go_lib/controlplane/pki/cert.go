package pki

import (
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	certutil "k8s.io/client-go/util/cert"
)

// CertConfig is a wrapper around certutil.Config extending it with EncryptionAlgorithm.
type CertConfig struct {
	certutil.Config
	NotAfter            time.Time
	EncryptionAlgorithm constants.EncryptionAlgorithmType
}
