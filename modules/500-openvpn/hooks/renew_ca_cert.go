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

package hooks

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "check_server_cert_expiry",
			Crontab: "0 */6 * * *", // Every 6 hours
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "openvpn_pki_server",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-openvpn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"openvpn-pki-server"},
			},
			FilterFunc: applyServerSecretFilter,
		},
		{
			Name:       "openvpn_client_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-openvpn"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"type": "clientAuth",
				},
			},
			FilterFunc: applyClientSecretFilter,
		},
	},
}, checkServerCertExpiry)

func applyServerSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret to structured object: %v", err)
	}

	certData, ok := secret.Data["tls.crt"]
	if !ok || len(certData) == 0 {
		log.Info("No tls.crt in secret")
		return nil, nil
	}

	return certData, nil
}

func applyClientSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}
	return secret.Name, nil
}

type CertInfo struct {
	CertData []byte
}

func checkServerCertExpiry(input *go_hook.HookInput) error {
	snapshots := input.Snapshots["openvpn_pki_server"]

	if len(snapshots) == 0 {
		input.Logger.Warn("Secret openvpn-pki-server not found, skipping")
		return nil
	}

	certData := snapshots[0].([]byte)

	block, _ := pem.Decode(certData)
	if block == nil || block.Type != "CERTIFICATE" {
		input.Logger.Errorf("Failed to decode PEM block from tls.crt")
		return nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		input.Logger.Errorf("Failed to parse certificate: %v", err)
		return nil
	}

	// // Check if cert expires within 6 hours
	now := time.Now()
	if cert.NotAfter.Sub(now) <= 6*time.Hour {
		input.Logger.Infof("Server certificate expired at %s, initiating cleanup and restart", cert.NotAfter)

		// Remove openvpn-pki-ca
		input.PatchCollector.Delete("v1", "Secret", "d8-openvpn", "openvpn-pki-ca")        // Ca cert
		input.PatchCollector.Delete("v1", "Secret", "d8-openvpn", "openvpn-pki-server")    // Server cert
		input.PatchCollector.Delete("v1", "Secret", "d8-openvpn", "openvpn-pki-crl")       // Certificate Revocation List
		input.PatchCollector.Delete("v1", "Secret", "d8-openvpn", "openvpn-pki-index-txt") // list client certs
		input.Logger.Info("Secret openvpn-pki-ca, openvpn-pki-server, openvpn-pki-crl scheduled for deletion")

		// Remove clients certs
		snapshotsClients := input.Snapshots["openvpn_client_secrets"]
		if len(snapshotsClients) == 0 {
			input.Logger.Warn("No client secrets with type=clientAuth found")
		}
		for _, snapshot := range snapshotsClients {
			clientSecretName := snapshot.(string)
			input.PatchCollector.Delete("v1", "Secret", "d8-openvpn", clientSecretName)
			input.Logger.Infof("Client secret %s scheduled for deletion", clientSecretName)
		}

		// Patch spec.template.metadata.annotations for reload SS
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]string{
							"restart-timestamp": now.Format(time.RFC3339),
						},
					},
				},
			},
		}
		input.PatchCollector.MergePatch(patch, "apps/v1", "StatefulSet", "d8-openvpn", "openvpn")
		input.Logger.Info("StatefulSet openvpn scheduled for restart")
	} else {
		input.Logger.Infof("Server certificate is valid until %s", cert.NotAfter)
	}

	return nil
}
