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
	"crypto/x509"
	"encoding/pem"
	"sort"
	"time"
)

// AssertT is the minimal subset of *testing.T (and the value returned by
// ginkgo.GinkgoT()) that the assertion helpers in this file rely on.
//
// Keeping the interface this narrow means the helpers are usable from:
//
//   - plain `go test` suites (`*testing.T` satisfies AssertT directly);
//   - Ginkgo specs (`GinkgoT()` returns a value that also satisfies AssertT).
//
// It also keeps the production package free of any gomega/ginkgo dependency.
type AssertT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// AssertOpensslVerifyOK is the in-test equivalent of:
//
//	openssl verify -CAfile <ca.crt> <tls.crt>
//
// It catches the two failure modes the legacy Deckhouse self-signed
// certificates exhibited and that strict validators reject:
//
//  1. leaf Subject == leaf Issuer (the depth-0 self-signed collision).
//     Only an explicit Go-side assertion reproduces this. crypto/x509
//     (and therefore kube-apiserver) silently accepts the collision, but
//     openssl, Java keystore, Trivy and MaxPatrol reject it. The classic
//     openssl error is:
//
//     error 18 at 0 depth lookup: self-signed certificate
//
//     The production fix is to put O=Deckhouse, OU=<module> on the CA's
//     Subject DN while keeping the leaf CN-only.
//
//  2. Everything else `openssl verify` checks: broken signature, expired
//     certificate, malformed BasicConstraints, etc. Performed by
//     x509.Certificate.Verify against a CertPool pre-loaded with the CA.
//
// On success the parsed CA and leaf certificates are returned so callers
// can chain follow-up assertions (Subject DN sanity, SAN/EKU, expiry).
//
// Direct shell equivalent:
//
//	kubectl -n <ns> get secret <name> -o jsonpath='{.data.ca\.crt}'  | base64 -d > /tmp/ca.crt
//	kubectl -n <ns> get secret <name> -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/tls.crt
//	openssl verify -CAfile /tmp/ca.crt /tmp/tls.crt
func AssertOpensslVerifyOK(t AssertT, caPEM, leafPEM string) (*x509.Certificate, *x509.Certificate) {
	t.Helper()

	caBlock, _ := pem.Decode([]byte(caPEM))
	if caBlock == nil {
		t.Fatalf("ca.crt PEM must decode")
		return nil, nil
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatalf("parse ca.crt: %v", err)
		return nil, nil
	}

	leafBlock, _ := pem.Decode([]byte(leafPEM))
	if leafBlock == nil {
		t.Fatalf("tls.crt PEM must decode")
		return nil, nil
	}
	leafCert, err := x509.ParseCertificate(leafBlock.Bytes)
	if err != nil {
		t.Fatalf("parse tls.crt: %v", err)
		return nil, nil
	}

	// The ONE check that crypto/x509.Verify will not perform for us.
	// Reproduces openssl's `error 18 at 0 depth lookup: self-signed certificate`.
	if leafCert.Subject.String() == leafCert.Issuer.String() {
		t.Fatalf(
			"openssl error 18 at 0 depth lookup: self-signed certificate; "+
				"leaf Subject == Issuer (%q). "+
				"Fix: give the CA a distinct Subject DN (O=Deckhouse, OU=<module>); "+
				"keep the leaf CN-only.",
			leafCert.Subject.String(),
		)
		return caCert, leafCert
	}

	// `openssl verify -CAfile ca.crt tls.crt` proper.
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(caPEM)) {
		t.Fatalf("ca.crt PEM is not loadable into a CertPool")
		return caCert, leafCert
	}
	if _, err = leafCert.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		t.Fatalf("openssl-equivalent verify failed: %v", err)
		return caCert, leafCert
	}
	return caCert, leafCert
}

