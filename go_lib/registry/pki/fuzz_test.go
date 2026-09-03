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

// Fuzz harnesses for the module's public key infrastructure helpers.
//
// These functions are the trust boundary of the registry module: the material
// they parse arrives from Kubernetes secrets (registry-pki, registry-state,
// registry-node-config-<node>) and decides which certificate every node in the
// cluster trusts when pulling images.
//
// Threat model coverage (registry-threat-model.md): TM-01, TM-07. The harnesses
// assert two properties:
//
//   - Robustness: no input may panic. These decoders are called from
//     Config.Validate() in registry-nodeservices-manager and from the
//     orchestrator hooks, so a panic denies node services reconciliation.
//   - Soundness of the chain check: ValidateCertWithCAChain must not accept a
//     certificate that was not issued by the supplied CA.
package pki

import (
	"testing"
	"unicode/utf8"
)

// pemSeeds are malformed PEM payloads for the decoders.
//
// Several carry a private-key header, which a secret scanner is right to notice.
// None carries a key: the body of every one is the literal "AAAA", which is four
// zero bytes of base64 and decodes to nothing a parser will accept. The scanner
// markers below say exactly that, line by line.
var pemSeeds = [][]byte{
	nil,
	[]byte(""),
	[]byte("not pem at all"),
	[]byte("-----BEGIN CERTIFICATE-----"),
	[]byte("-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"),
	[]byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"),
	[]byte("-----BEGIN CERTIFICATE-----\n!!!!\n-----END CERTIFICATE-----\n"),
	[]byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"),         // gitleaks:allow
	[]byte("-----BEGIN RSA PRIVATE KEY-----\nAAAA\n-----END RSA PRIVATE KEY-----\n"), // gitleaks:allow
	[]byte("-----BEGIN EC PRIVATE KEY-----\nAAAA\n-----END EC PRIVATE KEY-----\n"),   // gitleaks:allow
	// Two blocks: parsers differ on which one wins.
	[]byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n" +
		"-----BEGIN CERTIFICATE-----\nBBBB\n-----END CERTIFICATE-----\n"),
	// Headers in the PEM block.
	[]byte("-----BEGIN CERTIFICATE-----\nProc-Type: 4,ENCRYPTED\n\nAAAA\n-----END CERTIFICATE-----\n"),
	[]byte("\x00\x01\x02\x03"),
}

func FuzzDecodeCertificate(f *testing.F) {
	for _, seed := range pemSeeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, pemData []byte) {
		cert, err := DecodeCertificate(pemData)
		if err != nil {
			return
		}
		if cert == nil {
			t.Fatalf("DecodeCertificate returned a nil certificate and a nil error for %q", pemData)
		}
		// A decoded certificate must be usable by the callers without further checks.
		_ = cert.Subject.String()
		_ = cert.NotAfter
	})
}

func FuzzDecodePrivateKey(f *testing.F) {
	for _, seed := range pemSeeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, pemData []byte) {
		key, err := DecodePrivateKey(pemData)
		if err != nil {
			return
		}
		if key == nil {
			t.Fatalf("DecodePrivateKey returned a nil key and a nil error for %q", pemData)
		}
		if key.Public() == nil {
			t.Fatalf("DecodePrivateKey returned a key without a public part for %q", pemData)
		}
	})
}

