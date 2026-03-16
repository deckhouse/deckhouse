//go:build ai_tests

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

package csr

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = certificatesv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	return s
}

// generateCSRPEM creates a PEM-encoded CERTIFICATE REQUEST with the given parameters.
func generateCSRPEM(t *testing.T, cn string, org []string, dnsNames []string, ips []net.IP) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: org,
		},
		DNSNames:    dnsNames,
		IPAddresses: ips,
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
}

func buildCSR(name string, signerName string, username string, usages []certificatesv1.KeyUsage, groups []string, csrPEM []byte) *certificatesv1.CertificateSigningRequest {
	return &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:    csrPEM,
			SignerName: signerName,
			Usages:     usages,
			Username:   username,
			Groups:     groups,
		},
	}
}

func reconcileCSR(t *testing.T, csr *certificatesv1.CertificateSigningRequest) *certificatesv1.CertificateSigningRequest {
	t.Helper()
	s := newScheme()
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(csr).
		WithStatusSubresource(csr).
		Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: csr.Name},
	})
	require.NoError(t, err)

	got := &certificatesv1.CertificateSigningRequest{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: csr.Name}, got))
	return got
}

func TestAI_CSRApprover_ApproveValidCSR_WithIPAndDNS(t *testing.T) {
	csrPEM := generateCSRPEM(t, "system:node:dev-master-0", []string{"system:nodes"}, []string{"node1"}, []net.IP{net.ParseIP("1.2.3.4")})
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "system:node:dev-master-0",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageKeyEncipherment, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)

	got := reconcileCSR(t, csr)

	require.NotEmpty(t, got.Status.Conditions, "CSR should have been approved")
	assert.Equal(t, certificatesv1.CertificateApproved, got.Status.Conditions[0].Type)
	assert.Equal(t, corev1.ConditionTrue, got.Status.Conditions[0].Status)
}

func TestAI_CSRApprover_ApproveValidCSR_NoRSAUsages(t *testing.T) {
	csrPEM := generateCSRPEM(t, "system:node:dev-master-0", []string{"system:nodes"}, []string{"node1"}, []net.IP{net.ParseIP("1.2.3.4")})
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "system:node:dev-master-0",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)

	got := reconcileCSR(t, csr)

	require.NotEmpty(t, got.Status.Conditions, "CSR with non-RSA usages should be approved")
	assert.Equal(t, certificatesv1.CertificateApproved, got.Status.Conditions[0].Type)
}

func TestAI_CSRApprover_ApproveValidCSR_IPOnly(t *testing.T) {
	csrPEM := generateCSRPEM(t, "system:node:dev-master-0", []string{"system:nodes"}, nil, []net.IP{net.ParseIP("1.2.3.4")})
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "system:node:dev-master-0",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageKeyEncipherment, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)

	got := reconcileCSR(t, csr)

	require.NotEmpty(t, got.Status.Conditions, "CSR with only IP addresses should be approved")
	assert.Equal(t, certificatesv1.CertificateApproved, got.Status.Conditions[0].Type)
}

func TestAI_CSRApprover_ApproveValidCSR_DNSOnly(t *testing.T) {
	csrPEM := generateCSRPEM(t, "system:node:dev-master-0", []string{"system:nodes"}, []string{"foobar"}, nil)
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "system:node:dev-master-0",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageKeyEncipherment, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)

	got := reconcileCSR(t, csr)

	require.NotEmpty(t, got.Status.Conditions, "CSR with only DNS names should be approved")
	assert.Equal(t, certificatesv1.CertificateApproved, got.Status.Conditions[0].Type)
}

func TestAI_CSRApprover_RejectWrongOrg(t *testing.T) {
	csrPEM := generateCSRPEM(t, "system:node:dev-master-0", []string{"foobar"}, []string{"node1"}, []net.IP{net.ParseIP("1.2.3.4")})
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "system:node:dev-master-0",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageKeyEncipherment, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)

	got := reconcileCSR(t, csr)

	assert.Empty(t, got.Status.Conditions, "CSR with wrong org should NOT be approved")
}