// AssertCertBundleValid runs the full Deckhouse self-signed TLS contract
// check (see go_lib/hooks/tls_certificate/internal_tls.go) on a
// (CA, leaf, key) bundle:
//
//   - openssl-equivalent path validation (AssertOpensslVerifyOK);
//   - CA invariants: IsCA, BasicConstraintsValid, self-issued, O=Deckhouse,
//     OU non-empty;
//   - leaf invariants: not a CA, CN matches the configured value, Issuer
//     matches CA Subject, leaf has no O/OU (CN-only);
//   - DNSNames consist exactly of the expected SAN set;
//   - ExtendedKeyUsage contains serverAuth (catches the legacy
//     "requestheader-client" usage that produced empty EKU);
//   - NotAfter is within a sensible window (1y < ttl < 20y);
//   - per-SAN x509.Verify with KeyUsages=ServerAuth (the equivalent of
//     `openssl verify -purpose sslserver -CAfile ca.crt tls.crt` with
//     hostname matching);
//   - (crt, key) form a usable TLS key pair (catches mismatched keys/curves).
func AssertCertBundleValid(t AssertT, caPEM, crtPEM, keyPEM, expectedCN string, expectedSANs []string) {
	t.Helper()

	ca, leaf := AssertOpensslVerifyOK(t, caPEM, crtPEM)

	if !ca.IsCA {
		t.Fatalf("CA must have IsCA=true")
	}
	if !ca.BasicConstraintsValid {
		t.Fatalf("CA BasicConstraints must be valid")
	}
	if ca.Subject.String() != ca.Issuer.String() {
		t.Fatalf("CA must be self-issued: Subject=%q Issuer=%q",
			ca.Subject.String(), ca.Issuer.String())
	}
	if !sliceContains(ca.Subject.Organization, caOrganization) {
		t.Fatalf("CA Subject must contain O=%s; got %q", caOrganization, ca.Subject.String())
	}
	if len(ca.Subject.OrganizationalUnit) == 0 {
		t.Fatalf("CA Subject must carry an OU to differentiate from the leaf; got %q",
			ca.Subject.String())
	}

	if leaf.IsCA {
		t.Fatalf("leaf must not be a CA")
	}
	if leaf.Subject.CommonName != expectedCN {
		t.Fatalf("leaf CN: got %q, want %q", leaf.Subject.CommonName, expectedCN)
	}
	if leaf.Issuer.String() != ca.Subject.String() {
		t.Fatalf("leaf Issuer must equal CA Subject: leaf.Issuer=%q ca.Subject=%q",
			leaf.Issuer.String(), ca.Subject.String())
	}
	if len(leaf.Subject.Organization) > 0 {
		t.Fatalf("leaf must be CN-only (no O=); got %v", leaf.Subject.Organization)
	}
	if len(leaf.Subject.OrganizationalUnit) > 0 {
		t.Fatalf("leaf must be CN-only (no OU=); got %v", leaf.Subject.OrganizationalUnit)
	}

	if !stringSetEqual(leaf.DNSNames, expectedSANs) {
		t.Fatalf("SAN mismatch: got %v, want %v", leaf.DNSNames, expectedSANs)
	}

	if !sliceContainsExtKeyUsage(leaf.ExtKeyUsage, x509.ExtKeyUsageServerAuth) {
		t.Fatalf("leaf must carry serverAuth in ExtendedKeyUsage; got %v", leaf.ExtKeyUsage)
	}

	now := time.Now()
	if !leaf.NotAfter.After(now.Add(365 * 24 * time.Hour)) {
		t.Fatalf("leaf NotAfter must be > 1 year from now; got %s", leaf.NotAfter)
	}
	if !leaf.NotAfter.Before(now.Add(20 * 365 * 24 * time.Hour)) {
		t.Fatalf("leaf NotAfter must be < 20 years from now; got %s", leaf.NotAfter)
	}

	// per-SAN openssl-equivalent verify with sslserver purpose.
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM([]byte(caPEM))
	for _, san := range expectedSANs {
		if _, err := leaf.Verify(x509.VerifyOptions{
			Roots:     roots,
			DNSName:   san,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}); err != nil {
			t.Fatalf("leaf must verify for SAN %q with serverAuth EKU: %v", san, err)
		}
	}

	if _, err := tls.X509KeyPair([]byte(crtPEM), []byte(keyPEM)); err != nil {
		t.Fatalf("tls.X509KeyPair must succeed for the issued bundle: %v", err)
	}
}

func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func sliceContainsExtKeyUsage(haystack []x509.ExtKeyUsage, needle x509.ExtKeyUsage) bool {
	for _, eku := range haystack {
		if eku == needle {
			return true
		}
	}
	return false
}

func stringSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := append([]string(nil), a...)
	bCopy := append([]string(nil), b...)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}
