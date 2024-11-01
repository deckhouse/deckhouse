/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8s

import (
	"context"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

const (
	RegistryCACert   = "registry-ca.crt"
	RegistryCAKey    = "registry-ca.key"
	AuthCert         = "auth.crt"
	AuthKey          = "auth.key"
	DistributionCert = "distribution.crt"
	DistributionKey  = "distribution.key"
)

type Certificate struct {
	Cert []byte
	Key  []byte
}

// set cfssl global log level to fatal
func init() {
	cfssllog.Level = cfssllog.LevelFatal
}

// GenerateCA generates a new CA certificate and key.
func GenerateCA() (caCertPEM []byte, caKeyPEM []byte, err error) {

	caRequest := &csr.CertificateRequest{
		CN: "embedded-registry-ca",
		CA: &csr.CAConfig{
			Expiry: "87600h", // 10 years
		},
		KeyRequest: &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		},
	}

	caCertPEM, _, caKeyPEM, err = initca.New(caRequest)
	if err != nil {
		return nil, nil, err
	}

	return caCertPEM, caKeyPEM, nil
}

func Validator(req *csr.CertificateRequest) error {
	return nil
}

// GenerateCertificate generates a new certificate and key signed by the provided CA certificate and key.
func GenerateCertificate(commonName string, hosts []string, caCertPEM []byte, caKeyPEM []byte) (certPEM, keyPEM []byte, err error) {

	req := csr.CertificateRequest{
		CN: commonName,
		KeyRequest: &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		},
		Hosts: hosts,
	}

	// generate a CSR and private key
	g := &csr.Generator{Validator: Validator}
	csrPEM, keyPEM, err := g.ProcessRequest(&req)
	if err != nil {
		return nil, nil, err
	}

	// parse CA certificate and key
	caCert, err := helpers.ParseCertificatePEM(caCertPEM)
	if err != nil {
		return nil, nil, err
	}

	caKey, err := helpers.ParsePrivateKeyPEM(caKeyPEM)
	if err != nil {
		return nil, nil, err
	}

	// create a signer
	s, err := local.NewSigner(caKey, caCert, signer.DefaultSigAlgo(caKey), nil)
	if err != nil {
		return nil, nil, err
	}

	// create a sign request
	signReq := signer.SignRequest{
		Request:  string(csrPEM),
		NotAfter: caCert.NotAfter.Add(-1 * time.Hour),
	}

	// sign the certificate
	certPEM, err = s.Sign(signReq)
	if err != nil {
		return nil, nil, err
	}

	return certPEM, keyPEM, nil
}

// EnsureCASecret ensures that the CA secret exists in the cluster. If it does not exist, it generates a new CA certificate and key.
func EnsureCASecret(ctx context.Context, kubeClient *kubernetes.Clientset) (bool, []byte, []byte, error) {

	registryCASecret, err := kubeClient.CoreV1().Secrets(RegistryNamespace).Get(ctx, "registry-pki", metav1.GetOptions{})
	if err == nil {
		caCertPEM := registryCASecret.Data[RegistryCACert]
		caKeyPEM := registryCASecret.Data[RegistryCAKey]
		return false, caCertPEM, caKeyPEM, nil
	}

	if !apierrors.IsNotFound(err) {
		return false, nil, nil, err
	}

	// Secret does not exist, generate a new CA certificate and key
	caCertPEM, caKeyPEM, err := GenerateCA()
	if err != nil {
		return false, nil, nil, err
	}

	// Create the CA secret
	if err := CreateCASecret(ctx, kubeClient, caCertPEM, caKeyPEM); err != nil {
		return false, nil, nil, err
	}

	return true, caCertPEM, caKeyPEM, nil
}

// CreateCASecret creates a new CA secret in the specified namespace
func CreateCASecret(ctx context.Context, kubeClient *kubernetes.Clientset, caCertPEM, caKeyPEM []byte) error {
	labels := map[string]string{
		labelHeritageKey: labelHeritageValue,
		labelModuleKey:   labelModuleValue,
		labelTypeKey:     labelCaSecretTypeValue,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-pki",
			Namespace: RegistryNamespace,
			Labels:    labels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			RegistryCACert: caCertPEM,
			RegistryCAKey:  caKeyPEM,
		},
	}

	// Create the secret in Kubernetes
	_, err := kubeClient.CoreV1().Secrets(RegistryNamespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}
