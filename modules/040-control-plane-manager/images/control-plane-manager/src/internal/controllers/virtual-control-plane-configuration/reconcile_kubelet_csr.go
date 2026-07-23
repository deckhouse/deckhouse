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

package virtualcontrolplaneconfiguration

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const kubeletServingSignerName = "kubernetes.io/kubelet-serving"

var (
	kubeletServingRequiredUsages = []certv1.KeyUsage{
		certv1.UsageKeyEncipherment,
		certv1.UsageDigitalSignature,
		certv1.UsageServerAuth,
	}
	kubeletServingRequiredUsagesNoRSA = []certv1.KeyUsage{
		certv1.UsageDigitalSignature,
		certv1.UsageServerAuth,
	}
)

func (r *reconciler) reconcileTenantKubeletServingCSRs(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) error {
	clientset, _, err := r.tenantClients(ctx, vcp)
	if err != nil {
		return fmt.Errorf("build tenant clients: %w", err)
	}

	logger := log.FromContext(ctx)

	list, err := clientset.CertificatesV1().CertificateSigningRequests().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list tenant CSRs: %w", err)
	}

	for i := range list.Items {
		csr := &list.Items[i]
		if !isPendingKubeletServingCSR(csr) {
			continue
		}
		if err := validateKubeletServingCSR(csr); err != nil {
			logger.Info("skipping invalid kubelet-serving CSR", "csr", csr.Name, "reason", err.Error())
			continue
		}
		if err := approveKubeletServingCSR(ctx, clientset, csr); err != nil {
			logger.Error(err, "approve kubelet-serving CSR", "csr", csr.Name)
			continue
		}
		logger.Info("approved kubelet-serving CSR", "csr", csr.Name)
	}

	return nil
}

func isPendingKubeletServingCSR(csr *certv1.CertificateSigningRequest) bool {
	if csr.Spec.SignerName != kubeletServingSignerName {
		return false
	}
	if len(csr.Status.Certificate) != 0 {
		return false
	}
	for _, c := range csr.Status.Conditions {
		if c.Type == certv1.CertificateApproved || c.Type == certv1.CertificateDenied {
			return false
		}
	}
	return true
}

// validateKubeletServingCSR mirrors node-manager nodeServingCert function.
func validateKubeletServingCSR(csr *certv1.CertificateSigningRequest) error {
	block, _ := pem.Decode(csr.Spec.Request)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return fmt.Errorf("PEM block type must be CERTIFICATE REQUEST")
	}
	x509cr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse certificate request: %w", err)
	}

	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return fmt.Errorf("organization %v does not match [system:nodes]", x509cr.Subject.Organization)
	}
	if len(x509cr.IPAddresses)+len(x509cr.DNSNames) < 1 {
		return fmt.Errorf("no IPAddresses or DNSNames in SAN")
	}
	if len(x509cr.EmailAddresses) > 0 {
		return fmt.Errorf("EmailAddresses present")
	}
	if len(x509cr.URIs) > 0 {
		return fmt.Errorf("URIs present")
	}
	if !hasExactUsages(csr, kubeletServingRequiredUsages) && !hasExactUsages(csr, kubeletServingRequiredUsagesNoRSA) {
		return fmt.Errorf("usages %v do not match kubelet-serving set", csr.Spec.Usages)
	}
	if !strings.HasPrefix(x509cr.Subject.CommonName, "system:node:") {
		return fmt.Errorf("CN %q does not start with system:node:", x509cr.Subject.CommonName)
	}
	if csr.Spec.Username != x509cr.Subject.CommonName {
		return fmt.Errorf("CSR username %q does not match CN %q", csr.Spec.Username, x509cr.Subject.CommonName)
	}
	return nil
}

func hasExactUsages(csr *certv1.CertificateSigningRequest, usages []certv1.KeyUsage) bool {
	if len(usages) != len(csr.Spec.Usages) {
		return false
	}
	want := make(map[certv1.KeyUsage]struct{}, len(usages))
	for _, u := range usages {
		want[u] = struct{}{}
	}
	for _, u := range csr.Spec.Usages {
		if _, ok := want[u]; !ok {
			return false
		}
	}
	return true
}

func approveKubeletServingCSR(ctx context.Context, clientset kubernetes.Interface, csr *certv1.CertificateSigningRequest) error {
	csr.Status.Conditions = append(csr.Status.Conditions, certv1.CertificateSigningRequestCondition{
		Type:    certv1.CertificateApproved,
		Status:  corev1.ConditionTrue,
		Reason:  "AutoApprovedByVirtualControlPlaneManager",
		Message: "autoapproved by virtual-control-plane-manager",
	})
	_, err := clientset.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csr.Name, csr, metav1.UpdateOptions{})
	return err
}
