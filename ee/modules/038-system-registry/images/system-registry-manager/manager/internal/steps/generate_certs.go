/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"

	"system-registry-manager/internal/config"
	"system-registry-manager/pkg"

	log "github.com/sirupsen/logrus"
)

func GenerateCerts() error {
	log.Info("Generating certificates...")

	if err := generateSeaweedEtcdClientCert(); err != nil {
		log.Errorf("Error generating SeaweedEtcdClient certificate: %v", err)
		return err
	}
	if err := generateDockerAuthTokenCert(); err != nil {
		log.Errorf("Error generating DockerAuthToken certificate: %v", err)
		return err
	}
	log.Info("Certificates generation completed.")
	return nil
}

func generateSeaweedEtcdClientCert() error {
	cfg := config.GetConfig()
	return createEtcdClientCert(
		cfg.GeneratedCertificates.SeaweedEtcdClientCert.CACert.TmpPath,
		cfg.GeneratedCertificates.SeaweedEtcdClientCert.CAKey.TmpPath,
		cfg.GeneratedCertificates.SeaweedEtcdClientCert.Cert.TmpGeneratePath,
		cfg.GeneratedCertificates.SeaweedEtcdClientCert.Key.TmpGeneratePath,
	)
}

func generateDockerAuthTokenCert() error {
	cfg := config.GetConfig()
	return createSelfSignedCert(
		cfg.GeneratedCertificates.DockerAuthTokenCert.Cert.TmpGeneratePath,
		cfg.GeneratedCertificates.DockerAuthTokenCert.Key.TmpGeneratePath,
	)
}

func createEtcdClientCert(caCertPath, caKeyPath, certPath, keyPath string) error {
	log.Info("Creating etcd client certificate...")

	// Load the CA certificate content from file
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("error reading CA certificate: %v", err)
	}

	// Decode the PEM-encoded certificate
	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return fmt.Errorf("error decoding PEM block of CA certificate")
	}

	// Parse the decoded certificate into x509.Certificate object
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing CA certificate: %v", err)
	}

	// Load the CA private key content from file
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return fmt.Errorf("error reading CA private key: %v", err)
	}

	// Decode the PEM-encoded private key
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return fmt.Errorf("error decoding PEM block of CA private key")
	}

	// Parse the decoded private key into rsa.PrivateKey object
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing CA private key: %v", err)
	}

	// Prepare information for the new certificate
	subject := pkix.Name{
		Organization:       []string{"Example Organization"},
		OrganizationalUnit: []string{"Example Unit"},
		CommonName:         "etcd-client",
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Certificate valid for 1 year

	// Generate a private key for the new certificate
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("error generating private key: %v", err)
	}

	// Create a certificate template for etcd client
	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      subject,
		NotBefore:    notBefore,
		NotAfter:     notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, // For etcd client
		BasicConstraintsValid: true,
	}

	// Create the certificate using the private key and CA certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("error creating certificate: %v", err)
	}

	// Write the certificate to file
	if err := pkg.OsWriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes}), 0644); err != nil {
		return fmt.Errorf("error writing certificate: %v", err)
	}

	// Write the private key to file
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pkg.OsWriteFile(keyPath, pem.EncodeToMemory(privateKeyPEM), 0600); err != nil {
		return fmt.Errorf("error writing private key: %v", err)
	}

	log.Info("Etcd client certificate created successfully.")
	return nil
}

func createSelfSignedCert(certPath, keyPath string) error {
	log.Info("Creating self-signed certificate...")

	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}

	// Create a certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Example Organization"},
			CommonName:   "example.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create the certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %v", err)
	}

	// Write the certificate to file
	if err := pkg.OsWriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes}), 0644); err != nil {
		return fmt.Errorf("error writing certificate: %v", err)
	}

	// Write the private key to file
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pkg.OsWriteFile(keyPath, pem.EncodeToMemory(privateKeyPEM), 0600); err != nil {
		return fmt.Errorf("error writing private key: %v", err)
	}

	log.Info("Self-signed certificate created successfully.")
	return nil
}
