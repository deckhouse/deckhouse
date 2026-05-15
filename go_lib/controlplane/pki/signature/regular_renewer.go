/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package signature

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/go-jose/go-jose/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

type CertKeyType string

const (
	SignaturePublicJWKS             = "signature-public.jwks"
	SignaturePrivateJWK             = "signature-private.jwk"
	CertKeyTypeRSA      CertKeyType = "RSA"
	CertKeyTypeED25519  CertKeyType = "ED25519"
	// ApiserverRequestTimeout is a timeout for apiserver requests
	ApiserverRequestTimeout = 5 * time.Minute
	// TLSCertificateOField is an "Organization" of the generated self-signed certificates.
	TLSCertificateOField = "signature"
	// TLSCertificateOUField is an "Organization Unit" of the generated self-signed certifiates.
	TLSCertificateOUField = "apiserver"
	// DefaultCertificateTTL is the number of days generated certificates will be considered valid.
	DefaultCertificateTTL = 365
)

type RegularRenewer struct {
	kubernetesPkiPath string
	leftDaysToRenew   int
}

func NewRegularSignatureRenewer(kubernetesPkiPath string) *RegularRenewer {
	return &RegularRenewer{
		kubernetesPkiPath: kubernetesPkiPath,
		leftDaysToRenew:   60,
	}
}

func (s *RegularRenewer) WithLeftDaysToRenew(days int) *RegularRenewer {
	if days > 0 {
		s.leftDaysToRenew = days
	}

	return s
}

func (s *RegularRenewer) Renew(k8sInterface kubernetes.Interface) error {
	pkiDir := s.kubernetesPkiPath
	logger.Info("renew signature certs", slog.String("pki_dir", pkiDir))
	jwkPath := filepath.Join(pkiDir, SignaturePrivateJWK)
	jwksPath := filepath.Join(pkiDir, SignaturePublicJWKS)

	ctx, cancel := context.WithTimeout(context.Background(), ApiserverRequestTimeout)
	defer cancel()

	secret, err := k8sInterface.CoreV1().Secrets("kube-system").Get(ctx, "d8-pki", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	var privJWK jose.JSONWebKey
	var jwks jose.JSONWebKeySet

	jwkData := secret.Data["signature-private"]
	jwksData := secret.Data["signature-public"]
	isSecretRecordsExists := len(jwkData) > 0 && len(jwksData) > 0

	if isSecretRecordsExists {
		if err := json.Unmarshal(jwkData, &privJWK); err != nil {
			return fmt.Errorf("invalid JWK in secret: %w", err)
		}
		if err := json.Unmarshal(jwksData, &jwks); err != nil {
			return fmt.Errorf("invalid JWKS in secret: %w", err)
		}
	}

	fileJwk, errPriv := os.ReadFile(jwkPath)
	fileJwks, errPub := os.ReadFile(jwksPath)
	isFilesExists := errPriv == nil && errPub == nil

	if !isSecretRecordsExists && !isFilesExists {
		logger.Info("no signature records in secret d8-pki, and files exists, generating new")
		return generateNewSignatureCerts(pkiDir, k8sInterface, true)
	}

	if isFilesExists && !isSecretRecordsExists {
		logger.Info("no signature records in secret d8-pki, but files exists, syncing to secret")
		secret.Data["signature-private"] = fileJwk
		secret.Data["signature-public"] = fileJwks
		if _, err = k8sInterface.CoreV1().Secrets("kube-system").Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update secret: %w", err)
		}
		if err := json.Unmarshal(fileJwk, &privJWK); err != nil {
			return fmt.Errorf("invalid JWK in secret: %w", err)
		}
		if err := json.Unmarshal(fileJwks, &jwks); err != nil {
			return fmt.Errorf("invalid JWKS in secret: %w", err)
		}

	}
	if isSecretRecordsExists && (!isFilesExists || !bytes.Equal(jwkData, fileJwk) || !bytes.Equal(jwksData, fileJwks)) {
		logger.Info("files missing or mismatch with secret, syncing to disk")
		logger.Info("check if secret is valid")
		if err := replaceSignatureFiles(pkiDir, jwkData, jwksData); err != nil {
			return fmt.Errorf("failed to sync files to disk: %w", err)
		}
	}

	var targetCert *x509.Certificate
	for _, key := range jwks.Keys {
		if key.KeyID == privJWK.KeyID && len(key.Certificates) > 0 {
			targetCert = key.Certificates[0]
			break
		}
	}

	if targetCert == nil {
		return fmt.Errorf("certificate with KeyID %s not found in JWKS", privJWK.KeyID)
	}

	daysHours := s.leftDaysToRenew * 24 * int(time.Hour)

	if pkiutil.CertificateExpiresSoon(targetCert, time.Duration(daysHours)) {
		logger.Info("certificate is expiring soon, renewing...")
		return generateNewSignatureCerts(pkiDir, k8sInterface, false)
	}

	logger.Info("signature files up-to-date")
	return nil
}

