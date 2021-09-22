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

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	cr "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	stateSecretExist = `
apiVersion: v1
kind: Secret
metadata:
  name: vpa-tls-certs
  namespace: kube-system
data:
  caCert.pem: YQo=
  caKey.pem: Ygo=
  serverCert.pem: %s
  serverKey.pem: Ywo=
`
)

var _ = Describe("Vertical Pod Autoscaler hooks :: order certificate ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Cert data must be created and stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("verticalPodAutoscaler.internal.CACert")).ShouldNot(BeNil())
			Expect(f.ValuesGet("verticalPodAutoscaler.internal.CAKey")).ShouldNot(BeNil())
			Expect(f.ValuesGet("verticalPodAutoscaler.internal.ServerCert")).ShouldNot(BeNil())
			Expect(f.ValuesGet("verticalPodAutoscaler.internal.ServerKey")).ShouldNot(BeNil())
		})
	})

	Context("Cluster with expired secret", func() {
		var serverCert string
		BeforeEach(func() {
			priv, err := ecdsa.GenerateKey(elliptic.P256(), cr.Reader)
			if err != nil {
				panic(err)
			}
			template := x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					Organization: []string{"Acme Co"},
				},
				NotBefore: time.Now().Add(-time.Hour * 24 * 180),
				NotAfter:  time.Now().Add(-time.Hour * 24),

				KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				BasicConstraintsValid: true,
			}

			derBytes, err := x509.CreateCertificate(cr.Reader, &template, &template, &priv.PublicKey, priv)
			if err != nil {
				panic(err)
			}
			out := &bytes.Buffer{}
			err = pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
			if err != nil {
				panic(err)
			}
			serverCert = base64.StdEncoding.EncodeToString(out.Bytes())

			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(stateSecretExist, serverCert)))
			f.RunHook()
		})

		It("Cert data must be generated and stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("verticalPodAutoscaler.internal.CACert").String()).NotTo(Equal("a\n"))
			Expect(f.ValuesGet("verticalPodAutoscaler.internal.CAKey").String()).NotTo(Equal("b\n"))
			Expect(f.ValuesGet("verticalPodAutoscaler.internal.ServerCert").String()).ShouldNot(BeNil())
			Expect(f.ValuesGet("verticalPodAutoscaler.internal.ServerKey").String()).ShouldNot(BeNil())
		})
	})
})
