/*
Copyright 2023 Flant JSC

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

package main

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/cloudflare/cfssl/cli"
	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/cli/sign"
	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/selfsign"
	"github.com/cloudflare/cfssl/signer"
	log "github.com/sirupsen/logrus"
)

const (
	selfSignedTLSCertLocation = "/certs/tls.crt"
	selfSignedTLSKeyLocation  = "/certs/tls.key"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "generate-proxy-certs" {
		cert, err := generateSelfSignedCertFromFrontProxyCA()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Certificate: %s", base64.StdEncoding.EncodeToString(cert))

		return
	}

	err := generateAndSaveSelfSignedCertAndKey(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}

// Function to generate a CA and key
func generateAndSaveSelfSignedCertAndKey(certHosts []string) error {
	req := csr.New()
	req.CN = certHosts[0]
	req.Hosts = certHosts

	csrGen := &csr.Generator{Validator: genkey.Validator}
	request, key, err := csrGen.ProcessRequest(req)
	if err != nil {
		return err
	}

	priv, err := helpers.ParsePrivateKeyPEM(key)
	if err != nil {
		return err
	}

	cert, err := selfsign.Sign(priv, request, config.DefaultConfig())
	if err != nil {
		return err
	}

	err = os.WriteFile(selfSignedTLSCertLocation, cert, 0600)
	if err != nil {
		return err
	}
	err = os.WriteFile(selfSignedTLSKeyLocation, key, 0600)
	if err != nil {
		return err
	}

	return nil
}

func generateSelfSignedCertFromFrontProxyCA() ([]byte, error) {
	conf, err := config.LoadConfig([]byte(`{"CN":"front-proxy-client","hosts":[""],"key":{"algo": "rsa","size": 2048},"signing":{"default":{"expiry":"72h","usages":["signing","key encipherment","requestheader-client"]}}}`))
	if err != nil {
		return nil, err
	}

	s, err := sign.SignerFromConfig(cli.Config{
		CAFile:    "/etc/kubernetes/pki/front-proxy-ca.crt",
		CAKeyFile: "/etc/kubernetes/pki/front-proxy-ca.key",
		CFG:       conf,
	})
	if err != nil {
		return nil, err
	}

	csrBase64 := os.Getenv("CSR")
	if len(csrBase64) == 0 {
		return nil, fmt.Errorf("CSR is empty")
	}

	csr, err := base64.StdEncoding.DecodeString(csrBase64)
	if err != nil {
		return nil, fmt.Errorf("Decode base64 error: %w", err)
	}

	return s.Sign(signer.SignRequest{Request: string(csr)})
}
