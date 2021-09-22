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

package hooks

/*

User-stories:
1. For using TLS for bashible apiserver need a pair of certificates. This hook generates them and stores in cluster as Secret resource.

*/

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"nodeManager":{"internal":{}}}`
	initConfigValuesString = `{}`
	bashibleAPIServerNs    = "d8-cloud-instance-manager"
)

type bashibleAPIServerCertFields struct {
	ca  string
	crt string
	key string
}

type bashibleAPIServerGenCertTestFixtures struct {
	cert  bashibleAPIServerCertFields
	state string
}

func setupBashibleAPIServerCertHookTest(cert bashibleAPIServerCertFields) bashibleAPIServerGenCertTestFixtures {
	encode := func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	}

	state := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: bashible-api-server-tls
  namespace: %s
data:
  ca.crt: %s
  apiserver.crt: %s
  apiserver.key: %s
`, bashibleAPIServerNs, encode(cert.ca), encode(cert.crt), encode(cert.key))

	return bashibleAPIServerGenCertTestFixtures{
		cert:  cert,
		state: state,
	}
}

var (
	secretCreatedFixture = setupBashibleAPIServerCertHookTest(bashibleAPIServerCertFields{
		ca:  "a",
		crt: "b",
		key: "c",
	})

	secretChangedFixture = setupBashibleAPIServerCertHookTest(bashibleAPIServerCertFields{
		ca:  "x",
		crt: "y",
		key: "z",
	})
)

var _ = Describe("Node manager hooks :: bashible-apiserver :: gen webhook certs ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("For empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("executes successful with empty state", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("when secret created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(secretCreatedFixture.state))
				f.RunHook()
			})

			It("should store cert data in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				assertBashibleAPICertStoredValues(f, secretCreatedFixture.cert)
			})

			Context("when secret changed", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(secretChangedFixture.state))
					f.RunHook()
				})

				It("should store new cert data in values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertBashibleAPICertStoredValues(f, secretChangedFixture.cert)
				})
			})
		})

		Context("when fire onBeforeHelm event", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("should generated and stored new cert data to values", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertExistsCertInValues(f)
			})

			It("stores server cert which is signed by ca for k8s service DNS", func() {
				Expect(f).To(ExecuteSuccessfully())

				certFields := assertExistsCertInValues(f)
				assertCaSignServerCert(certFields, fmt.Sprintf("bashible-api.%s.svc", bashibleAPIServerNs))
			})

		})
	})

	Context("For cluster with secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretCreatedFixture.state))
			f.RunHook()
		})

		It("should store cert data in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertBashibleAPICertStoredValues(f, secretCreatedFixture.cert)
		})

		Context("when delete secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})

			It("should generated and stored new cert data to values", func() {
				Expect(f).To(ExecuteSuccessfully())

				cert := assertExistsCertInValues(f)
				assertNotEqualsCerts(cert, secretCreatedFixture.cert)
			})

			It("stores server cert which is signed by ca for k8s service DNS", func() {
				Expect(f).To(ExecuteSuccessfully())

				certFields := assertExistsCertInValues(f)
				assertCaSignServerCert(certFields, fmt.Sprintf("bashible-api.%s.svc", bashibleAPIServerNs))
			})
		})
	})

})

func assertNotEqualsCerts(a bashibleAPIServerCertFields, b bashibleAPIServerCertFields) {
	Expect(a.ca).To(Not(Equal(b.ca)))
	Expect(a.crt).To(Not(Equal(b.crt)))
	Expect(a.key).To(Not(Equal(b.key)))
}

func assertBashibleAPICertStoredValues(f *HookExecutionConfig, cert bashibleAPIServerCertFields) {
	Expect(f).To(ExecuteSuccessfully())

	certFromValues := assertExistsCertInValues(f)

	Expect(certFromValues.ca).To(Equal(cert.ca))
	Expect(certFromValues.crt).To(Equal(cert.crt))
	Expect(certFromValues.key).To(Equal(cert.key))
}

func assertCaSignServerCert(certFields bashibleAPIServerCertFields, dnsName string) {
	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM([]byte(certFields.ca))
	Expect(ok).To(BeTrue())

	block, _ := pem.Decode([]byte(certFields.crt))
	Expect(block).ShouldNot(BeNil())

	cert, err := x509.ParseCertificate(block.Bytes)
	Expect(err).ShouldNot(HaveOccurred())

	opts := x509.VerifyOptions{
		DNSName: dnsName,
		Roots:   certPool,
	}

	_, err = cert.Verify(opts)
	Expect(err).ShouldNot(HaveOccurred())
}

func assertExistsCertInValues(f *HookExecutionConfig) bashibleAPIServerCertFields {
	ca := f.ValuesGet("nodeManager.internal.bashibleApiServerCA")
	crt := f.ValuesGet("nodeManager.internal.bashibleApiServerCrt")
	key := f.ValuesGet("nodeManager.internal.bashibleApiServerKey")

	Expect(ca.Exists()).To(BeTrue())
	Expect(crt.Exists()).To(BeTrue())
	Expect(key.Exists()).To(BeTrue())

	cert := bashibleAPIServerCertFields{
		ca:  ca.String(),
		crt: crt.String(),
		key: key.String(),
	}

	return cert
}
