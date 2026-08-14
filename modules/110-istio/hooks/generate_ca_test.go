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

package hooks

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// b64 base64-encodes a string for embedding into a Secret's `data` in KubeStateSet YAML.
func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// hasGaugeSet reports whether the collected metrics contain a gauge-set for the
// named metric with the given `source` label (value 1).
func hasGaugeSet(m []operation.MetricOperation, name, source string) bool {
	for _, op := range m {
		if op.Action == operation.ActionGaugeSet && op.Name == name &&
			op.Labels["source"] == source && op.Value != nil && *op.Value == 1.0 {
			return true
		}
	}
	return false
}

// hasMetricName reports whether the collected metrics contain any gauge-set for
// the named metric (regardless of labels).
func hasMetricName(m []operation.MetricOperation, name string) bool {
	for _, op := range m {
		if op.Action == operation.ActionGaugeSet && op.Name == name {
			return true
		}
	}
	return false
}

// testCA is a generated CA (cert + key in PEM) used to build realistic referenced Secrets.
type testCA struct {
	cert     *x509.Certificate
	key      *rsa.PrivateKey
	certPEM  string
	keyPEM   string
	template *x509.Certificate
}

// generateTestCA builds a CA certificate/key valid from one hour ago to 24 hours from now.
// If parent is nil the CA is self-signed; otherwise it is signed by parent (an intermediate CA).
func generateTestCA(commonName string, parent *testCA) testCA {
	return generateTestCAWithValidity(commonName, parent, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
}

// generateTestCAWithValidity is like generateTestCA but with an explicit validity window,
// used to model expired (or not-yet-valid) plugged CAs.
func generateTestCAWithValidity(commonName string, parent *testCA, notBefore, notAfter time.Time) testCA {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	signerCert := tmpl
	signerKey := key
	if parent != nil {
		signerCert = parent.template
		signerKey = parent.key
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, signerCert, &key.PublicKey, signerKey)
	Expect(err).NotTo(HaveOccurred())

	cert, err := x509.ParseCertificate(der)
	Expect(err).NotTo(HaveOccurred())

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	return testCA{cert: cert, key: key, certPEM: certPEM, keyPEM: keyPEM, template: tmpl}
}

// pkcs8KeyPEM re-encodes a testCA's private key in PKCS#8 (`PRIVATE KEY`) form, which is
// what cert-manager actually writes to `tls.key` (the other helpers emit PKCS#1).
func pkcs8KeyPEM(ca testCA) string {
	der, err := x509.MarshalPKCS8PrivateKey(ca.key)
	Expect(err).NotTo(HaveOccurred())
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// generateCACertWithoutCertSign builds a self-signed cert with basicConstraints CA:TRUE but a
// KeyUsage that does NOT include keyCertSign. Such a cert claims to be a CA yet is not permitted
// to sign certificates, so a chain built on it would be rejected by conformant verifiers. It must
// be refused rather than published as the mesh signing CA.
func generateCACertWithoutCertSign(commonName string) testCA {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature, // no keyCertSign
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	cert, err := x509.ParseCertificate(der)
	Expect(err).NotTo(HaveOccurred())

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	return testCA{cert: cert, key: key, certPEM: certPEM, keyPEM: keyPEM, template: tmpl}
}

// generateNonCACert builds a self-signed leaf (non-CA) certificate/key.
func generateNonCACert(commonName string) testCA {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	cert, err := x509.ParseCertificate(der)
	Expect(err).NotTo(HaveOccurred())

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	return testCA{cert: cert, key: key, certPEM: certPEM, keyPEM: keyPEM, template: tmpl}
}

// generateSelfIssuedNotSelfSignedCA builds a CA certificate that is signed with its own private
// key (so a signature-only self-signed check passes) but carries a *different* Issuer DN than its
// Subject DN. Such a cert is not a real self-signed root: no standard tool produces it, but it
// models a hand-crafted/forged input that must not be accepted as its own trust anchor.
func generateSelfIssuedNotSelfSignedCA(subjectCN, issuerCN string) testCA {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: subjectCN},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	// Parent supplies the Issuer DN written into the cert, but we sign with the cert's OWN key,
	// so the resulting cert is self-issued (signature verifies against itself) yet Issuer != Subject.
	parent := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() + 1),
		Subject:      pkix.Name{CommonName: issuerCN},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	cert, err := x509.ParseCertificate(der)
	Expect(err).NotTo(HaveOccurred())

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	return testCA{cert: cert, key: key, certPEM: certPEM, keyPEM: keyPEM, template: tmpl}
}

