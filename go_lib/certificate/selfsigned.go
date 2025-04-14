/*
Copyright 2021 Flant JSC

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

package certificate

import (
	"bytes"
	"log"
	"log/slog"
	"time"

	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

type Certificate struct {
	Key  string `json:"key"`
	Cert string `json:"cert"`
	CA   string `json:"ca"`
}

type SigningOption func(signing *config.Signing)

func WithSigningDefaultExpiry(expiry time.Duration) SigningOption {
	return func(signing *config.Signing) {
		signing.Default.Expiry = expiry
		signing.Default.ExpiryString = expiry.String()
	}
}

func WithSigningDefaultUsage(usage []string) SigningOption {
	return func(signing *config.Signing) {
		signing.Default.Usage = usage
	}
}

func GenerateSelfSignedCert(logger go_hook.Logger, cn string, ca Authority, options ...interface{}) (Certificate, error) {
	logger.Debug("Generate self-signed cert", slog.String("cn", cn))
	request := &csr.CertificateRequest{
		CN: cn,
		KeyRequest: &csr.KeyRequest{
			A: "ecdsa",
			S: 256,
		},
	}

	for _, option := range options {
		if f, ok := option.(Option); ok {
			f(request)
		}
	}

	// Catch cfssl output and show it only if error is occurred.
	var buf bytes.Buffer
	logWriter := log.Writer()

	log.SetOutput(&buf)
	defer log.SetOutput(logWriter)

	g := &csr.Generator{Validator: genkey.Validator}
	csrBytes, key, err := g.ProcessRequest(request)
	if err != nil {
		return Certificate{}, err
	}

	req := signer.SignRequest{Request: string(csrBytes)}

	parsedCa, err := helpers.ParseCertificatePEM([]byte(ca.Cert))
	if err != nil {
		return Certificate{}, err
	}

	priv, err := helpers.ParsePrivateKeyPEM([]byte(ca.Key))
	if err != nil {
		return Certificate{}, err
	}

	signingConfig := &config.Signing{
		Default: config.DefaultConfig(),
	}

	for _, option := range options {
		if f, ok := option.(SigningOption); ok {
			f(signingConfig)
		}
	}

	s, err := local.NewSigner(priv, parsedCa, signer.DefaultSigAlgo(priv), signingConfig)
	if err != nil {
		return Certificate{}, err
	}

	cert, err := s.Sign(req)
	if err != nil {
		return Certificate{}, err
	}

	return Certificate{CA: ca.Cert, Key: string(key), Cert: string(cert)}, nil
}
