package certificate

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
)

// ParseCertificatesFromBase64 parsing base64 input string and return ca cert and/or verified tls.Certificate
func ParseCertificatesFromBase64(ca, crt, key string) (*x509.Certificate, *tls.Certificate, error) {
	caCert, err := generateCACert(ca)
	if err != nil {
		return nil, nil, err
	}

	clientCert, err := generateTLSCert(crt, key)
	if err != nil {
		return nil, nil, err
	}

	return caCert, clientCert, nil
}

func generateCACert(caBase64 string) (*x509.Certificate, error) {
	if caBase64 == "" {
		return nil, nil
	}

	caData, err := base64.StdEncoding.DecodeString(caBase64)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(caData)
	if block == nil {
		return nil, fmt.Errorf("block not found")
	}

	if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
		return nil, fmt.Errorf("not valid ca certificate")
	}

	return x509.ParseCertificate(block.Bytes)
}

func generateTLSCert(crt, key string) (*tls.Certificate, error) {
	if crt == "" || key == "" {
		return nil, nil
	}

	certData, err := base64.StdEncoding.DecodeString(crt)
	if err != nil {
		return nil, err
	}
	keyData, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}