// FuzzDecodeCertKey drives the cert/key pairing check used for every service
// certificate in the node configuration secret.
func FuzzDecodeCertKey(f *testing.F) {
	for _, certSeed := range pemSeeds {
		f.Add(certSeed, []byte("-----BEGIN EC PRIVATE KEY-----\nAAAA\n-----END EC PRIVATE KEY-----\n")) // gitleaks:allow
	}
	for _, keySeed := range pemSeeds {
		f.Add([]byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"), keySeed)
	}

	f.Fuzz(func(t *testing.T, certPEM, keyPEM []byte) {
		certKey, err := DecodeCertKey(certPEM, keyPEM)
		if err != nil {
			return
		}
		if certKey.Cert == nil || certKey.Key == nil {
			t.Fatalf("DecodeCertKey succeeded but returned cert=%v key=%v", certKey.Cert, certKey.Key)
		}
		// Success is documented to mean the key matches the certificate.
		equal, err := ComparePublicKeys(certKey.Cert.PublicKey, certKey.Key.Public())
		if err != nil {
			t.Fatalf("DecodeCertKey succeeded but its result cannot be compared: %v", err)
		}
		if !equal {
			t.Fatalf("DecodeCertKey succeeded for a certificate and key that do not match")
		}
	})
}

// FuzzValidateCertWithCAChain checks that the chain verification used by
// ValidateCertWithCAChain cannot be satisfied by attacker-supplied PEM material
// alone: a certificate must only verify against a CA that actually issued it.
func FuzzValidateCertWithCAChain(f *testing.F) {
	f.Add([]byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"))
	f.Add([]byte(""))
	f.Add([]byte("x"))

	// A real CA and a certificate it issued, plus an unrelated CA.
	trustedCA, err := GenerateCACertificate("fuzz-trusted-ca")
	if err != nil {
		f.Fatalf("cannot generate trusted CA: %v", err)
	}
	foreignCA, err := GenerateCACertificate("fuzz-foreign-ca")
	if err != nil {
		f.Fatalf("cannot generate foreign CA: %v", err)
	}
	issued, err := GenerateCertificate("fuzz-leaf", trustedCA, "127.0.0.1", "localhost")
	if err != nil {
		f.Fatalf("cannot generate leaf certificate: %v", err)
	}

	f.Fuzz(func(t *testing.T, pemData []byte) {
		// Must not panic on an empty chain or on arbitrary decoded material.
		if err := ValidateCertWithCAChain(issued.Cert); err == nil {
			t.Fatalf("ValidateCertWithCAChain accepted an empty CA chain")
		}

		// The genuine pair must keep verifying: a regression here would break every
		// node configuration load.
		if err := ValidateCertWithCAChain(issued.Cert, trustedCA.Cert); err != nil {
			t.Fatalf("ValidateCertWithCAChain rejected a certificate issued by the given CA: %v", err)
		}

		// An unrelated CA must never verify it, regardless of what else is supplied.
		if err := ValidateCertWithCAChain(issued.Cert, foreignCA.Cert); err == nil {
			t.Fatalf("ValidateCertWithCAChain accepted a certificate against an unrelated CA")
		}

		// Whatever the fuzzed PEM decodes to, it must not turn into a trust anchor.
		injected, err := DecodeCertificate(pemData)
		if err != nil || injected == nil {
			return
		}
		if err := ValidateCertWithCAChain(issued.Cert, injected); err == nil {
			t.Fatalf("ValidateCertWithCAChain accepted a certificate against a CA decoded from %q", pemData)
		}
		if err := ValidateCertWithCAChain(injected, trustedCA.Cert); err == nil &&
			!injected.Equal(issued.Cert) {
			t.Fatalf("ValidateCertWithCAChain accepted a foreign certificate %q against the trusted CA",
				injected.Subject.String())
		}
	})
}

// FuzzComputeHash guards the configuration version hash: nodes compare it to
// decide whether they already applied a configuration, so it must be defined and
// stable for every value the module hashes.
func FuzzComputeHash(f *testing.F) {
	f.Add("", "")
	f.Add("a", "b")
	f.Add("\x00", "\xff")
	f.Add("10.0.0.1:5001", "10.0.0.2:5001")

	f.Fuzz(func(t *testing.T, first, second string) {
		type payload struct {
			First  string   `json:"first"`
			Second string   `json:"second"`
			List   []string `json:"list"`
		}

		value := payload{First: first, Second: second, List: []string{first, second}}

		hash, err := ComputeHash(&value)
		if err != nil {
			t.Fatalf("ComputeHash failed for %+v: %v", value, err)
		}
		if len(hash) != 64 {
			t.Fatalf("ComputeHash returned a %d-character hash for %+v", len(hash), value)
		}

		again, err := ComputeHash(&value)
		if err != nil {
			t.Fatalf("ComputeHash failed on repeat for %+v: %v", value, err)
		}
		if hash != again {
			t.Fatalf("ComputeHash is not stable for %+v: %q then %q", value, hash, again)
		}

		// Distinct payloads must produce distinct versions, otherwise a node can
		// consider a new configuration already applied.
		//
		// Restricted to valid UTF-8: ComputeHash hashes the json.Marshal output,
		// which replaces every invalid byte with U+FFFD, so distinct invalid
		// sequences legitimately collapse onto the same hash. Nothing that reaches
		// ComputeHash can carry invalid UTF-8 (the values come from JSON/YAML
		// decoded Kubernetes objects), so that collapse is out of scope here.
		if first != second && utf8.ValidString(first) && utf8.ValidString(second) {
			swapped := payload{First: second, Second: first, List: []string{second, first}}
			other, err := ComputeHash(&swapped)
			if err != nil {
				t.Fatalf("ComputeHash failed for %+v: %v", swapped, err)
			}
			if other == hash {
				t.Fatalf("ComputeHash collides for %+v and %+v", value, swapped)
			}
		}
	})
}
