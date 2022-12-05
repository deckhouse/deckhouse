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

/*
LINSTOR secure setup requires four secrets with certificates. This hook generates them and stores in values.
https://github.com/piraeusdatastore/piraeus-operator/blob/master/doc/security.md
*/

import (
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

type LinstorCertSnapshot struct {
	Name string
	Cert certificate.Certificate
}

func applyCertsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert selfsigned certificate secret to secret: %v", err)
	}

	return LinstorCertSnapshot{
		Name: secret.Name,
		Cert: certificate.Certificate{
			CA:   string(secret.Data["ca.crt"]),
			Key:  string(secret.Data["tls.key"]),
			Cert: string(secret.Data["tls.crt"]),
		}}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "https_certs",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{linstorNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{linstorHTTPSControllerSecret, linstorHTTPSClientSecret},
			},
			FilterFunc: applyCertsFilter,
		},
		{
			Name:       "ssl_certs",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{linstorNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{linstorSSLControllerSecret, linstorSSLNodeSecret},
			},
			FilterFunc: applyCertsFilter,
		},
	},
}, generateSelfSignedCertificates)

func generateSelfSignedCertificates(input *go_hook.HookInput) error {
	err := generateHTTPSCertificates(input)
	if err != nil {
		return err
	}
	err = generateSSLCertificates(input)
	if err != nil {
		return err
	}
	return nil
}

func generateHTTPSCertificates(input *go_hook.HookInput) error {
	var caCert certificate.Authority
	var controllerCert certificate.Certificate
	var clientCert certificate.Certificate

	snaps := input.Snapshots["https_certs"]
	for _, snap := range snaps {
		switch s := snap.(LinstorCertSnapshot); s.Name {
		case linstorHTTPSControllerSecret:
			controllerCert = s.Cert
		case linstorHTTPSClientSecret:
			clientCert = s.Cert
		}
	}

	if len(controllerCert.CA) == 0 || controllerCert.CA != clientCert.CA {
		var err error
		caCert, err = certificate.GenerateCA(input.LogEntry, "linstor-ca")
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned ca: %v", err)
		}

		// linstorServiceFQDN := fmt.Sprintf(
		// 	"%s.%s",
		// 	linstorServiceHost,
		// 	input.Values.Get("global.discovery.clusterDomain").String(),
		// )
		controllerCert, err = certificate.GenerateSelfSignedCert(input.LogEntry,
			"linstor-controller",
			caCert,
			certificate.WithSigningDefaultExpiry(87600*time.Hour),
			certificate.WithSANs(
				linstorServiceName,
				linstorServiceHost,
				// linstorServiceFQDN,
				"localhost",
				"::1",
				"127.0.0.1",
			),
		)
		if err != nil {
			return fmt.Errorf("cannot generate controller certificate: %v", err)
		}
		clientCert, err = certificate.GenerateSelfSignedCert(input.LogEntry,
			"linstor-client",
			caCert,
			certificate.WithSigningDefaultExpiry(87600*time.Hour),
		)
		if err != nil {
			return fmt.Errorf("cannot generate client certificate: %v", err)
		}
	}

	input.Values.Set(httpsControllerCertPath, controllerCert)
	input.Values.Set(httpsClientCertPath, clientCert)
	return nil
}

func generateSSLCertificates(input *go_hook.HookInput) error {
	var caCert certificate.Authority
	var controllerCert certificate.Certificate
	var nodeCert certificate.Certificate

	snaps := input.Snapshots["ssl_certs"]
	for _, snap := range snaps {
		switch s := snap.(LinstorCertSnapshot); s.Name {
		case linstorSSLControllerSecret:
			controllerCert = s.Cert
		case linstorSSLNodeSecret:
			nodeCert = s.Cert
		}
	}

	if len(controllerCert.CA) == 0 || controllerCert.CA != nodeCert.CA {
		var err error
		caCert, err = certificate.GenerateCA(input.LogEntry, "linstor-ca")
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned ca: %v", err)
		}

		// linstorServiceFQDN := fmt.Sprintf(
		// 	"%s.%s",
		// 	linstorServiceHost,
		// 	input.Values.Get("global.discovery.clusterDomain").String(),
		// )
		controllerCert, err = certificate.GenerateSelfSignedCert(input.LogEntry,
			"linstor-controller",
			caCert,
			certificate.WithSigningDefaultExpiry(87600*time.Hour),
			certificate.WithSANs(
				linstorServiceName,
				linstorServiceHost,
				// linstorServiceFQDN,
				"localhost",
				"::1",
				"127.0.0.1",
			),
		)
		if err != nil {
			return fmt.Errorf("cannot generate controller certificate: %v", err)
		}
		nodeCert, err = certificate.GenerateSelfSignedCert(input.LogEntry,
			"linstor-node",
			caCert,
			certificate.WithSigningDefaultExpiry(87600*time.Hour),
		)
		if err != nil {
			return fmt.Errorf("cannot generate node certificate: %v", err)
		}
	}

	input.Values.Set(sslControllerCertPath, controllerCert)
	input.Values.Set(sslNodeCertPath, nodeCert)
	return nil
}
