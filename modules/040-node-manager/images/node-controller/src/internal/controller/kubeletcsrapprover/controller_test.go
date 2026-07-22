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

package kubeletcsrapprover

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"testing"

	cv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/register"
)

const testNodeUser = "system:node:dev-master-0"

var rsaUsages = []cv1.KeyUsage{cv1.UsageDigitalSignature, cv1.UsageKeyEncipherment, cv1.UsageServerAuth}

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := cv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add certificates scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func csrPEM(t *testing.T, org, cn string, dnsNames, ips []string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: cn}}
	if org != "" {
		tmpl.Subject.Organization = []string{org}
	}
	tmpl.DNSNames = dnsNames
	for _, ip := range ips {
		tmpl.IPAddresses = append(tmpl.IPAddresses, net.ParseIP(ip))
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		t.Fatalf("create csr: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
}

func newCSR(name, signer, username string, usages []cv1.KeyUsage, request []byte) *cv1.CertificateSigningRequest {
	return &cv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: cv1.CertificateSigningRequestSpec{
			Username:   username,
			SignerName: signer,
			Request:    request,
			Usages:     usages,
		},
	}
}

func doReconcile(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
}

func approved(t *testing.T, r *Reconciler, name string) bool {
	t.Helper()
	csr := &cv1.CertificateSigningRequest{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, csr); err != nil {
		t.Fatalf("get csr %s: %v", name, err)
	}
	for _, c := range csr.Status.Conditions {
		if c.Type == cv1.CertificateApproved {
			return true
		}
	}
	return false
}

func TestReconcile_NonExistent_NoError(t *testing.T) {
	r := newReconciler(t)
	doReconcile(t, r, "missing")
}

func TestReconcile_ValidServingCSR_RSAUsages_Approves(t *testing.T) {
	req := csrPEM(t, "system:nodes", testNodeUser, []string{"node1"}, []string{"1.2.3.4"})
	r := newReconciler(t, newCSR("csr-1", signerNameKubeletServing, testNodeUser, rsaUsages, req))
	doReconcile(t, r, "csr-1")

	if !approved(t, r, "csr-1") {
		t.Fatal("valid kubelet-serving CSR must be approved")
	}
}

func TestReconcile_ValidServingCSR_NoRSAUsages_Approves(t *testing.T) {
	usages := []cv1.KeyUsage{cv1.UsageDigitalSignature, cv1.UsageServerAuth}
	req := csrPEM(t, "system:nodes", testNodeUser, []string{"node1"}, []string{"1.2.3.4"})
	r := newReconciler(t, newCSR("csr-1", signerNameKubeletServing, testNodeUser, usages, req))
	doReconcile(t, r, "csr-1")

	if !approved(t, r, "csr-1") {
		t.Fatal("valid CSR without KeyEncipherment must be approved")
	}
}

func TestReconcile_ValidServingCSR_IPOnly_Approves(t *testing.T) {
	req := csrPEM(t, "system:nodes", testNodeUser, nil, []string{"1.2.3.4"})
	r := newReconciler(t, newCSR("csr-1", signerNameKubeletServing, testNodeUser, rsaUsages, req))
	doReconcile(t, r, "csr-1")

	if !approved(t, r, "csr-1") {
		t.Fatal("CSR with only IP SAN must be approved")
	}
}

func TestReconcile_ValidServingCSR_DNSOnly_Approves(t *testing.T) {
	req := csrPEM(t, "system:nodes", testNodeUser, []string{"foobar"}, nil)
	r := newReconciler(t, newCSR("csr-1", signerNameKubeletServing, testNodeUser, rsaUsages, req))
	doReconcile(t, r, "csr-1")

	if !approved(t, r, "csr-1") {
		t.Fatal("CSR with only DNS SAN must be approved")
	}
}

func TestReconcile_WrongOrg_NotApproved(t *testing.T) {
	req := csrPEM(t, "foobar", testNodeUser, []string{"foobar"}, []string{"1.2.3.4"})
	r := newReconciler(t, newCSR("csr-1", signerNameKubeletServing, testNodeUser, rsaUsages, req))
	doReconcile(t, r, "csr-1")

	if approved(t, r, "csr-1") {
		t.Fatal("CSR with organization != system:nodes must not be approved")
	}
}

