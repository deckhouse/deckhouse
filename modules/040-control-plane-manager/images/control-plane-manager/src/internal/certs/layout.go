package certs

import (
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
)

func VirtualCertsFileLayout() map[string]string {
	return pki.FileLayout(
		pki.WithExcludedRootCertificates(pki.EtcdCACertBaseName),
		pki.WithExcludedLeafCertificates(
			pki.EtcdHealthcheckClientCertBaseName,
			pki.EtcdPeerCertBaseName,
			pki.EtcdServerCertBaseName,
		),
	)
}