func TestAI_CSRApprover_RejectWrongCN(t *testing.T) {
	csrPEM := generateCSRPEM(t, "dev-master-0", []string{"system:nodes"}, []string{"foobar"}, []net.IP{net.ParseIP("1.2.3.4")})
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "dev-master-0",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageKeyEncipherment, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)

	got := reconcileCSR(t, csr)

	assert.Empty(t, got.Status.Conditions, "CSR with wrong CN (no system:node: prefix) should NOT be approved")
}

func TestAI_CSRApprover_SkipAlreadyApproved(t *testing.T) {
	csrPEM := generateCSRPEM(t, "system:node:dev-master-0", []string{"system:nodes"}, []string{"node1"}, []net.IP{net.ParseIP("1.2.3.4")})
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "system:node:dev-master-0",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageKeyEncipherment, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)
	csr.Status.Conditions = []certificatesv1.CertificateSigningRequestCondition{
		{
			Type:   certificatesv1.CertificateApproved,
			Status: corev1.ConditionTrue,
			Reason: "AlreadyApproved",
		},
	}

	// Filtering of already-approved CSRs is done in SetupForPredicates, not in Reconcile.
	r := &Reconciler{}
	preds := r.SetupForPredicates()
	require.Len(t, preds, 1)

	assert.False(t, preds[0].Create(event.CreateEvent{Object: csr}),
		"predicate should reject already-approved CSR")
	assert.False(t, preds[0].Update(event.UpdateEvent{ObjectNew: csr}),
		"predicate should reject already-approved CSR on update")
}

func TestAI_CSRApprover_SkipCSRWithCertificate(t *testing.T) {
	csrPEM := generateCSRPEM(t, "system:node:dev-master-0", []string{"system:nodes"}, []string{"node1"}, []net.IP{net.ParseIP("1.2.3.4")})
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "system:node:dev-master-0",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageKeyEncipherment, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)
	csr.Status.Certificate = []byte("some-cert-data")

	// Filtering of CSRs with certificates is done in SetupForPredicates, not in Reconcile.
	r := &Reconciler{}
	preds := r.SetupForPredicates()
	require.Len(t, preds, 1)

	assert.False(t, preds[0].Create(event.CreateEvent{Object: csr}),
		"predicate should reject CSR with certificate already issued")
	assert.False(t, preds[0].Update(event.UpdateEvent{ObjectNew: csr}),
		"predicate should reject CSR with certificate already issued on update")
}

func TestAI_CSRApprover_NotFoundCSR(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()

	r := &Reconciler{}
	r.Client = c

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestAI_CSRApprover_RejectUsernameMismatch(t *testing.T) {
	csrPEM := generateCSRPEM(t, "system:node:dev-master-0", []string{"system:nodes"}, []string{"node1"}, []net.IP{net.ParseIP("1.2.3.4")})
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "system:node:different-node",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageKeyEncipherment, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)

	got := reconcileCSR(t, csr)

	assert.Empty(t, got.Status.Conditions, "CSR with username/CN mismatch should NOT be approved")
}

func TestAI_CSRApprover_RejectNoIPOrDNS(t *testing.T) {
	csrPEM := generateCSRPEM(t, "system:node:dev-master-0", []string{"system:nodes"}, nil, nil)
	csr := buildCSR("kubelet-csr", "kubernetes.io/kubelet-serving", "system:node:dev-master-0",
		[]certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature, certificatesv1.UsageKeyEncipherment, certificatesv1.UsageServerAuth},
		[]string{"system:nodes", "system:authenticated"}, csrPEM)

	got := reconcileCSR(t, csr)

	assert.Empty(t, got.Status.Conditions, "CSR with no IP or DNS should NOT be approved")
}