func TestReconcile_WrongCN_NotApproved(t *testing.T) {
	// CN without the system:node: prefix; username still valid so only the CN check fails.
	req := csrPEM(t, "system:nodes", "dev-master-0", []string{"foobar"}, []string{"1.2.3.4"})
	r := newReconciler(t, newCSR("csr-1", signerNameKubeletServing, testNodeUser, rsaUsages, req))
	doReconcile(t, r, "csr-1")

	if approved(t, r, "csr-1") {
		t.Fatal("CSR whose CN does not start with system:node: must not be approved")
	}
}

func TestReconcile_UsernameMismatch_NotApproved(t *testing.T) {
	req := csrPEM(t, "system:nodes", testNodeUser, []string{"node1"}, []string{"1.2.3.4"})
	r := newReconciler(t, newCSR("csr-1", signerNameKubeletServing, "system:node:other", rsaUsages, req))
	doReconcile(t, r, "csr-1")

	if approved(t, r, "csr-1") {
		t.Fatal("CSR whose username does not match the CN must not be approved")
	}
}

func TestReconcile_WrongUsages_NotApproved(t *testing.T) {
	usages := []cv1.KeyUsage{cv1.UsageDigitalSignature}
	req := csrPEM(t, "system:nodes", testNodeUser, []string{"node1"}, []string{"1.2.3.4"})
	r := newReconciler(t, newCSR("csr-1", signerNameKubeletServing, testNodeUser, usages, req))
	doReconcile(t, r, "csr-1")

	if approved(t, r, "csr-1") {
		t.Fatal("CSR with an unexpected usage set must not be approved")
	}
}

func TestReconcile_UnparseableRequest_NotApproved(t *testing.T) {
	r := newReconciler(t, newCSR("csr-1", signerNameKubeletServing, testNodeUser, rsaUsages, []byte("not a pem")))
	doReconcile(t, r, "csr-1")

	if approved(t, r, "csr-1") {
		t.Fatal("CSR with an unparseable request must not be approved")
	}
}

// Parity quirk: a non-kubelet-serving signer skips serving validation and is
// approved as soon as its PEM parses (matches the original hook behaviour).
func TestReconcile_NonKubeletServingSigner_ApprovedOnParse(t *testing.T) {
	req := csrPEM(t, "foobar", "whoever", nil, nil)
	r := newReconciler(t, newCSR("csr-1", "example.com/other", "whoever", nil, req))
	doReconcile(t, r, "csr-1")

	if !approved(t, r, "csr-1") {
		t.Fatal("a parseable non-kubelet-serving CSR must be approved (hook parity)")
	}
}

func TestReconcile_AlreadyApproved_Skipped(t *testing.T) {
	req := csrPEM(t, "system:nodes", testNodeUser, []string{"node1"}, []string{"1.2.3.4"})
	csr := newCSR("csr-1", signerNameKubeletServing, testNodeUser, rsaUsages, req)
	csr.Status.Conditions = []cv1.CertificateSigningRequestCondition{{
		Type:   cv1.CertificateApproved,
		Status: corev1.ConditionTrue,
		Reason: "SomethingElse",
	}}
	r := newReconciler(t, csr)
	doReconcile(t, r, "csr-1")

	got := &cv1.CertificateSigningRequest{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: "csr-1"}, got); err != nil {
		t.Fatalf("get csr: %v", err)
	}
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != "SomethingElse" {
		t.Fatalf("already-approved CSR must be left untouched, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_AlreadyDenied_NotApproved(t *testing.T) {
	req := csrPEM(t, "system:nodes", testNodeUser, []string{"node1"}, []string{"1.2.3.4"})
	csr := newCSR("csr-1", signerNameKubeletServing, testNodeUser, rsaUsages, req)
	csr.Status.Conditions = []cv1.CertificateSigningRequestCondition{{
		Type:   cv1.CertificateDenied,
		Status: corev1.ConditionTrue,
	}}
	r := newReconciler(t, csr)
	doReconcile(t, r, "csr-1")

	if approved(t, r, "csr-1") {
		t.Fatal("a denied CSR must not be approved")
	}
}

func TestReconcile_AlreadyIssued_Skipped(t *testing.T) {
	req := csrPEM(t, "system:nodes", testNodeUser, []string{"node1"}, []string{"1.2.3.4"})
	csr := newCSR("csr-1", signerNameKubeletServing, testNodeUser, rsaUsages, req)
	csr.Status.Certificate = []byte("already-issued-cert")
	r := newReconciler(t, csr)
	doReconcile(t, r, "csr-1")

	if approved(t, r, "csr-1") {
		t.Fatal("an already-issued CSR must not get a new approval condition")
	}
}