func (s *RegularRenewer) APIServerChecksumPaths() []string {
	return []string{
		filepath.Join(s.kubernetesPkiPath, SignaturePrivateJWK),
		filepath.Join(s.kubernetesPkiPath, SignaturePublicJWKS),
	}
}

func generateNewSignatureCerts(filePath string, k8sInterface kubernetes.Interface, isInitial bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), ApiserverRequestTimeout)
	defer cancel()

	JWKSmaxSize := 5
	logger.Info("generate new signature certs")
	keys, err := generateTLSCertificate(
		TLSCertificateOField,
		TLSCertificateOUField,
		CertKeyTypeED25519,
		DefaultCertificateTTL,
	)
	if err != nil {
		return err
	}

	dateNow := time.Now().Format("2006-01-02 15:04")

	privJWK := jose.JSONWebKey{
		Key:       keys.PrivateKey,
		KeyID:     dateNow,
		Algorithm: string(jose.EdDSA),
		Use:       "sig",
	}
	jwkJSON, err := json.Marshal(privJWK)
	if err != nil {
		return err
	}

	// Create the public JWK from the certificate
	var pubJWK jose.JSONWebKey

	if len(keys.Certificate) == 0 {
		return fmt.Errorf("no certificates found")
	}

	cert, err := x509.ParseCertificate(keys.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}
	pubJWK = jose.JSONWebKey{
		Key:          cert.PublicKey,
		KeyID:        dateNow,
		Algorithm:    string(jose.EdDSA),
		Use:          "sig",
		Certificates: []*x509.Certificate{cert},
	}
	var keySet jose.JSONWebKeySet

	if !isInitial {
		jwksData, err := os.ReadFile(filepath.Join(filePath, SignaturePublicJWKS))
		if err == nil {
			if err := json.Unmarshal(jwksData, &keySet); err != nil {
				return fmt.Errorf("failed to parse existing JWKS: %w", err)
			}
		}
	}
	keySet.Keys = append(keySet.Keys, pubJWK)

	// Trim the slice if it exceeds the defined maximum size
	if len(keySet.Keys) > JWKSmaxSize {
		logger.Info("signature JWKS size is too big, trimming oldest keys")
		startIdx := len(keySet.Keys) - JWKSmaxSize
		newKeys := make([]jose.JSONWebKey, JWKSmaxSize)
		copy(newKeys, keySet.Keys[startIdx:])
		keySet.Keys = newKeys
	}
	jwksJSON, err := json.MarshalIndent(keySet, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JWKS: %w", err)
	}
	// write data to secret ns kube-system d8-pki
	logger.Info("write data to secret d8-pki")
	secret, err := k8sInterface.CoreV1().Secrets("kube-system").Get(ctx, "d8-pki", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}
	secret.Data["signature-private"] = jwkJSON
	secret.Data["signature-public"] = jwksJSON
	_, err = k8sInterface.CoreV1().Secrets("kube-system").Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}
	replaceSignatureFiles(filePath, jwkJSON, jwksJSON)
	return nil
}

func replaceSignatureFiles(filePath string, jwkData, jwksData []byte) error {
	logger.Info("replace signature files on disk")
	if err := os.WriteFile(filepath.Join(filePath, SignaturePublicJWKS), jwksData, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(filePath, SignaturePrivateJWK), jwkData, 0o600); err != nil {
		return err
	}
	return nil
}
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func generateTLSCertificate(serviceName, clusterDomain string, keyType CertKeyType, durationInDays int) (*tls.Certificate, error) {
	now := time.Now()
	subjectKeyId := make([]byte, 10)
	rand.Read(subjectKeyId)
	commonName := fmt.Sprintf("%s.%s", serviceName, clusterDomain)
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			CommonName:         commonName,
			Country:            []string{"Unknown"},
			Organization:       []string{clusterDomain},
			OrganizationalUnit: []string{serviceName},
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(0, 0, durationInDays),
		SubjectKeyId:          subjectKeyId,
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	var (
		priv interface{}
		pub  interface{}
		err  error
	)
	switch keyType {
	case CertKeyTypeRSA:
		priv, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA private key: %w", err)
		}
		pub = priv.(*rsa.PrivateKey).Public()
	case CertKeyTypeED25519:
		pub, priv, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate Ed25519 private key: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported key type: %s (must be CertKeyRSA or CertKeyED25519)", keyType)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, pub, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}
	tlsCert := &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}
	return tlsCert, nil
}
