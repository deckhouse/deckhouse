package certificate

import (
	"bytes"
	"log"
	"os"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/sirupsen/logrus"
)

type Authority struct {
	Key  string `json:"key"`
	Cert string `json:"cert"`
}

type Option func(request *csr.CertificateRequest)

func WithKeyAlgo(algo string) Option {
	return func(request *csr.CertificateRequest) {
		request.KeyRequest.A = algo
	}
}

func WithKeySize(size int) Option {
	return func(request *csr.CertificateRequest) {
		request.KeyRequest.S = size
	}
}

func WithCAExpiry(expiry string) Option {
	return func(request *csr.CertificateRequest) {
		request.CA.Expiry = expiry
	}
}

func WithCAConfig(caConfig *csr.CAConfig) Option {
	return func(request *csr.CertificateRequest) {
		request.CA = caConfig
	}
}

func WithKeyRequest(keyRequest *csr.KeyRequest) Option {
	return func(request *csr.CertificateRequest) {
		request.KeyRequest = keyRequest
	}
}

func GenerateCA(logger *logrus.Entry, cn string, options ...Option) (Authority, error) {
	request := &csr.CertificateRequest{
		CN: cn,
		CA: &csr.CAConfig{
			Expiry: "87600h",
		},
		KeyRequest: &csr.KeyRequest{
			A: "ecdsa",
			S: 256,
		},
	}

	for _, option := range options {
		option(request)
	}

	// Catch cfssl logs message
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	ca, _, key, err := initca.New(request)
	if err != nil {
		logger.Errorln(buf.String())
	}

	return Authority{Cert: string(ca), Key: string(key)}, err
}
