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

package tls_certificate

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	certificatesv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	csrutil "k8s.io/client-go/util/certificate/csr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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
	Namespace  string
	SecretName string
	CommonName string
	SANs       []string
	Groups     []string
	Usages     []certificatesv1.KeyUsage
	SignerName string

	ValueName   string
	ModuleName  string
	WaitTimeout time.Duration

	ExpirationSeconds *int32
}

func (r *OrderCertificateRequest) DeepCopy() OrderCertificateRequest {
	newR := OrderCertificateRequest{
		Namespace:  r.Namespace,
		SecretName: r.SecretName,
		CommonName: r.CommonName,
		SignerName: r.SignerName,
		SANs:       append(make([]string, 0, len(r.SANs)), r.SANs...),
		Groups:     append(make([]string, 0, len(r.Groups)), r.Groups...),
		Usages:     append(make([]certificatesv1.KeyUsage, 0, len(r.Usages)), r.Usages...),

		ValueName:   r.ValueName,
		ModuleName:  r.ModuleName,
		WaitTimeout: r.WaitTimeout,
	}
	return newR
}

func ParseSecret(secret *v1.Secret) *CertificateSecret {
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

	return cc
}

func ApplyCertificateSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	cc := ParseSecret(secret)

	return cc, err
}

func RegisterOrderCertificateHook(requests []OrderCertificateRequest) bool {
	namespaces := make([]string, 0, len(requests))
	secretNames := make([]string, 0, len(requests))

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

func certificateHandler(requests []OrderCertificateRequest) func(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	return func(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
		return certificateHandlerWithRequests(ctx, input, dc, requests)
	}
}

func certificateHandlerWithRequests(_ context.Context, input *go_hook.HookInput, dc dependency.Container, requests []OrderCertificateRequest) error {
	publicDomain := input.Values.Get("global.modules.publicDomainTemplate").String()
	clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()

	for _, originalRequest := range requests {
		request := originalRequest.DeepCopy()

		// Convert cluster domain and public domain sans
		for index, san := range request.SANs {
			switch {
			case strings.HasPrefix(san, publicDomainPrefix) && publicDomain != "":
				request.SANs[index] = getPublicDomainSAN(san, publicDomain)

			case strings.HasPrefix(san, clusterDomainPrefix) && clusterDomain != "":
				request.SANs[index] = getClusterDomainSAN(san, clusterDomain)
			}
		}

		valueName := fmt.Sprintf("%s.%s", request.ModuleName, request.ValueName)
		secrets, err := sdkobjectpatch.UnmarshalToStruct[CertificateSecret](input.Snapshots, "certificateSecrets")
		if err != nil {
			return fmt.Errorf("failed to unmarshal certificateSecrets snapshot: %w", err)
		}
		if len(secrets) != 0 {
			var secret CertificateSecret

			for _, secretSnap := range secrets {
				if secretSnap.Name == request.SecretName {
					secret = secretSnap
					break
				}
			}

			if len(secret.Crt) > 0 && len(secret.Key) > 0 {
				// Check that certificate is not expired and has the same order request
				genNew, err := shouldGenerateNewCert(secret.Crt, request, time.Hour*24*15)
				if err != nil {
					return err
				}
				if !genNew {
					info := CertificateInfo{Certificate: string(secret.Crt), Key: string(secret.Key)}
					input.Values.Set(valueName, info)
					continue
				}
			}
		}

		info, err := IssueCertificate(input, dc, request)
		if err != nil {
			return err
		}
		input.Values.Set(valueName, info)
	}
	return nil
}

func IssueCertificate(input *go_hook.HookInput, dc dependency.Container, request OrderCertificateRequest) (*CertificateInfo, error) {
	k8, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("can't init Kubernetes client: %v", err)
	}

	if request.WaitTimeout == 0 {
		request.WaitTimeout = certificateWaitTimeoutDefault
	}

	if len(request.Usages) == 0 {
		request.Usages = []certificatesv1.KeyUsage{
			certificatesv1.UsageDigitalSignature,
			certificatesv1.UsageKeyEncipherment,
			certificatesv1.UsageClientAuth,
		}
	}

	if request.SignerName == "" {
		request.SignerName = certificatesv1.KubeAPIServerClientSignerName
	}

	// Delete existing CSR from the cluster.
	_ = k8.CertificatesV1().CertificateSigningRequests().Delete(context.TODO(), request.CommonName, metav1.DeleteOptions{})

	csrPEM, key, err := certificate.GenerateCSR(input.Logger, request.CommonName,
		certificate.WithGroups(request.Groups...),
		certificate.WithSANs(request.SANs...))
	if err != nil {
		return nil, fmt.Errorf("error generating CSR: %v", err)
	}

	// Create new CSR in the cluster.
	csr := &certificatesv1.CertificateSigningRequest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CertificateSigningRequest",
			APIVersion: "certificates.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: request.CommonName,
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:           csrPEM,
			Usages:            request.Usages,
			SignerName:        request.SignerName,
			ExpirationSeconds: request.ExpirationSeconds,
		},
	}

	// Create CSR.
	req, err := k8.CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error creating CertificateSigningRequest: %v", err)
	}

	// Add CSR approved status.
	csr.Status.Conditions = append(csr.Status.Conditions,
		certificatesv1.CertificateSigningRequestCondition{
			Type:    certificatesv1.CertificateApproved,
			Status:  v1.ConditionTrue,
			Reason:  "HookApprove",
			Message: "This CSR was approved by a hook.",
		})

	// Approve CSR.
	_, err = k8.CertificatesV1().CertificateSigningRequests().UpdateApproval(context.TODO(), request.CommonName, csr, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error approving of CertificateSigningRequest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), request.WaitTimeout)
	defer cancel()

	crtPEM, err := csrutil.WaitForCertificate(ctx, k8, req.Name, req.UID)
	if err != nil {
		return nil, fmt.Errorf("%s CertificateSigningRequest was not signed: %v", request.CommonName, err)
	}

	// Delete CSR.
	_ = k8.CertificatesV1().CertificateSigningRequests().Delete(context.TODO(), request.CommonName, metav1.DeleteOptions{})

	info := CertificateInfo{Certificate: string(crtPEM), Key: string(key), CertificateUpdated: true}

	return &info, nil
}

// shouldGenerateNewCert checks that the certificate from the cluster matches the order
func shouldGenerateNewCert(cert []byte, request OrderCertificateRequest, durationLeft time.Duration) (bool, error) {
	c, err := helpers.ParseCertificatePEM(cert)
	if err != nil {
		return false, fmt.Errorf("certificate cannot parsed: %v", err)
	}

	if c.Subject.CommonName != request.CommonName {
		return true, nil
	}

	if !arraysAreEqual(c.Subject.Organization, request.Groups) {
		return true, nil
	}

	if !arraysAreEqual(c.DNSNames, request.SANs) {
		return true, nil
	}

	// TODO: compare usages
	// if !arraysAreEqual(c.ExtKeyUsage, request.Usages) {
	//	  return true, nil
	// }

	if time.Until(c.NotAfter) < durationLeft {
		return true, nil
	}
	return false, nil
}

func arraysAreEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Strings(a)
	sort.Strings(b)

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
