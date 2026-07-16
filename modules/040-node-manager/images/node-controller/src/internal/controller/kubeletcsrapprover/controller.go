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

// Package kubeletcsrapprover auto-approves kubelet-serving CertificateSigningRequests.
//
// When a kubelet rotates its serving certificate it submits a CSR signed by
// kubernetes.io/kubelet-serving that no built-in approver handles. This
// controller validates such a CSR (organization system:nodes, CN prefixed with
// system:node:, exact serving usages, an IP or DNS SAN, no email/URI SAN, and a
// username matching the CN) and, when it passes, appends the Approved condition
// via the approval subresource.
//
// This replaces the shell-operator hook hooks/kubelet_csr_approver.go with the
// same trigger (watch CSR) and effect (approve valid CSRs). Behaviour is kept
// identical to the hook, including the hook's quirk that a CSR whose signer is
// NOT kubernetes.io/kubelet-serving is approved as soon as its PEM parses,
// without further validation.
package kubeletcsrapprover

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"strings"

	cv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/deckhouse/node-controller/internal/register"
)

const (
	signerNameKubeletServing = "kubernetes.io/kubelet-serving"
	approvedReason           = "AutoApproved by node-manager/kubelet_csr_approver hook"
	approvedMessage          = "autoapproved by Deckhouse"
)

// https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/certificates/helpers.go#L85-L87
var kubeletServingRequiredUsages = []cv1.KeyUsage{
	cv1.UsageKeyEncipherment,
	cv1.UsageDigitalSignature,
	cv1.UsageServerAuth,
}

var kubeletServingRequiredUsagesNoRSA = []cv1.KeyUsage{
	cv1.UsageDigitalSignature,
	cv1.UsageServerAuth,
}

func init() {
	register.RegisterController("node-kubelet-csr-approver", &cv1.CertificateSigningRequest{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(_ register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	csr := &cv1.CertificateSigningRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, csr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Already issued: nothing to do.
	if len(csr.Status.Certificate) != 0 {
		return ctrl.Result{}, nil
	}

	// Already decided (approved or denied): leave it alone.
	for _, c := range csr.Status.Conditions {
		if c.Type == cv1.CertificateApproved || c.Type == cv1.CertificateDenied {
			return ctrl.Result{}, nil
		}
	}

	if err := validateCSR(csr); err != nil {
		logger.Info("csr not valid, skipping", "csr", csr.Name, "reason", err.Error())
		return ctrl.Result{}, nil
	}

	appendApprovalCondition(csr)
	if err := r.Client.SubResource("approval").Update(ctx, csr); err != nil {
		logger.Error(err, "failed to approve csr", "csr", csr.Name)
		return ctrl.Result{}, err
	}

	logger.Info("approved kubelet-serving csr", "csr", csr.Name)
	return ctrl.Result{}, nil
}

// validateCSR mirrors csrFilterFunc from the original hook: the PEM must parse,
// and a kubernetes.io/kubelet-serving CSR must additionally pass nodeServingCert.
// Any other signer is accepted once its PEM parses.
func validateCSR(csr *cv1.CertificateSigningRequest) error {
	x509cr, err := parseCSR(csr)
	if err != nil {
		return err
	}
	if csr.Spec.SignerName == signerNameKubeletServing {
		return nodeServingCert(csr, x509cr)
	}
	return nil
}

func appendApprovalCondition(csr *cv1.CertificateSigningRequest) {
	for _, cond := range csr.Status.Conditions {
		if cond.Type == cv1.CertificateApproved {
			return
		}
	}
	csr.Status.Conditions = append(csr.Status.Conditions, cv1.CertificateSigningRequestCondition{
		Type:    cv1.CertificateApproved,
		Status:  corev1.ConditionTrue,
		Reason:  approvedReason,
		Message: approvedMessage,
	})
}

func parseCSR(obj *cv1.CertificateSigningRequest) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(obj.Spec.Request)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("PEM block type must be CERTIFICATE REQUEST")
	}
	return x509.ParseCertificateRequest(block.Bytes)
}

func nodeServingCert(csr *cv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) error {
	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return fmt.Errorf("org does not match: %s", x509cr.Subject.Organization)
	}
	if len(x509cr.IPAddresses)+len(x509cr.DNSNames) < 1 {
		return fmt.Errorf("field IPAddresses or DNSNames must be set")
	}
	if len(x509cr.EmailAddresses) > 0 {
		return fmt.Errorf("field EmailAddresses is present")
	}
	if len(x509cr.URIs) > 0 {
		return fmt.Errorf("field URIs is present")
	}
	if !hasExactUsages(csr, kubeletServingRequiredUsages) && !hasExactUsages(csr, kubeletServingRequiredUsagesNoRSA) {
		return fmt.Errorf("usage does not match")
	}
	if !strings.HasPrefix(x509cr.Subject.CommonName, "system:node:") {
		return fmt.Errorf("CN does not start with 'system:node': %s", x509cr.Subject.CommonName)
	}
	if csr.Spec.Username != x509cr.Subject.CommonName {
		return fmt.Errorf("x509 CN %q doesn't match CSR username %q", x509cr.Subject.CommonName, csr.Spec.Username)
	}
	return nil
}

func hasExactUsages(csr *cv1.CertificateSigningRequest, usages []cv1.KeyUsage) bool {
	if len(usages) != len(csr.Spec.Usages) {
		return false
	}
	usageMap := map[cv1.KeyUsage]struct{}{}
	for _, u := range usages {
		usageMap[u] = struct{}{}
	}
	for _, u := range csr.Spec.Usages {
		if _, ok := usageMap[u]; !ok {
			return false
		}
	}
	return true
}
