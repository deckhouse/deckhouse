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
	"context"

	"github.com/cloudflare/cfssl/csr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func newKubeCSR(org string, cn string, dnsNames []string, ipAddresses []string, usages []cv1.KeyUsage) *cv1.CertificateSigningRequest {
	csrPEM, _, _ := certificate.GenerateCSR(nil, cn, certificate.WithNames(csr.Name{O: org}), certificate.WithCSRKeyRequest(&csr.KeyRequest{A: "rsa", S: 2048}), certificate.WithSANs(dnsNames...), certificate.WithSANs(ipAddresses...))

	return &cv1.CertificateSigningRequest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CertificateSigningRequest",
			APIVersion: "certificates.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubelet-csr",
		},
		Spec: cv1.CertificateSigningRequestSpec{
			Username:   "system:node:dev-master-0",
			SignerName: "kubernetes.io/kubelet-serving",
			Request:    csrPEM,
			Usages:     usages,
			Groups:     []string{"system:nodes", "system:authenticated"},
		},
	}
}

var _ = Describe("Modules :: nodeManager :: hooks :: kubelet_csr_approver ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with proper csr, with IPAddresses and DNSNames", func() {
		BeforeEach(func() {
			csrUsages := []cv1.KeyUsage{cv1.UsageDigitalSignature, cv1.UsageKeyEncipherment, cv1.UsageServerAuth}
			csr := newKubeCSR("system:nodes", "system:node:dev-master-0", []string{"node1"}, []string{"1.2.3.4"}, csrUsages)
			csrYaml, _ := yaml.Marshal(csr)

			f.BindingContexts.Set(f.KubeStateSet(string(csrYaml)))
			_, _ = f.KubeClient().CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Must be executed successfully and approve csr", func() {
			Expect(f).To(ExecuteSuccessfully())
			csr, err := f.KubeClient().CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "kubelet-csr", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(csr.Status.Conditions[0].Type).To(Equal(cv1.CertificateApproved))
		})
	})

	Context("Cluster with proper csr (without UsageKeyEncipherment), with IPAddresses and DNSNames", func() {
		BeforeEach(func() {
			csrUsages := []cv1.KeyUsage{cv1.UsageDigitalSignature, cv1.UsageServerAuth}
			csr := newKubeCSR("system:nodes", "system:node:dev-master-0", []string{"node1"}, []string{"1.2.3.4"}, csrUsages)
			csrYaml, _ := yaml.Marshal(csr)

			f.BindingContexts.Set(f.KubeStateSet(string(csrYaml)))
			_, _ = f.KubeClient().CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Must be executed successfully and approve csr", func() {
			Expect(f).To(ExecuteSuccessfully())
			csr, err := f.KubeClient().CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "kubelet-csr", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(csr.Status.Conditions[0].Type).To(Equal(cv1.CertificateApproved))
		})
	})

	Context("Cluster with proper csr, with IPAddresses and without DNSNames", func() {
		BeforeEach(func() {
			csrUsages := []cv1.KeyUsage{cv1.UsageDigitalSignature, cv1.UsageKeyEncipherment, cv1.UsageServerAuth}
			csr := newKubeCSR("system:nodes", "system:node:dev-master-0", nil, []string{"1.2.3.4"}, csrUsages)
			csrYaml, _ := yaml.Marshal(csr)

			f.BindingContexts.Set(f.KubeStateSet(string(csrYaml)))
			_, _ = f.KubeClient().CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Must be executed successfully and approve csr", func() {
			Expect(f).To(ExecuteSuccessfully())
			csr, err := f.KubeClient().CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "kubelet-csr", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(csr.Status.Conditions[0].Type).To(Equal(cv1.CertificateApproved))
		})
	})

	Context("Cluster with proper csr, without IPAddresses and with DNSNames", func() {
		BeforeEach(func() {
			csrUsages := []cv1.KeyUsage{cv1.UsageDigitalSignature, cv1.UsageKeyEncipherment, cv1.UsageServerAuth}
			csr := newKubeCSR("system:nodes", "system:node:dev-master-0", []string{"foobar"}, nil, csrUsages)
			csrYaml, _ := yaml.Marshal(csr)

			f.BindingContexts.Set(f.KubeStateSet(string(csrYaml)))
			_, _ = f.KubeClient().CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Must be executed successfully and approve csr", func() {
			Expect(f).To(ExecuteSuccessfully())
			csr, err := f.KubeClient().CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "kubelet-csr", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(csr.Status.Conditions[0].Type).To(Equal(cv1.CertificateApproved))
		})
	})

	Context("Cluster with wrong csr (Organization must match 'system:nodes')", func() {
		BeforeEach(func() {
			csrUsages := []cv1.KeyUsage{cv1.UsageDigitalSignature, cv1.UsageKeyEncipherment, cv1.UsageServerAuth}
			csr := newKubeCSR("foobar", "system:node:dev-master-0", []string{"foobar"}, []string{"1.2.3.4"}, csrUsages)
			csrYaml, _ := yaml.Marshal(csr)

			f.BindingContexts.Set(f.KubeStateSet(string(csrYaml)))
			_, _ = f.KubeClient().CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Must be executed successfully and don't approve csr", func() {
			Expect(f).To(ExecuteSuccessfully())
			csr, err := f.KubeClient().CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "kubelet-csr", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(csr.Status.Conditions).To(BeNil())
		})
	})

	Context("Cluster with wrong csr (CommonName must start with 'system:node:')", func() {
		BeforeEach(func() {
			csrUsages := []cv1.KeyUsage{cv1.UsageDigitalSignature, cv1.UsageKeyEncipherment, cv1.UsageServerAuth}
			csr := newKubeCSR("system:nodes", "dev-master-0", []string{"foobar"}, []string{"1.2.3.4"}, csrUsages)
			csrYaml, _ := yaml.Marshal(csr)

			f.BindingContexts.Set(f.KubeStateSet(string(csrYaml)))
			_, _ = f.KubeClient().CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Must be executed successfully and don't approve csr", func() {
			Expect(f).To(ExecuteSuccessfully())
			csr, err := f.KubeClient().CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "kubelet-csr", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(csr.Status.Conditions).To(BeNil())
		})
	})
})
