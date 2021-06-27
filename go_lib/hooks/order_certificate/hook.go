/*
Copyright 2021 Flant CJSC

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

package order_certificate

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	csrutil "k8s.io/client-go/util/certificate/csr"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// certificateWaitTimeoutDefault controls default amount of time we wait for certificate
// approval in one iteration.
const certificateWaitTimeoutDefault = 1 * time.Minute

type CertificateSecret struct {
	Name string
	Crt  []byte
	Key  []byte
}

type CertificateInfo struct {
	Certificate        string `json:"certificate,omitempty"`
	Key                string `json:"key,omitempty"`
	CertificateUpdated bool   `json:"certificate_updated,omitempty"`
}

type OrderCertificateRequest struct {
	Namespace   string
	SecretName  string
	CommonName  string
	ValueName   string
	Group       string
	ModuleName  string
	WaitTimeout time.Duration
}

func ApplyCertificateSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	cc := &CertificateSecret{
		Name: secret.Name,
	}

	if tls, ok := secret.Data["tls.crt"]; ok {
		cc.Crt = tls
	} else if client, ok := secret.Data["client.crt"]; ok {
		cc.Crt = client
	}

	if tls, ok := secret.Data["tls.key"]; ok {
		cc.Key = tls
	} else if client, ok := secret.Data["client.key"]; ok {
		cc.Key = client
	}

	return cc, err
}

func RegisterOrderCertificateHook(requests []OrderCertificateRequest) bool {
	var namespaces []string
	var secretNames []string
	for _, request := range requests {
		namespaces = append(namespaces, request.Namespace)
		secretNames = append(secretNames, request.SecretName)
	}
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{
			Order: 5,
		},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:              "certificateSecrets",
				ApiVersion:        "v1",
				Kind:              "Secret",
				NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: namespaces}},
				NameSelector:      &types.NameSelector{MatchNames: secretNames},
				FilterFunc:        ApplyCertificateSecretFilter,
			},
		},
		Schedule: []go_hook.ScheduleConfig{
			{
				Name:    "certificateCheck",
				Crontab: "42 4 * * *",
			},
		},
	}, dependency.WithExternalDependencies(certificateHandler(requests)))
}

func certificateHandler(requests []OrderCertificateRequest) func(input *go_hook.HookInput, dc dependency.Container) error {

	return func(input *go_hook.HookInput, dc dependency.Container) error {
		for _, request := range requests {
			if snaps, ok := input.Snapshots["certificateSecrets"]; ok {
				var secret *CertificateSecret

				for _, snap := range snaps {
					snapSecret := snap.(*CertificateSecret)
					if snapSecret.Name == request.SecretName {
						secret = snapSecret
						break
					}
				}

				// If existing Certificate expires in more than 7 days - use it.
				if secret != nil && len(secret.Crt) > 0 && len(secret.Key) > 0 {
					shouldGenerateNewCert, err := certificate.IsCertificateExpiringSoon(string(secret.Crt), time.Hour*24*7)
					if err != nil {
						return err
					}
					if !shouldGenerateNewCert {
						info := CertificateInfo{Certificate: string(secret.Crt), Key: string(secret.Key)}
						input.Values.Set(fmt.Sprintf("%s.%s", request.ModuleName, request.ValueName), info)
						continue
					}
				}
			}

			err := issueCertificate(input, dc, request)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func issueCertificate(input *go_hook.HookInput, dc dependency.Container, request OrderCertificateRequest) error {
	if request.WaitTimeout == 0 {
		request.WaitTimeout = certificateWaitTimeoutDefault
	}

	k8, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("can't init Kubernetes client: %v", err)
	}

	// Delete existing CSR from the cluster.
	_ = k8.CertificatesV1beta1().CertificateSigningRequests().Delete(context.TODO(), request.CommonName, metav1.DeleteOptions{})

	csrPEM, key, err := certificate.GenerateCSR(input.LogEntry, request.CommonName, request.Group)
	if err != nil {
		return fmt.Errorf("error generating CSR: %v", err)
	}

	// Create new CSR in the cluster.
	csr := &certificatesv1beta1.CertificateSigningRequest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CertificateSigningRequest",
			APIVersion: "certificates.k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: request.CommonName,
		},
		Spec: certificatesv1beta1.CertificateSigningRequestSpec{
			Request: csrPEM,
			Usages: []certificatesv1beta1.KeyUsage{
				certificatesv1beta1.UsageDigitalSignature,
				certificatesv1beta1.UsageKeyEncipherment,
				certificatesv1beta1.UsageClientAuth,
			},
		},
	}

	// Create CSR.
	req, err := k8.CertificatesV1beta1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating CertificateSigningRequest: %v", err)
	}

	// Add CSR approved status.
	csr.Status.Conditions = append(csr.Status.Conditions,
		certificatesv1beta1.CertificateSigningRequestCondition{
			Type:           certificatesv1beta1.CertificateApproved,
			Reason:         "HookApprove",
			Message:        "This CSR was approved by a hook.",
			LastUpdateTime: metav1.Now(),
		})
	_, err = k8.CertificatesV1beta1().CertificateSigningRequests().UpdateStatus(context.TODO(), csr, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating status of CertificateSigningRequest: %v", err)
	}

	// Approve CSR.
	_, err = k8.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(context.TODO(), csr, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error approving of CertificateSigningRequest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), request.WaitTimeout)
	defer cancel()

	crtPEM, err := csrutil.WaitForCertificate(ctx, k8, req.Name, req.UID)
	if err != nil {
		return fmt.Errorf("%s CertificateSigningRequest was not signed: %v", request.CommonName, err)
	}

	// Delete CSR.
	_ = k8.CertificatesV1beta1().CertificateSigningRequests().Delete(context.TODO(), request.CommonName, metav1.DeleteOptions{})

	info := CertificateInfo{Certificate: string(crtPEM), Key: string(key), CertificateUpdated: true}
	input.Values.Set(fmt.Sprintf("%s.%s", request.ModuleName, request.ValueName), info)

	return nil
}
