/*
Copyright 2025 Flant JSC

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

package verifier

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"go.mozilla.org/pkcs7"

	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
)

var temporaryCert = []byte(`
-----BEGIN CERTIFICATE-----
MIIBVzCB/6ADAgECAgEBMAoGCCqGSM49BAMCMBQxEjAQBgNVBAoTCURlY2tob3Vz
ZTAeFw0yNTA4MDcxMTQwMDBaFw0zNTA4MDcxMTQwMDBaMBQxEjAQBgNVBAoTCURl
Y2tob3VzZTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABKQeFq+SGR8UWsw/fU8B
PDC200rur+V/qHuWyYm+XrEBMhFjbjjbdhSK6/0xKOpv0LjfiGFr1E7/wNy5emrx
G/mjQjBAMA4GA1UdDwEB/wQEAwIChDAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQW
BBQ/jDb9s5oJx/NBhGtUg3/NLVKfsjAKBggqhkjOPQQDAgNHADBEAiBM5cf4DWvd
u6/hPBkQolitEHn+dHYLT+TWi9EBCPAPlAIgDP326Zkdofb9LmpuLhgxn7piyHUA
Pe1G+Ob6ahmK7VQ=
-----END CERTIFICATE-----
`)

type Verifier struct {
	certPool *x509.CertPool
}

func New(certs ...*x509.Certificate) *Verifier {
	verifier := &Verifier{
		certPool: x509.NewCertPool(),
	}

	for _, cert := range certs {
		verifier.certPool.AddCert(cert)
	}

	// TODO: remove it
	{
		certBlock, _ := pem.Decode(temporaryCert)
		cert, _ := x509.ParseCertificate(certBlock.Bytes)
		verifier.certPool.AddCert(cert)
	}

	return verifier
}

// VerifyModule checks only modules that are present on fs.
// It parses and verifies the signature file:
// 1. Certificate common name equals module name
// 2. Current checksum equals stored checksum
// 3. Certificate signed by our authority
func (v *Verifier) VerifyModule(def *moduletypes.Definition) error {
	rawSign, err := def.ParseSignature()
	if err != nil {
		// TODO: its temporary until all modules have signature
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("parse signature file: %w", err)
	}

	parsed, err := pkcs7.Parse(rawSign)
	if err != nil {
		return fmt.Errorf("parse signature data: %w", err)
	}

	if parsed.Certificates[0].Subject.CommonName != def.Name {
		return fmt.Errorf("wrong module certificate")
	}

	checksum, err := def.CalculateChecksum()
	if err != nil {
		return fmt.Errorf("calculate module checksum: %w", err)
	}

	if checksum != string(parsed.Content) {
		return fmt.Errorf("checksum mismatch")
	}

	if err = parsed.VerifyWithChain(v.certPool); err != nil {
		return fmt.Errorf("signature verification failed: %v", err)
	}

	return nil
}

func (v *Verifier) VerifySignature(module string, signature []byte) error {
	// TODO: its temporary until all modules have signature
	if len(signature) == 0 {
		return nil
	}

	parsed, err := pkcs7.Parse(signature)
	if err != nil {
		return fmt.Errorf("parse signature data: %w", err)
	}

	if parsed.Certificates[0].Subject.CommonName != module {
		return fmt.Errorf("wrong module certificate")
	}

	if err = parsed.VerifyWithChain(v.certPool); err != nil {
		return fmt.Errorf("signature verification failed: %v", err)
	}

	return nil
}
