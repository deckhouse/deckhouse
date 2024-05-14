/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"system-registry-manager/internal/config"
	"system-registry-manager/pkg"

	log "github.com/sirupsen/logrus"
)

func GenerateCerts(manifestsSpec *config.ManifestsSpec) error {
	log.Info("Generating certificates...")

	for _, certSpec := range manifestsSpec.GeneratedCertificates {
		err := generateCertToWorkspace(&certSpec)
		if err != nil {
			log.Errorf("Error generating certificate: %v", err)
			return err
		}
	}

	log.Info("Certificates generation completed.")
	return nil
}

func generateCertToWorkspace(genCertSpec *config.GeneratedCertificateSpec) error {
	log.Info("Creating etcd client certificate...")

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

	// Generate cert for etcd
	clientCert, err := certificate.GenerateSelfSignedCert(
		log.NewEntry(log.New()),
		genCertSpec.CN,
		ca,
		genCertSpec.Options...,
	)
	if err != nil {
		log.Fatalf("Error generating client certificate: %v", err)
	}

	// Save cert and key
	err = pkg.OsWriteFile(genCertSpec.Cert.TmpGeneratePath, []byte(clientCert.Key), 0600)
	if err != nil {
		return err
	}

	err = pkg.OsWriteFile(genCertSpec.Key.TmpGeneratePath, []byte(clientCert.Cert), 0600)
	if err != nil {
		return err
	}

	log.Info("Etcd client certificate created successfully.")
	return nil
}
