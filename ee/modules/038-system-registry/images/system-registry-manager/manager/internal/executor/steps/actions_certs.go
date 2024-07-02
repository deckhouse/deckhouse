/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"context"
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/certificate"
	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_files "system-registry-manager/pkg/files"
	pkg_logs "system-registry-manager/pkg/logs"
)

func CreateCertBundle(ctx context.Context, generatedCertificateSpec *pkg_cfg.GeneratedCertificateSpec) (*CertBundle, error) {
	log := pkg_logs.GetLoggerFromContext(ctx)

	// Load the CA cert and key content from file
	caCert, err := os.ReadFile(generatedCertificateSpec.CACert.InputPath)
	if err != nil {
		return nil, fmt.Errorf("error reading CA certificate: %v", err)
	}

	caKey, err := os.ReadFile(generatedCertificateSpec.CAKey.InputPath)
	if err != nil {
		return nil, fmt.Errorf("error reading CA private key: %v", err)
	}

	ca := certificate.Authority{
		Key:  string(caKey),
		Cert: string(caCert),
	}

	// Generate cert
	clientCert, err := certificate.GenerateSelfSignedCert(
		log,
		generatedCertificateSpec.CN,
		ca,
		generatedCertificateSpec.Options...,
	)
	if err != nil {
		return nil, fmt.Errorf("error generating client certificate: %v", err)
	}

	certBundle := CertBundle{
		Key: FileBundle{
			DestPath: generatedCertificateSpec.Key.DestPath,
			Content:  clientCert.Key,
		},
		Cert: FileBundle{
			DestPath: generatedCertificateSpec.Cert.DestPath,
			Content:  clientCert.Cert,
		},
		Check: FileCheck{},
	}
	return &certBundle, nil
}

func CheckCertDest(ctx context.Context, certBundle *CertBundle, params *InputParams) error {
	if !params.Certs.UpdateOrCreate {
		return nil
	}

	// Check Cert
	if !pkg_files.IsPathExists(certBundle.Cert.DestPath) {
		certBundle.Check.NeedCreate = true
		return nil
	}
	// Uncomment and implement checksum comparison if needed
	// checkSumEq, err := pkg_files.CompareChecksumByDestFilePath(certBundle.Cert.Content, certBundle.Cert.DestPath)
	// if err != nil {
	// 	return fmt.Errorf("error comparing checksums for file %s: %v", certBundle.Cert.DestPath, err)
	// }
	// certBundle.Check.NeedUpdate = !checkSumEq

	// Check Key
	if !pkg_files.IsPathExists(certBundle.Key.DestPath) {
		certBundle.Check.NeedCreate = true
		return nil
	}
	// Uncomment and implement checksum comparison if needed
	// checkSumEq, err := pkg_files.CompareChecksumByDestFilePath(certBundle.Key.Content, certBundle.Key.DestPath)
	// if err != nil {
	// 	return fmt.Errorf("error comparing checksums for file %s: %v", certBundle.Key.DestPath, err)
	// }
	// certBundle.Check.NeedUpdate = !checkSumEq

	return nil
}

func UpdateCertDest(ctx context.Context, certBundle *CertBundle) error {
	if certBundle.Check.NeedCreateOrUpdate() {
		if err := pkg_files.WriteFile(certBundle.Cert.DestPath, []byte(certBundle.Cert.Content), 0600); err != nil {
			return fmt.Errorf("error writing cert to %s: %v", certBundle.Cert.DestPath, err)
		}

		if err := pkg_files.WriteFile(certBundle.Key.DestPath, []byte(certBundle.Key.Content), 0600); err != nil {
			return fmt.Errorf("error writing cert key to %s: %v", certBundle.Key.DestPath, err)
		}
	}
	return nil
}

func DeleteCertDest(ctx context.Context, generatedCertificateSpec *pkg_cfg.GeneratedCertificateSpec) error {
	if err := pkg_files.DeleteFileIfExist(generatedCertificateSpec.Cert.DestPath); err != nil {
		return fmt.Errorf("error deleting cert from '%s': %w", generatedCertificateSpec.Cert.DestPath, err)
	}

	if err := pkg_files.DeleteFileIfExist(generatedCertificateSpec.Key.DestPath); err != nil {
		return fmt.Errorf("error deleting cert key from '%s': %w", generatedCertificateSpec.Key.DestPath, err)
	}
	return nil
}