func createReferencedCASecret(name, namespace string, data map[string][]byte) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	_, err := dependency.TestDC.MustGetK8sClient().
		CoreV1().
		Secrets(namespace).
		Create(context.TODO(), secret, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func deleteReferencedCASecret(name, namespace string) {
	err := dependency.TestDC.MustGetK8sClient().
		CoreV1().
		Secrets(namespace).
		Delete(context.TODO(), name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("Istio hooks :: generate_ca ::", func() {
	f := HookExecutionConfigInit(`{"global":{"discovery":{"clusterDomain":"cluster.flomaster"}},"istio":{"internal":{"ca":{}}}}`, "")

	Context("Empty cluster; empty values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should generate ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.root").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.chain").Exists()).To(BeTrue())

			caCert := f.ValuesGet("istio.internal.ca.cert").String()
			caRoot := f.ValuesGet("istio.internal.ca.root").String()
			caChain := f.ValuesGet("istio.internal.ca.chain").String()

			Expect(caCert).To(Equal(caRoot))
			Expect(caCert).To(Equal(caChain))

			block, _ := pem.Decode([]byte(caCert))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeTrue())
			Expect(cert.Subject.Organization[0]).To(Equal("d8-istio"))

			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("selfSigned"))
		})
	})

	Context("Empty cluster; multicluster is on", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should generate ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.root").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.chain").Exists()).To(BeTrue())
		})
	})

	Context("Secret cacerts is in cluster; values aren't set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa
  ca-key.pem: YmJi # bbb
  cert-chain.pem: Y2Nj # ccc
  root-cert.pem: ZWVl # eee
`))
			f.RunHook()
		})
		It("Should add existing ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal("aaa"))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal("bbb"))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal("ccc"))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal("eee"))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("cacerts"))
		})
	})

	Context("Secret cacerts is in cluster; inline intermediate CA values are set", func() {
		var rootCA, interCA testCA
		var inlineChain string
		BeforeEach(func() {
			// A valid inline *intermediate* CA (cert + explicit chain + root). It must win over the
			// existing cacerts Secret and pass the inline validation gate, including the root-anchoring
			// check (root actually anchors cert through the chain).
			rootCA = generateTestCA("inline-root-ca", nil)
			interCA = generateTestCA("inline-intermediate-ca", &rootCA)
			inlineChain = joinPEM(interCA.certPEM, rootCA.certPEM)
			f.ValuesSet("istio.ca.cert", interCA.certPEM)
			f.ValuesSet("istio.ca.key", interCA.keyPEM)
			f.ValuesSet("istio.ca.chain", inlineChain)
			f.ValuesSet("istio.ca.root", rootCA.certPEM)
			// this secret should be ignored
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa
  ca-key.pem: YmJi # bbb
  cert-chain.pem: Y2Nj # ccc
  root-cert.pem: ZWVl # eee
`))
			f.RunHook()
		})
		It("Should copy all inline cert data from values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(interCA.keyPEM))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(inlineChain))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("inline"))
		})
	})

	Context("inline istio.ca root does not anchor the cert", func() {
		var interCA, unrelatedRoot testCA
		BeforeEach(func() {
			// Inline intermediate whose declared root is unrelated — must hard-block, mirroring the
			// secretRef anchoring check, so a bad trust anchor is never published as the webhook caBundle.
			rootCA := generateTestCA("inline-real-root", nil)
			interCA = generateTestCA("inline-intermediate", &rootCA)
			unrelatedRoot = generateTestCA("inline-unrelated-root", nil)
			f.ValuesSet("istio.ca.cert", interCA.certPEM)
			f.ValuesSet("istio.ca.key", interCA.keyPEM)
			f.ValuesSet("istio.ca.root", unrelatedRoot.certPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("does not anchor the signing certificate"))
		})
	})

	Context("inline istio.ca explicitly uses the same NON-self-signed intermediate as cert and root", func() {
		var interCA testCA
		BeforeEach(func() {
			// The operator deliberately sets `root` == `cert` to a non-self-signed intermediate, i.e.
			// "anchor mesh trust at this intermediate". This is EXPLICIT self-anchoring: a valid (if
			// unusual) PKI choice that upstream istiod and the K8s API server both accept as-is, so the
			// module honors it rather than requiring self-signage. (Contrast the implicit no-root case,
			// which still hard-blocks because the true root would only be guessed.)
			rootCA := generateTestCA("inline-real-root", nil)
			interCA = generateTestCA("inline-intermediate", &rootCA)
			f.ValuesSet("istio.ca.cert", interCA.certPEM)
			f.ValuesSet("istio.ca.key", interCA.keyPEM)
			f.ValuesSet("istio.ca.root", interCA.certPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should accept the intermediate as its own explicit trust anchor", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("inline"))
		})
	})

	Context("inline istio.ca explicitly uses the same self-signed cert as cert and root", func() {
		var selfSignedCA testCA
		BeforeEach(func() {
			// Explicit root == cert is valid only when the signing cert is actually self-signed.
			selfSignedCA = generateTestCA("inline-selfsigned-ca", nil)
			f.ValuesSet("istio.ca.cert", selfSignedCA.certPEM)
			f.ValuesSet("istio.ca.key", selfSignedCA.keyPEM)
			f.ValuesSet("istio.ca.root", selfSignedCA.certPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should accept the cert as its own explicit root", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(selfSignedCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(selfSignedCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(selfSignedCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("inline"))
		})
	})

	Context("inline istio.ca is a NON-self-signed intermediate with no root", func() {
		var interCA testCA
		BeforeEach(func() {
			// An inline intermediate CA cert (signed by rootCA) supplied via cert/key only, with no
			// root. It is NOT self-signed, so its true root is unknowable here; defaulting root to
			// the cert would publish the intermediate as its own root-cert.pem/caBundle and wrongly
			// anchor trust at an intermediate. This must hard-block, mirroring the secretRef 'tls'
			// (missing ca.crt) and 'cacerts' (missing root-cert.pem) paths.
			rootCA := generateTestCA("inline-real-root", nil)
			interCA = generateTestCA("inline-intermediate", &rootCA)
			f.ValuesSet("istio.ca.cert", interCA.certPEM)
			f.ValuesSet("istio.ca.key", interCA.keyPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("not self-signed"))
			Expect(f.GoHookError.Error()).To(ContainSubstring("no 'root' trust anchor"))
		})
	})

	Context("inline istio.ca is a self-signed cert with no root", func() {
		var selfSignedCA testCA
		BeforeEach(func() {
			// A single self-signed CA cert supplied via cert/key only (the common case). The cert
			// genuinely IS its own root, so defaulting root/chain to it is correct and must NOT be
			// regressed by the non-self-signed hard-block above.
			selfSignedCA = generateTestCA("inline-selfsigned-ca", nil)
			f.ValuesSet("istio.ca.cert", selfSignedCA.certPEM)
			f.ValuesSet("istio.ca.key", selfSignedCA.keyPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should treat the signing cert as its own root and chain", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(selfSignedCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(selfSignedCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(selfSignedCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("inline"))
		})
	})

	Context("inline istio.ca is a self-signed cert with malformed explicit chain", func() {
		BeforeEach(func() {
			// Config-sourced CA material is hard-blocked on malformed fields.
			selfSignedCA := generateTestCA("inline-selfsigned-ca", nil)
			f.ValuesSet("istio.ca.cert", selfSignedCA.certPEM)
			f.ValuesSet("istio.ca.key", selfSignedCA.keyPEM)
			f.ValuesSet("istio.ca.chain", "not a PEM certificate")
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("certificate chain is not valid"))
		})
	})

	Context("inline istio.ca cert has a trailing private-key block", func() {
		BeforeEach(func() {
			// The signing cert is published verbatim as ca-cert.pem, so it must be exactly one CERTIFICATE
			// PEM block. A pasted private key after the cert must be rejected instead of being persisted.
			selfSignedCA := generateTestCA("inline-selfsigned-ca", nil)
			f.ValuesSet("istio.ca.cert", selfSignedCA.certPEM+"\n"+selfSignedCA.keyPEM)
			f.ValuesSet("istio.ca.key", selfSignedCA.keyPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("signing certificate is not valid"))
		})
	})

	Context("inline istio.ca cert has trailing non-PEM garbage", func() {
		BeforeEach(func() {
			// Same class of bug as a trailing private-key block: certs must be strict PEM material with
			// no garbage after the single CERTIFICATE block.
			selfSignedCA := generateTestCA("inline-selfsigned-ca", nil)
			f.ValuesSet("istio.ca.cert", selfSignedCA.certPEM+"\nthis is not a PEM block\n")
			f.ValuesSet("istio.ca.key", selfSignedCA.keyPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("signing certificate is not valid"))
		})
	})

	Context("inline istio.ca self-signed cert with an explicit root that has a trailing private-key block", func() {
		BeforeEach(func() {
			// The explicit root is the self-signed signing cert itself (so it takes the self-signed /
			// root-equals-cert branch), but with a stray private-key PEM block appended. certsEqual and
			// verifyCertIsSelfSigned only inspect the FIRST block, so without strict root validation this
			// would slip through and publish a private key inside root-cert.pem / the webhook caBundle.
			// It must hard-block instead.
			selfSignedCA := generateTestCA("inline-selfsigned-ca", nil)
			f.ValuesSet("istio.ca.cert", selfSignedCA.certPEM)
			f.ValuesSet("istio.ca.key", selfSignedCA.keyPEM)
			f.ValuesSet("istio.ca.root", selfSignedCA.certPEM+"\n"+selfSignedCA.keyPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("root certificate is not valid"))
		})
	})

	Context("inline istio.ca self-signed cert with an explicit root that has trailing non-PEM garbage", func() {
		BeforeEach(func() {
			// Same class of bug: the root's first block is the valid self-signed cert but trailing
			// non-PEM bytes follow. The first-block-only self-signed check would accept it; strict root
			// validation must reject it so garbage never reaches root-cert.pem / the webhook caBundle.
			selfSignedCA := generateTestCA("inline-selfsigned-ca", nil)
			f.ValuesSet("istio.ca.cert", selfSignedCA.certPEM)
			f.ValuesSet("istio.ca.key", selfSignedCA.keyPEM)
			f.ValuesSet("istio.ca.root", selfSignedCA.certPEM+"\nthis is not a PEM block\n")
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("root certificate is not valid"))
		})
	})

	Context("inline istio.ca cert is self-issued but NOT self-signed (Issuer != Subject) with no root", func() {
		BeforeEach(func() {
			// A CA cert signed with its own key (so a signature-only self-signed check passes) but
			// bearing a foreign Issuer DN. It is not a real self-signed root: publishing it as its own
			// root-cert.pem/caBundle would anchor mesh trust at a cert whose stated issuer never exists.
			// With no explicit root supplied, it must hard-block just like the non-self-signed case.
			forged := generateSelfIssuedNotSelfSignedCA("forged-ca", "nonexistent-issuer")
			f.ValuesSet("istio.ca.cert", forged.certPEM)
			f.ValuesSet("istio.ca.key", forged.keyPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("not self-signed"))
			Expect(f.GoHookError.Error()).To(ContainSubstring("no 'root' trust anchor"))
		})
	})

	Context("ca.secretRef 'tls' secret whose cert is self-issued but NOT self-signed and has no ca.crt", func() {
		BeforeEach(func() {
			// Same forged shape as above, delivered via a cert-manager 'tls' secret with no ca.crt
			// trust anchor. The true root is unknowable and the cert is not genuinely self-signed,
			// so the module must hard-block instead of publishing it as its own root-cert.pem.
			forged := generateSelfIssuedNotSelfSignedCA("forged-ca", "nonexistent-issuer")
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(forged.certPEM),
				"tls.key": []byte(forged.keyPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("not self-signed"))
			Expect(f.GoHookError.Error()).To(ContainSubstring("no 'ca.crt' trust anchor"))
		})
	})

	Context("inline istio.ca is malformed", func() {
		BeforeEach(func() {
			// Non-PEM inline material is user config, so it must hard-block like a malformed
			// secretRef rather than being pushed to istiod.
			f.ValuesSet("istio.ca.cert", "not-a-pem-cert")
			f.ValuesSet("istio.ca.key", "not-a-pem-key")
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("the inline CA in 'istio.ca.*' is not valid"))
		})
	})

	Context("ca.secretRef points to a cert-manager 'tls' secret (self-signed root)", func() {
		var rootCA testCA
		BeforeEach(func() {
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			// self-signed root: tls.crt == ca.crt
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should map cert-manager keys and dedupe the chain", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(rootCA.keyPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
			// tls.crt == ca.crt, so chain is deduped to just the signing cert
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))
		})
	})

	Context("ca.secretRef points to a cert-manager 'tls' secret (intermediate)", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			rootCA = generateTestCA("root-ca", nil)
			interCA = generateTestCA("intermediate-ca", &rootCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			// intermediate: tls.crt (signing cert) != ca.crt (root)
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(interCA.certPEM),
				"tls.key": []byte(interCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should map cert-manager keys and concatenate the chain", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(interCA.keyPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
			// chain = signing cert + root
			expectedChain := joinPEM(interCA.certPEM, rootCA.certPEM)
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(expectedChain))
		})
	})

	Context("ca.secretRef points to a cert-manager 'tls' secret with a chained tls.crt (leaf+intermediate)", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			rootCA = generateTestCA("root-ca", nil)
			interCA = generateTestCA("intermediate-ca", &rootCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			// cert-manager puts the whole leaf-first chain (root omitted) in tls.crt.
			// Here tls.crt = intermediate (leaf, the signing CA) + root as an intermediate entry.
			chainedTLSCrt := joinPEM(interCA.certPEM, rootCA.certPEM)
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(chainedTLSCrt),
				"tls.key": []byte(interCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should use only the leaf as ca-cert.pem, not the whole chain", func() {
			Expect(f).To(ExecuteSuccessfully())
			// ca-cert.pem must be exactly the single signing (intermediate) cert.
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
		})
	})

	Context("ca.secretRef 'tls' secret with a single self-signed cert and no ca.crt", func() {
		var rootCA testCA
		BeforeEach(func() {
			// A single self-signed CA cert with no ca.crt (e.g. before cert-manager propagates it):
			// the signing cert IS the root, so this must resolve, defaulting root/chain to the cert.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should treat the signing cert as its own root and chain", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(rootCA.certPEM))
		})
	})

	Context("ca.secretRef 'tls' secret with a single NON-self-signed cert and no ca.crt", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			// A single intermediate CA cert (signed by rootCA) stored alone in tls.crt with no ca.crt
			// and no intermediates in the chain. It is NOT self-signed, so its true root is unknowable
			// here; publishing it as its own root-cert.pem/caBundle would wrongly anchor trust at an
			// intermediate. This must hard-block instead of being silently accepted.
			rootCA = generateTestCA("root-ca", nil)
			interCA = generateTestCA("intermediate-ca", &rootCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(interCA.certPEM),
				"tls.key": []byte(interCA.keyPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("not self-signed"))
			Expect(f.GoHookError.Error()).To(ContainSubstring("no 'ca.crt' trust anchor"))
		})
	})

	Context("ca.secretRef 'tls' secret with a chained tls.crt but no ca.crt trust anchor", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			// tls.crt carries leaf + intermediate but there is no ca.crt. The true root is
			// unknowable (cert-manager omits it from tls.crt), so publishing the intermediate as
			// root-cert.pem/caBundle would silently break trust. This must hard-block.
			rootCA = generateTestCA("root-ca", nil)
			interCA = generateTestCA("intermediate-ca", &rootCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(joinPEM(interCA.certPEM, rootCA.certPEM)),
				"tls.key": []byte(interCA.keyPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("no 'ca.crt' trust anchor"))
		})
	})

	Context("ca.secretRef points to a Vault-issued intermediate CA (3-level chain, isCA: true)", func() {
		// Models a cert-manager Certificate with `isCA: true` issued by a Vault issuer
		// backed by a root -> intermediate PKI. cert-manager stores the issued signing
		// CA leaf-first, followed by the intermediate(s) (root omitted) in tls.crt, and
		// the trust anchor (root) in ca.crt — the same PEMBundle shape as other issuers.
		var rootCA, interCA, issuingCA testCA
		BeforeEach(func() {
			rootCA = generateTestCA("vault-root-ca", nil)
			interCA = generateTestCA("vault-intermediate-ca", &rootCA)
			issuingCA = generateTestCA("istiod-ca", &interCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			// tls.crt = issuing CA (leaf) + intermediate; ca.crt = root.
			chainedTLSCrt := joinPEM(issuingCA.certPEM, interCA.certPEM)
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(chainedTLSCrt),
				"tls.key": []byte(issuingCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should use the issuing CA as ca-cert.pem and keep the full chain", func() {
			Expect(f).To(ExecuteSuccessfully())
			// ca-cert.pem is exactly the single issuing CA cert (not the whole chain).
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(issuingCA.certPEM))
			// root-cert.pem is the Vault root.
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
			// cert-chain.pem = issuing CA + intermediate (from tls.crt) + root.
			expectedChain := joinPEM(issuingCA.certPEM, interCA.certPEM, rootCA.certPEM)
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(expectedChain))
		})
	})

	Context("ca.secretRef points to a native 'cacerts' secret", func() {
		var interCA, rootCA testCA
		BeforeEach(func() {
			rootCA = generateTestCA("root-ca", nil)
			interCA = generateTestCA("intermediate-ca", &rootCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"ca-cert.pem":    []byte(interCA.certPEM),
				"ca-key.pem":     []byte(interCA.keyPEM),
				"cert-chain.pem": []byte(joinPEM(interCA.certPEM, rootCA.certPEM)),
				"root-cert.pem":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should passthrough the cacerts keys", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(interCA.keyPEM))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(joinPEM(interCA.certPEM, rootCA.certPEM)))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
		})
	})

	Context("ca.secretRef 'cacerts' secret with an intermediate ca-cert.pem but no root-cert.pem", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			// A native cacerts Secret carrying an intermediate signing cert but omitting
			// root-cert.pem. The true root is unknowable here, so publishing the intermediate as
			// its own root-cert.pem would wrongly anchor trust at an intermediate (webhook caBundle
			// / workload trust root) and silently break the mesh. It must hard-block.
			rootCA = generateTestCA("real-root-ca", nil)
			interCA = generateTestCA("intermediate-ca", &rootCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"ca-cert.pem": []byte(interCA.certPEM),
				"ca-key.pem":  []byte(interCA.keyPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("missing 'root-cert.pem'"))
		})
	})

	Context("ca.secretRef 'cacerts' secret with a self-signed ca-cert.pem and no root-cert.pem", func() {
		var rootCA testCA
		BeforeEach(func() {
			// A native cacerts Secret with a self-signed signing cert and no root-cert.pem. The cert
			// genuinely IS its own root, so defaulting root-cert.pem to it is correct and accepted.
			rootCA = generateTestCA("selfsigned-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"ca-cert.pem": []byte(rootCA.certPEM),
				"ca-key.pem":  []byte(rootCA.keyPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should default root and chain to the self-signed cert", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(rootCA.certPEM))
		})
	})

	Context("ca.secretRef 'cacerts' secret with a self-signed ca-cert.pem and malformed cert-chain.pem", func() {
		BeforeEach(func() {
			// Config-sourced native cacerts material is hard-blocked on malformed fields.
			rootCA := generateTestCA("selfsigned-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"ca-cert.pem":    []byte(rootCA.certPEM),
				"ca-key.pem":     []byte(rootCA.keyPEM),
				"cert-chain.pem": []byte("not a PEM certificate"),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("certificate chain is not valid"))
		})
	})

	Context("ca.secretRef namespace defaults to d8-istio", func() {
		var rootCA testCA
		BeforeEach(func() {
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "d8-istio", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should find the secret in d8-istio", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(rootCA.keyPEM))
		})
	})

	Context("ca.secretRef is resolved on a scheduled run", func() {
		var rootCA testCA
		BeforeEach(func() {
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should resolve the CA via the schedule binding", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(rootCA.keyPEM))
		})
	})

	Context("ca.secretRef wins over an existing cacerts secret", func() {
		var rootCA testCA
		BeforeEach(func() {
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			// a cacerts secret exists but must be ignored in favor of secretRef
			f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa
  ca-key.pem: YmJi # bbb
  cert-chain.pem: Y2Nj # ccc
  root-cert.pem: ZWVl # eee
`)
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should use the referenced secret, not cacerts", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(rootCA.keyPEM))
		})
	})

	Context("ca.secretRef points to a missing secret", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("was not found"))
		})
	})

	Context("ca.secretRef points to a malformed secret", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			// cert-manager 'tls' secret missing tls.key
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte("xxx"),
				"ca.crt":  []byte("xxx"),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("tls.key"))
		})
	})

	Context("ca.secretRef points to a secret with unrecognized format", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"something.else": []byte("xxx"),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("recognized CA format"))
		})
	})

	Context("ca.secretRef points to a 'tls' secret with non-PEM garbage", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			// tls.crt/tls.key are present but not valid PEM certificate material.
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte("not a pem cert"),
				"tls.key": []byte("not a pem key"),
				"ca.crt":  []byte("not a pem cert"),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("malformed 'tls.crt'"))
		})
	})

	Context("ca.secretRef points to a 'tls' secret whose cert has a trailing private-key block", func() {
		BeforeEach(func() {
			selfSignedCA := generateTestCA("secretref-selfsigned-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(selfSignedCA.certPEM + "\n" + selfSignedCA.keyPEM),
				"tls.key": []byte(selfSignedCA.keyPEM),
				"ca.crt":  []byte(selfSignedCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("malformed 'tls.crt'"))
		})
	})

	Context("ca.secretRef points to a 'tls' secret whose cert has trailing non-PEM garbage", func() {
		BeforeEach(func() {
			selfSignedCA := generateTestCA("secretref-selfsigned-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(selfSignedCA.certPEM + "\nthis is not a PEM block\n"),
				"tls.key": []byte(selfSignedCA.keyPEM),
				"ca.crt":  []byte(selfSignedCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("malformed 'tls.crt'"))
		})
	})

	Context("ca.secretRef points to a 'tls' secret whose cert is not a CA", func() {
		var leaf testCA
		BeforeEach(func() {
			// A non-CA (leaf) certificate: reuse the generator but flip IsCA off.
			leaf = generateNonCACert("leaf.example.com")
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(leaf.certPEM),
				"tls.key": []byte(leaf.keyPEM),
				"ca.crt":  []byte(leaf.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("not a CA certificate"))
		})
	})

	Context("ca.secretRef points to a 'tls' secret whose CA cert cannot sign certificates", func() {
		var noSignCA testCA
		BeforeEach(func() {
			// A cert with basicConstraints CA:TRUE but no keyCertSign key usage. istiod might still
			// cryptographically sign workload certs, but conformant verifiers reject a chain whose
			// issuer is not permitted to sign certs, breaking mTLS after rollout. Must hard-block.
			noSignCA = generateCACertWithoutCertSign("no-certsign-ca")
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(noSignCA.certPEM),
				"tls.key": []byte(noSignCA.keyPEM),
				"ca.crt":  []byte(noSignCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("keyCertSign"))
		})
	})

	Context("ca.secretRef 'tls' secret whose ca.crt equals a NON-self-signed tls.crt", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			// A Secret that puts the same non-self-signed intermediate in both tls.crt and ca.crt.
			// This is EXPLICIT self-anchoring (root == cert): the operator has stated that this
			// intermediate is the trust anchor. istiod and the K8s API server accept an intermediate
			// anchor as-is, so the module honors it rather than requiring self-signage.
			rootCA = generateTestCA("tls-real-root", nil)
			interCA = generateTestCA("tls-intermediate", &rootCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(interCA.certPEM),
				"tls.key": []byte(interCA.keyPEM),
				"ca.crt":  []byte(interCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should accept the intermediate as its own explicit trust anchor", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))
		})
	})

	Context("ca.secretRef 'cacerts' secret whose root-cert.pem equals a NON-self-signed ca-cert.pem", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			// Same explicit self-anchoring via the native cacerts format: root-cert.pem is present and
			// equals ca-cert.pem (a non-self-signed intermediate). The operator has stated the anchor,
			// so the module honors it — consistent with the inline and 'tls' paths.
			rootCA = generateTestCA("cacerts-real-root", nil)
			interCA = generateTestCA("cacerts-intermediate", &rootCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"ca-cert.pem":   []byte(interCA.certPEM),
				"ca-key.pem":    []byte(interCA.keyPEM),
				"root-cert.pem": []byte(interCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should accept the intermediate as its own explicit trust anchor", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))
		})
	})

	Context("inline istio.ca explicitly sets root to a NON-self-signed cert equal to cert (cacerts-style)", func() {
		var interCA testCA
		BeforeEach(func() {
			// The inline analogue: cert == root, both a non-self-signed intermediate, with an explicit
			// chain. Explicit self-anchoring — accepted, consistent with the secretRef paths above.
			rootCA := generateTestCA("inline-eq-root", nil)
			interCA = generateTestCA("inline-eq-intermediate", &rootCA)
			f.ValuesSet("istio.ca.cert", interCA.certPEM)
			f.ValuesSet("istio.ca.key", interCA.keyPEM)
			f.ValuesSet("istio.ca.root", interCA.certPEM)
			f.ValuesSet("istio.ca.chain", interCA.certPEM)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should accept the intermediate as its own explicit trust anchor", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("inline"))
		})
	})

	Context("ca.secretRef points to a 'tls' secret whose key does not match the cert", func() {
		var caA, caB testCA
		BeforeEach(func() {
			caA = generateTestCA("ca-a", nil)
			caB = generateTestCA("ca-b", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			// cert from caA, key from caB — mismatched pair.
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(caA.certPEM),
				"tls.key": []byte(caB.keyPEM),
				"ca.crt":  []byte(caA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("key pair"))
		})
	})

	Context("ca.secretRef intermediate 'tls' secret whose ca.crt root does not anchor the signing cert", func() {
		var rootCA, unrelatedRoot, interCA testCA
		BeforeEach(func() {
			// interCA is signed by rootCA, but the Secret advertises an unrelated (valid) root in
			// ca.crt. Without root-anchoring validation this would be published as the webhook
			// caBundle and silently break the mesh, so it must hard-block.
			rootCA = generateTestCA("real-root-ca", nil)
			unrelatedRoot = generateTestCA("unrelated-root-ca", nil)
			interCA = generateTestCA("intermediate-ca", &rootCA)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(interCA.certPEM),
				"tls.key": []byte(interCA.keyPEM),
				"ca.crt":  []byte(unrelatedRoot.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("does not anchor the signing certificate"))
		})
	})

	Context("ca.secretRef points to a cert-manager 'tls' secret with a PKCS#8 key", func() {
		var rootCA testCA
		BeforeEach(func() {
			// cert-manager writes tls.key in PKCS#8 (`PRIVATE KEY`), not PKCS#1. Exercise that the
			// primary real-world key format is accepted by the key-pair validation.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(pkcs8KeyPEM(rootCA)),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should accept the PKCS#8 key", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(pkcs8KeyPEM(rootCA)))
		})
	})

	Context("ca.secretRef intermediate 'tls' secret whose chain is expired", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			// A correctly-anchored intermediate CA that has already expired. Expiry must NOT be
			// treated as a structural failure: the root-anchoring check verifies trust structure
			// only, so a scheduled re-resolution never hard-blocks a mesh purely because of the clock.
			rootCA = generateTestCAWithValidity("expired-root-ca", nil, time.Now().Add(-72*time.Hour), time.Now().Add(-48*time.Hour))
			interCA = generateTestCAWithValidity("expired-intermediate-ca", &rootCA, time.Now().Add(-72*time.Hour), time.Now().Add(-48*time.Hour))
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(interCA.certPEM),
				"tls.key": []byte(interCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should resolve successfully (expiry does not hard-block)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
		})
	})

	Context("ca.secretRef intermediate 'tls' secret whose root was issued AFTER the signing cert", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			// A correctly-anchored intermediate CA whose root has a *later* NotBefore than the signing
			// cert — e.g. after a root rotation, or when an issuer backdates the leaf's NotBefore. Both
			// certs are currently valid. The root-anchoring check must verify trust structure only and
			// NOT reject this as "root does not anchor cert" merely because of the issuance order.
			rootCA = generateTestCAWithValidity("late-root-ca", nil, time.Now(), time.Now().Add(240*time.Hour))
			interCA = generateTestCAWithValidity("backdated-intermediate-ca", &rootCA, time.Now().Add(-2*time.Hour), time.Now().Add(240*time.Hour))
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(interCA.certPEM),
				"tls.key": []byte(interCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should resolve successfully (issuance order does not hard-block)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
		})
	})

	Context("ca.secretRef intermediate 'tls' secret whose leaf and root validity windows do NOT overlap", func() {
		var rootCA, interCA testCA
		BeforeEach(func() {
			// A structurally-correct intermediate CA (interCA is genuinely signed by rootCA) whose
			// validity windows do not overlap at all: the signing (intermediate) cert is already
			// expired, while the root was freshly issued afterwards — e.g. an old leaf re-anchored
			// under a rotated root. No single point in time is inside every cert's window, so a
			// time-coupled check (x509.Verify with any single CurrentTime) would wrongly report
			// "root does not anchor cert". The pairwise signature walk verifies trust structure only
			// and must resolve successfully: expiry is istiod's concern, not a structural failure.
			rootCA = generateTestCAWithValidity("nonoverlap-root-ca", nil, time.Now().Add(-time.Hour), time.Now().Add(240*time.Hour))
			interCA = generateTestCAWithValidity("nonoverlap-expired-intermediate-ca", &rootCA, time.Now().Add(-72*time.Hour), time.Now().Add(-48*time.Hour))
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(interCA.certPEM),
				"tls.key": []byte(interCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should resolve successfully (non-overlapping validity windows do not hard-block)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(interCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(rootCA.certPEM))
		})
	})

	Context("ca.secretRef re-resolution fails after a successful first resolution", func() {
		var rootCA testCA
		var firstCert, firstKey string
		BeforeEach(func() {
			// First run resolves the referenced Secret and persists istio.internal.ca.*.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			firstCert = f.ValuesGet("istio.internal.ca.cert").String()
			firstKey = f.ValuesGet("istio.internal.ca.key").String()
			Expect(firstCert).NotTo(BeEmpty())

			// The source Secret disappears; a scheduled run fires and cannot re-resolve it.
			deleteReferencedCASecret("my-istio-ca", "my-pki")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should keep the last-good CA instead of hard-blocking", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(firstCert))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(firstKey))
			// The provenance marker matches the current secretRef, which is why the fallback is allowed.
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))
			// The degraded rotation must be surfaced as a metric. The kept material is still
			// valid, so only the unresolved metric fires, not the invalid-CA one.
			m := f.MetricsCollector.CollectedMetrics()
			Expect(hasGaugeSet(m, "d8_istio_ca_secretref_unresolved", "secretRef:my-pki/my-istio-ca")).To(BeTrue())
			Expect(hasMetricName(m, "d8_istio_ca_material_invalid")).To(BeFalse())
		})
	})

	Context("ca.secretRef re-resolution fails and the last-published CA is now INVALID", func() {
		var rootCA testCA
		var invalidCert = "not-a-valid-pem-cert-anymore"
		BeforeEach(func() {
			// Regression for the validation-severity asymmetry between the secretRef last-published
			// fallback and the module-owned reuse path. A secretRef resolves successfully, so
			// istio.internal.ca.* holds this secretRef's material with a matching provenance marker and
			// istiod is running on it. The material then becomes invalid *in place* while it is still the
			// live CA — modelling e.g. a chain that has since expired, or the non-overlapping-validity
			// window edge case in verifyRootAnchorsCert. The source Secret is then (transiently)
			// unavailable, so re-resolution fails.
			//
			// Removing the ca.secretRef config would reuse the identical bytes at log level only (the
			// module-owned reuse path), so keeping the config must NOT produce the opposite outcome. The
			// fallback must keep the last-published CA (log-only), never hard-block: the wantSource marker
			// match — not validity — is the safety gate, and hard-blocking would regress an already-
			// working mesh purely because of a transient/edge-case validation failure.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))

			// The live CA material goes invalid in place (marker still matches this secretRef), and the
			// source Secret disappears so a scheduled run cannot re-resolve it.
			f.ValuesSet("istio.internal.ca.cert", invalidCert)
			deleteReferencedCASecret("my-istio-ca", "my-pki")
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should keep the last-published CA (log-only) instead of hard-blocking", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(invalidCert))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))
			// Both metrics fire: the secretRef could not be re-resolved AND the kept material
			// is now invalid, so the operator is alerted istiod is being served bad material.
			m := f.MetricsCollector.CollectedMetrics()
			Expect(hasGaugeSet(m, "d8_istio_ca_secretref_unresolved", "secretRef:my-pki/my-istio-ca")).To(BeTrue())
			Expect(hasGaugeSet(m, "d8_istio_ca_material_invalid", "secretRef:my-pki/my-istio-ca")).To(BeTrue())
		})
	})

	Context("ca.secretRef re-resolution fails after a Deckhouse restart (durable cacerts fallback)", func() {
		var rootCA testCA
		var firstCert string
		BeforeEach(func() {
			// First run resolves the referenced Secret and persists istio.internal.ca.*. The module
			// would then render a d8-istio/cacerts Secret annotated with the same provenance marker.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			firstCert = f.ValuesGet("istio.internal.ca.cert").String()
			Expect(firstCert).NotTo(BeEmpty())

			// Simulate a Deckhouse restart: the volatile istio.internal.ca.* values are wiped (the
			// `ca` object itself is reset to empty, matching a fresh process start). The durable store
			// is the rendered d8-istio/cacerts Secret, carrying the provenance annotation. The source
			// secretRef Secret is (transiently) unavailable at restart time.
			f.ValuesSet("istio.internal.ca", map[string]interface{}{})
			deleteReferencedCASecret("my-istio-ca", "my-pki")
			f.KubeStateSet(fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
  annotations:
    istio.deckhouse.io/ca-source: secretRef:my-pki/my-istio-ca
data:
  ca-cert.pem: %s
  ca-key.pem: %s
  cert-chain.pem: %s
  root-cert.pem: %s
`,
				b64(rootCA.certPEM), b64(rootCA.keyPEM), b64(rootCA.certPEM), b64(rootCA.certPEM)))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should keep the last-good CA from the annotated cacerts Secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(rootCA.keyPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))
		})
	})

	Context("ca.secretRef re-resolution fails after restart with an UNRELATED cacerts Secret", func() {
		var rootCA testCA
		BeforeEach(func() {
			// After a restart (empty istio.internal.ca.*), the source secretRef Secret is missing and
			// the durable d8-istio/cacerts Secret carries a *different* provenance (e.g. a leftover
			// self-signed CA, or one from a different secretRef). The marker does not match the current
			// secretRef, so the module must hard-block rather than adopt an unrelated CA.
			rootCA = generateTestCA("leftover-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet(fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
  annotations:
    istio.deckhouse.io/ca-source: selfSigned
data:
  ca-cert.pem: %s
  ca-key.pem: %s
  cert-chain.pem: %s
  root-cert.pem: %s
`,
				b64(rootCA.certPEM), b64(rootCA.keyPEM), b64(rootCA.certPEM), b64(rootCA.certPEM)))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should fail the hook (hard block) instead of adopting the unrelated cacerts Secret", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("was not found"))
		})
	})

	Context("ca.secretRef points to a missing secret while a *self-signed* CA is already persisted", func() {
		var selfSignedCert string
		BeforeEach(func() {
			// A cluster that already runs on a generated self-signed CA. The operator then configures
			// ca.secretRef (e.g. with a typo) pointing to a Secret that does not exist. The first
			// resolution of *this* secretRef must hard-block: the persisted CA is self-signed, not this
			// secretRef's material, so silently keeping it would leave the mesh on an unrelated CA the
			// operator did not ask for.
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			selfSignedCert = f.ValuesGet("istio.internal.ca.cert").String()
			Expect(selfSignedCert).NotTo(BeEmpty())
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("selfSigned"))

			// Now point at a non-existent Secret and fire a scheduled run.
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should fail the hook (hard block) instead of keeping the self-signed CA", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("was not found"))
		})
	})

	Context("ca.secretRef is repointed to a *different* missing secret", func() {
		var rootCA testCA
		BeforeEach(func() {
			// secretRef A resolves successfully and persists its material with a matching marker.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: ca-a
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("ca-a", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/ca-a"))

			// The operator repoints to a *different* secretRef B that does not exist. Because the
			// persisted material belongs to A (marker mismatch), this must hard-block rather than keep A.
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: ca-b
  namespace: my-pki
`))
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should fail the hook (hard block) instead of keeping the previous secretRef's CA", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("was not found"))
		})
	})

	Context("ca.secretRef initial resolution fails with no persisted CA", func() {
		BeforeEach(func() {
			// No previously-resolved CA exists (istio.internal.ca.* is empty), so a missing source
			// Secret must hard-block: there is nothing safe to fall back to.
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should fail the hook (hard block)", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("was not found"))
		})
	})

	Context("scheduled run with no cacerts and an already-generated self-signed CA", func() {
		var firstCert, firstKey string
		BeforeEach(func() {
			// First run (no cacerts, no secretRef) generates a self-signed CA.
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			firstCert = f.ValuesGet("istio.internal.ca.cert").String()
			firstKey = f.ValuesGet("istio.internal.ca.key").String()
			Expect(firstCert).NotTo(BeEmpty())

			// A scheduled run fires while the cacerts Secret snapshot is still empty.
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should NOT rotate the CA (reuse the previously generated one)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(firstCert))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(firstKey))
		})
	})

	Context("ca.secretRef is removed and the cacerts Secret is deleted within the same process", func() {
		var rootCA testCA
		var externalCert string
		BeforeEach(func() {
			// The operator runs on an external (secretRef) CA, then removes the ca.secretRef config AND
			// deletes the module-owned d8-istio/cacerts Secret — but the Deckhouse process keeps running,
			// so the volatile istio.internal.ca.* values still hold the last-published external material.
			//
			// The dead external marker is demoted to `cacerts` and the internal-reuse fallback keeps the
			// last-published CA rather than rotating the live mesh root. This is the intentional safe
			// trade-off of demoting the marker up front: a transiently-empty snapshot (or any run ordering
			// right after config removal) can never surprise-rotate the mesh CA to a fresh self-signed one.
			// The actual revert to self-signed only takes effect once the volatile values are also gone
			// (a fresh process start / restart) — see the next Context.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			externalCert = f.ValuesGet("istio.internal.ca.cert").String()
			Expect(externalCert).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))

			// Remove ca.secretRef from config; the cacerts Secret is deleted (nothing left to re-adopt).
			// The volatile istio.internal.ca.* values still hold the external material and marker.
			f.ValuesSet("istio.ca", map[string]interface{}{})
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should keep the last-published CA (no surprise rotation) and demote the marker to cacerts", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(externalCert))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("cacerts"))
		})
	})

	Context("ca.secretRef is removed and the cacerts Secret is deleted across a restart", func() {
		var rootCA testCA
		var externalCert string
		BeforeEach(func() {
			// Same deliberate revert-to-self-signed, but now across a Deckhouse restart: the config is
			// removed, the cacerts Secret is deleted, AND the volatile istio.internal.ca.* values are wiped
			// (a fresh process start). With no snapshot to re-adopt and no persisted internal CA, the hook
			// reaches the generate case and produces a fresh self-signed CA. This is the point at which the
			// revert actually takes effect.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			externalCert = f.ValuesGet("istio.internal.ca.cert").String()
			Expect(externalCert).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))

			// Remove ca.secretRef, delete the cacerts Secret, and wipe the volatile internal values to
			// model a fresh process start.
			f.ValuesSet("istio.ca", map[string]interface{}{})
			f.ValuesSet("istio.internal.ca", map[string]interface{}{})
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should generate a fresh self-signed CA instead of resurrecting the external one", func() {
			Expect(f).To(ExecuteSuccessfully())
			newCert := f.ValuesGet("istio.internal.ca.cert").String()
			Expect(newCert).NotTo(BeEmpty())
			Expect(newCert).NotTo(Equal(externalCert))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("selfSigned"))

			block, _ := pem.Decode([]byte(newCert))
			Expect(block).NotTo(BeNil())
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeTrue())
			Expect(cert.Subject.Organization[0]).To(Equal("d8-istio"))
		})
	})

	Context("ca.secretRef removed; a schedule fires with an empty snapshot BEFORE the transitional beforeHelm", func() {
		var rootCA testCA
		var externalCert string
		BeforeEach(func() {
			// Regression for the rotation race: the operator removes ca.secretRef, but a scheduled run
			// fires before the transitional beforeHelm run gets a chance to re-stamp the durable cacerts
			// Secret. On that scheduled run the cacerts snapshot is *also* transiently empty (informer
			// hiccup / brief delete-recreate) and the volatile marker is still the external `secretRef:...`.
			// Previously the reuse branch was skipped for an external marker, so the hook generated a fresh
			// self-signed CA and rotated the live mesh root, breaking mTLS mesh-wide. The up-front demotion
			// of the dead external marker must keep the last-published CA regardless of run ordering.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			externalCert = f.ValuesGet("istio.internal.ca.cert").String()
			Expect(externalCert).To(Equal(rootCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))

			// Config removed; the cacerts snapshot is transiently empty; a SCHEDULE fires (not beforeHelm),
			// so the still-external marker is observed before any re-stamp.
			f.ValuesSet("istio.ca", map[string]interface{}{})
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should NOT rotate the CA to a fresh self-signed one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(externalCert))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("cacerts"))
		})
	})

	Context("ca.secretRef removed but cacerts Secret kept, then a transiently-empty snapshot on schedule", func() {
		var rootCA testCA
		var externalCert string
		BeforeEach(func() {
			// The operator runs on an external (secretRef) CA and then removes the ca.secretRef config,
			// but intentionally KEEPS the module-owned d8-istio/cacerts Secret (the documented "keep the
			// last-published CA" state). The adopted Secret must be re-stamped module-owned (`cacerts`), so
			// that a later transiently-empty snapshot on the schedule reuses it via the internal-reuse
			// fallback instead of rotating the live mesh CA to a fresh self-signed one (which would break
			// mTLS mesh-wide). This is the anti-rotation guarantee that a still-`secretRef`-marked source
			// would otherwise defeat.
			rootCA = generateTestCA("istiod-ca", nil)
			f.ValuesSetFromYaml("istio.ca", []byte(`
secretRef:
  name: my-istio-ca
  namespace: my-pki
`))
			f.KubeStateSet("")
			createReferencedCASecret("my-istio-ca", "my-pki", map[string][]byte{
				"tls.crt": []byte(rootCA.certPEM),
				"tls.key": []byte(rootCA.keyPEM),
				"ca.crt":  []byte(rootCA.certPEM),
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			externalCert = f.ValuesGet("istio.internal.ca.cert").String()
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("secretRef:my-pki/my-istio-ca"))

			// Remove ca.secretRef; the rendered cacerts Secret (annotated with the secretRef marker) is
			// still present in the snapshot and gets adopted. It must be re-stamped to `cacerts`.
			f.ValuesSet("istio.ca", map[string]interface{}{})
			f.KubeStateSet(fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
  annotations:
    istio.deckhouse.io/ca-source: secretRef:my-pki/my-istio-ca
data:
  ca-cert.pem: %s
  ca-key.pem: %s
  cert-chain.pem: %s
  root-cert.pem: %s
`,
				b64(rootCA.certPEM), b64(rootCA.keyPEM), b64(rootCA.certPEM), b64(rootCA.certPEM)))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(externalCert))
			// Provenance is re-stamped to module-owned so the anti-rotation fallback stays active.
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("cacerts"))

			// A scheduled run fires while the cacerts snapshot is transiently empty (informer hiccup /
			// brief delete-recreate), still with no ca.* config.
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should NOT rotate the CA (reuse the adopted module-owned one)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(externalCert))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("cacerts"))
		})
	})

	Context("Secret cacerts is in cluster; inline values are not fully set", func() {
		var inlineCA testCA
		BeforeEach(func() {
			// Only cert/key are provided inline; chain/root must default to cert. The material is a
			// valid self-signed CA so it passes the inline validation gate.
			inlineCA = generateTestCA("inline-ca", nil)
			f.ValuesSet("istio.ca.cert", inlineCA.certPEM)
			f.ValuesSet("istio.ca.key", inlineCA.keyPEM)
			// this secret should be ignored
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa
  ca-key.pem: YmJi # bbb
  cert-chain.pem: Y2Nj # ccc
  root-cert.pem: ZWVl # eee
`))
			f.RunHook()
		})
		It("Should copy cert data from values, root and chain should be set to cert", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal(inlineCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal(inlineCA.keyPEM))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal(inlineCA.certPEM))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal(inlineCA.certPEM))
		})
	})

	Context("Secret cacerts in cluster omits cert-chain.pem and root-cert.pem", func() {
		BeforeEach(func() {
			// A minimal cacerts Secret (only ca-cert.pem/ca-key.pem). The snapshot path must
			// still yield a self-consistent CA: chain and root default to the signing cert,
			// so the rendered cacerts Secret and webhook caBundle are never empty.
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa
  ca-key.pem: YmJi # bbb
`))
			f.RunHook()
		})
		It("Should default chain and root to the signing cert", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal("aaa"))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal("bbb"))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal("aaa"))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal("aaa"))
		})
	})

	Context("Secret cacerts in cluster carries invalid CA material", func() {
		BeforeEach(func() {
			// A cacerts Secret already live in the cluster whose material is not a valid CA (garbage,
			// not PEM). Unlike the config-sourced paths (inline / secretRef), the snapshot path must
			// NOT hard-block: this is material istiod may already be running with, and hard-blocking
			// on a scheduled run would regress a working mesh. It is validated at log level only and
			// published unchanged.
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa (not a valid PEM certificate)
  ca-key.pem: YmJi # bbb
`))
			f.RunHook()
		})
		It("Should NOT hard-block and should publish the material unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal("aaa"))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal("bbb"))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("cacerts"))
			// Publishing invalid module-owned material as-is must raise the invalid-CA metric.
			m := f.MetricsCollector.CollectedMetrics()
			Expect(hasGaugeSet(m, "d8_istio_ca_material_invalid", "cacerts")).To(BeTrue())
		})
	})

	Context("invalid live cacerts published log-only, then a transiently-empty snapshot on schedule", func() {
		BeforeEach(func() {
			// Regression for the log-only vs. hard-block inconsistency: an out-of-band cacerts Secret
			// with invalid material is published as-is with only a warning (the `len(certs) == 1`
			// path). On a subsequent scheduled run where the snapshot is transiently empty (informer
			// hiccup / brief delete-recreate), the internal-reuse fallback handles the SAME
			// module-owned material and must react identically: log-only, never hard-block. Otherwise
			// a transient snapshot gap would escalate the prior warning into a module-wide render
			// failure.
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa (not a valid PEM certificate)
  ca-key.pem: YmJi # bbb
`))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("cacerts"))

			// The snapshot goes transiently empty; a scheduled run fires.
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})
		It("Should NOT hard-block and should keep reusing the material", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal("aaa"))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal("bbb"))
			Expect(f.ValuesGet("istio.internal.ca.source").String()).To(Equal("cacerts"))
			// The internal-reuse fallback publishes the SAME invalid material, so it must raise
			// the invalid-CA metric too — identical severity to the live-snapshot path above.
			m := f.MetricsCollector.CollectedMetrics()
			Expect(hasGaugeSet(m, "d8_istio_ca_material_invalid", "cacerts")).To(BeTrue())
		})
	})
})
