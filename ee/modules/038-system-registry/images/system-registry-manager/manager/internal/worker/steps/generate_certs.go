/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/certificate"
	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_files "system-registry-manager/pkg/files"

	log "github.com/sirupsen/logrus"
)

func GenerateCerts(manifestsSpec *pkg_cfg.ManifestsSpec) error {
	log.Info("Starting certificate generation...")

	for _, certSpec := range manifestsSpec.GeneratedCertificates {
		err := generateCertToWorkspace(&certSpec)
		if err != nil {
			return fmt.Errorf("Error generating certificate: %v", err)
		}
	}

	log.Info("Certificate generation completed successfully.")
	return nil
}

func generateCertToWorkspace(genCertSpec *pkg_cfg.GeneratedCertificateSpec) error {
	// Load the CA cert and key content from file
	caCert, err := os.ReadFile(genCertSpec.CACert.TmpPath)
	if err != nil {
		return fmt.Errorf("error reading CA certificate: %v", err)
	}

	caKey, err := os.ReadFile(genCertSpec.CAKey.TmpPath)
	if err != nil {
		return fmt.Errorf("error reading CA private key: %v", err)
	}

	ca := certificate.Authority{
		Key:  string(caKey),
		Cert: string(caCert),
	}

	// Generate cert
	clientCert, err := certificate.GenerateSelfSignedCert(
		log.NewEntry(log.New()),
		genCertSpec.CN,
		ca,
		genCertSpec.Options...,
	)
	if err != nil {
		return fmt.Errorf("error generating client certificate: %v", err)
	}

	// Save cert and key
	err = pkg_files.WriteFile(genCertSpec.Cert.TmpGeneratePath, []byte(clientCert.Cert), 0600)
	if err != nil {
		return fmt.Errorf("error writing certificate to %s: %v", genCertSpec.Cert.TmpGeneratePath, err)
	}

	err = pkg_files.WriteFile(genCertSpec.Key.TmpGeneratePath, []byte(clientCert.Key), 0600)
	if err != nil {
		return fmt.Errorf("error writing private key to %s: %v", genCertSpec.Key.TmpGeneratePath, err)
	}
	return nil
}
