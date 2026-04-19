/*
Copyright 2022 Flant JSC

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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

const (
	mTLSCrtPath = "prometheus.internal.prometheusScraperIstioMTLS.certificate"
	mTLSKeyPath = "prometheus.internal.prometheusScraperIstioMTLS.key"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/generate_istio_mtls_cert",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "istio_secret_ca",
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applySecertIstioCAFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-istio"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cacerts"},
			},
		},
		{
			Name:       "secret_istio_mtls",
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applySecertIstioMTLSFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"prometheus-scraper-istio-mtls"},
			},
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "42 4 * * *"}, // every day at 04:42 am
	},
}, generateMTLSCertHook)

func applySecertIstioMTLSFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)

	if err != nil {
		return nil, fmt.Errorf("can't convert ca secret to secret struct: %v", err)
	}

	var cert certificate.Certificate
	var certBytes, keyBytes []byte
	var ok bool
	if certBytes, ok = secret.Data["tls.crt"]; ok {
		cert.Cert = string(certBytes)
	} else {
		return nil, fmt.Errorf("can't get certificate from secert %v", secret.Name)
	}

	if keyBytes, ok = secret.Data["tls.key"]; ok {
		cert.Key = string(keyBytes)
	} else {
		return nil, fmt.Errorf("can't get key from secert %v", secret.Name)
	}
	return cert, nil
}

func applySecertIstioCAFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)

	if err != nil {
		return nil, fmt.Errorf("can't convert ca secret to secret struct: %v", err)
	}

	var cert certificate.Authority
	var certBytes, keyBytes []byte
	var ok bool
	if certBytes, ok = secret.Data["ca-cert.pem"]; ok {
		cert.Cert = string(certBytes)
	} else {
		return nil, fmt.Errorf("can't get certificate from secert %v", secret.Name)
	}

	if keyBytes, ok = secret.Data["ca-key.pem"]; ok {
		cert.Key = string(keyBytes)
	} else {
		return nil, fmt.Errorf("can't get key from secert %v", secret.Name)
	}
	return cert, nil
}

func isCertValid(cert certificate.Certificate, ca certificate.Authority) (bool, error) {
	// create CA pool to validate certificate chain
	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM([]byte(ca.Cert))
	if !ok {
		return false, fmt.Errorf("certificate validation check: can't add CA certificate to pool")
	}
	opts := x509.VerifyOptions{
		Roots:       certPool,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		CurrentTime: time.Now().Add(time.Hour * 8030), // 8030 Hours ~ 11 Months.
	}
	block, _ := pem.Decode([]byte(cert.Cert))
	// If the block is nil, it means there is garbage in it, or it is just empty.
	if block == nil {
		return false, nil
	}
	x509cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, nil
	}
	_, err = x509cert.Verify(opts)
	if err != nil {
		return false, nil
	}
	// Everything is ok.
	return true, nil
}

func generateMTLSCertHook(_ context.Context, input *go_hook.HookInput) error {
	var ok bool
	var err error
	var istioCA certificate.Authority
	var mTLSCert certificate.Certificate

	// Get istio CA keypair.
	istioCASnap := input.Snapshots.Get("istio_secret_ca")
	if len(istioCASnap) == 0 {
		input.Values.Remove(mTLSCrtPath)
		input.Values.Remove(mTLSKeyPath)
		return nil
	}

	err = istioCASnap[0].UnmarshalTo(&istioCA)
	if err != nil {
		return fmt.Errorf("can't convert certificate to certificate struct")
	}

	clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()
	mTLSCertSAN := fmt.Sprintf("spiffe://%s/ns/d8-monitoring/sa/prometheus", clusterDomain)

	// Get prometheus scraper mTLS keypair.
	mTLSCertSnap := input.Snapshots.Get("secret_istio_mtls")
	if len(mTLSCertSnap) == 1 {
		err = mTLSCertSnap[0].UnmarshalTo(&mTLSCert)
		if err != nil {
			return fmt.Errorf("can't convert certificate to certificate struct")
		}
	}

	ok, err = isCertValid(mTLSCert, istioCA)
	if err != nil {
		return err
	}
	if !ok {
		mTLSCert, err = certificate.GenerateSelfSignedCert(input.Logger,
			"prometheus-scraper-istio-mtls",
			istioCA,
			certificate.WithKeyAlgo("ecdsa"),
			certificate.WithKeySize(256),
			certificate.WithSigningDefaultUsage([]string{
				"signing",
				"key encipherment",
				"client auth",
			}),
			certificate.WithSigningDefaultExpiry(8766*time.Hour), // 8766 hours = 1 Year
			certificate.WithSANs(
				mTLSCertSAN,
			),
		)
		if err != nil {
			return fmt.Errorf("can't generate certificate: %v", err)
		}
	}
	input.Values.Set(mTLSCrtPath, mTLSCert.Cert)
	input.Values.Set(mTLSKeyPath, mTLSCert.Key)

	return nil
}
