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
Hook based on https://github.com/kontena/kubelet-rubber-stamp/blob/master/pkg/controller/certificatesigningrequest/certificatesigningrequest_controller.go
and kubernetes recommendations - https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/#certificate-rotation
*/

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	cv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

type CsrInfo struct {
	Name   string
	Valid  bool
	ErrMsg string
}

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

func csrFilterFunc(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	csr := &cv1.CertificateSigningRequest{}
	err := sdk.FromUnstructured(obj, csr)
	if err != nil {
		return nil, err
	}

	// CSR already has a certificate, ignoring
	if len(csr.Status.Certificate) != 0 {
		return nil, nil
	}

	// CSR already has a approval status
	for _, c := range csr.Status.Conditions {
		if c.Type == cv1.CertificateApproved {
			return nil, nil
		}
		if c.Type == cv1.CertificateDenied {
			return nil, nil
		}
	}

	ret := &CsrInfo{
		Name: csr.GetName(),
	}

	// Parse CSR
	x509cr, err := parseCSR(csr)
	if err != nil {
		ret.ErrMsg = err.Error()
		return ret, nil
	}

	if csr.Spec.SignerName == "kubernetes.io/kubelet-serving" {
		err = nodeServingCert(csr, x509cr)
		if err != nil {
			ret.ErrMsg = err.Error()
			return ret, nil
		}
	}

	ret.Valid = true
	return ret, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/kubelet_csr_approver",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "csr",
			ApiVersion:             "certificates.k8s.io/v1",
			Kind:                   "CertificateSigningRequest",
			FilterFunc:             csrFilterFunc,
			WaitForSynchronization: go_hook.Bool(false),
		},
	},
}, dependency.WithExternalDependencies(csrHandler))

func csrHandler(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	k8sCli, err := dc.GetK8sClient()
	if err != nil {
		return err
	}
	snaps := input.Snapshots.Get("csr")
	for csrInfo, err := range sdkobjectpatch.SnapshotIter[CsrInfo](snaps) {
		if err != nil {
			return fmt.Errorf("failted to iterate over 'csr' snapshots: %w", err)
		}

		if !csrInfo.Valid {
			input.Logger.Warn("csr info not valid", slog.String(csrInfo.Name, csrInfo.ErrMsg))
			continue
		}

		csr, err := k8sCli.CertificatesV1().CertificateSigningRequests().Get(context.TODO(), csrInfo.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		appendApprovalCondition(csr)

		_, err = k8sCli.CertificatesV1().CertificateSigningRequests().UpdateApproval(context.TODO(), csr.Name, csr, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
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

func appendApprovalCondition(csr *cv1.CertificateSigningRequest) {
	for _, cond := range csr.Status.Conditions {
		if cond.Type == cv1.CertificateApproved {
			return
		}
	}
	csr.Status.Conditions = append(csr.Status.Conditions, cv1.CertificateSigningRequestCondition{
		Type:    cv1.CertificateApproved,
		Status:  corev1.ConditionTrue,
		Reason:  "AutoApproved by node-manager/kubelet_csr_approver hook",
		Message: "autoapproved by Deckhouse",
	})
}

func parseCSR(obj *cv1.CertificateSigningRequest) (*x509.CertificateRequest, error) {
	// extract PEM from request object
	pemBytes := obj.Spec.Request
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("PEM block type must be CERTIFICATE REQUEST")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, err
	}
	return csr, nil
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
