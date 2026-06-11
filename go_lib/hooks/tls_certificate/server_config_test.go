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

package tls_certificate

import (
	"crypto/tls"
	"strings"
	"testing"
)

func TestApplyServerCategoryA_SetsTLS13AndClearsCiphers(t *testing.T) {
	c := &tls.Config{
		MinVersion: tls.VersionTLS10,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		},
		NextProtos: []string{"h2", "http/1.1"},
	}

	ApplyServerCategoryA(c)

	if c.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion: got %#x, want VersionTLS13 (%#x)", c.MinVersion, tls.VersionTLS13)
	}
	if len(c.CipherSuites) != 0 {
		t.Fatalf("CipherSuites: want empty for Category A (TLS 1.3-only), got %v", c.CipherSuites)
	}
	if got := c.NextProtos; len(got) != 2 || got[0] != "h2" || got[1] != "http/1.1" {
		t.Fatalf("NextProtos must be preserved; got %v", got)
	}
}

func TestApplyServerCategoryB_SetsTLS12AndAEADOnly(t *testing.T) {
	c := &tls.Config{}
	ApplyServerCategoryB(c)

	if c.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion: got %#x, want VersionTLS12 (%#x)", c.MinVersion, tls.VersionTLS12)
	}
	if len(c.CipherSuites) == 0 {
		t.Fatal("CipherSuites must be set for Category B")
	}

	allowed := map[uint16]struct{}{}
	for _, s := range CategoryBCipherSuites {
		allowed[s] = struct{}{}
	}
	for _, s := range c.CipherSuites {
		if _, ok := allowed[s]; !ok {
			t.Fatalf("cipher suite %#x not in CategoryBCipherSuites allow-list", s)
		}
	}
}

func TestCategoryBCipherSuites_NoForbiddenAlgorithms(t *testing.T) {
	// Names listed in CategoryBCipherSuiteNames must:
	//   - start with TLS_ECDHE_ (PFS),
	//   - contain _GCM_ or _CHACHA20_POLY1305_ (AEAD),
	//   - end with _SHA256 or _SHA384,
	//   - NOT contain _RSA_WITH_ (RSA key exchange),
	//   - NOT contain _CBC_,
	//   - NOT mention GOST/KUZNYECHIK/MAGMA.
	if len(CategoryBCipherSuiteNames) != len(CategoryBCipherSuites) {
		t.Fatalf("CategoryBCipherSuiteNames (%d) and CategoryBCipherSuites (%d) must match", len(CategoryBCipherSuiteNames), len(CategoryBCipherSuites))
	}
	for _, name := range CategoryBCipherSuiteNames {
		if !strings.HasPrefix(name, "TLS_ECDHE_") {
			t.Errorf("%s: not ECDHE-keyed (no PFS)", name)
		}
		// "TLS_RSA_WITH_*" is a key-exchange RSA suite (no PFS).
		// "TLS_ECDHE_RSA_WITH_*" uses ECDHE for the key exchange and only RSA
		// for the certificate signature, which is fine.
		if strings.HasPrefix(name, "TLS_RSA_WITH_") {
			t.Errorf("%s: forbidden RSA key exchange (no PFS)", name)
		}
		if strings.Contains(name, "_CBC_") {
			t.Errorf("%s: forbidden CBC suite", name)
		}
		if strings.HasSuffix(name, "_SHA") {
			t.Errorf("%s: forbidden SHA1 suite", name)
		}
		hasAEAD := strings.Contains(name, "_GCM_") || strings.Contains(name, "_CHACHA20_POLY1305_")
		if !hasAEAD {
			t.Errorf("%s: not an AEAD suite (require GCM or CHACHA20_POLY1305)", name)
		}
		if strings.Contains(name, "GOST") || strings.Contains(name, "KUZNYECHIK") || strings.Contains(name, "MAGMA") {
			t.Errorf("%s: GOST suites are not implemented in upstream Go", name)
		}
	}
}

func TestServerOptionCategoryA_AppliedLastWinsOverDowngrade(t *testing.T) {
	c := &tls.Config{}
	earlier := func(c *tls.Config) {
		c.MinVersion = tls.VersionTLS12
		c.CipherSuites = []uint16{tls.TLS_RSA_WITH_AES_128_CBC_SHA}
	}
	for _, opt := range []func(*tls.Config){earlier, ServerOptionCategoryA()} {
		opt(c)
	}
	if c.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion after Category A applied last: got %#x, want VersionTLS13", c.MinVersion)
	}
	if len(c.CipherSuites) != 0 {
		t.Fatalf("CipherSuites after Category A applied last: want empty, got %v", c.CipherSuites)
	}
}
