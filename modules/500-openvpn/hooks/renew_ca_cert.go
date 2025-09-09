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
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type CertInfo struct {
	CertData []byte
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "check_server_cert_expiry",
			Crontab: "*/10 * * * *", // Every 10 minutes
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "openvpn_pki_ca",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-openvpn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"openvpn-pki-ca"},
			},
			FilterFunc: applyCASecretFilter,
		},
	},
}, checkServerCertExpiry)

func applyCASecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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

	return CertInfo{certData}, nil
}

func checkServerCertExpiry(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("openvpn_pki_ca")

	if len(snaps) == 0 {
		input.Logger.Warn("Secret openvpn-pki-server or openvpn-pki-ca not found, skipping")
		return nil
	}

	now := time.Now()
	var caInfo CertInfo
	if err := snaps[0].UnmarshalTo(&caInfo); err != nil {
		return fmt.Errorf("failed to unmarshal 'openvpn_pki_ca': %w", err)
	}
	caCertNotAfter := certNotAfter(caInfo, input)

	if caCertNotAfter == nil {
		input.Logger.Error("Failed to parse certificates, skipping")
		return nil
	}

	daysLeftCa := int(caCertNotAfter.Sub(now).Hours() / 24)

	input.Logger.Info("Certificate status",
		slog.Int("ca_days_left", daysLeftCa),
	)

	// We check whether the CA expires in the next 24 hours
	if caCertNotAfter.Sub(now) <= 24*time.Hour {
		input.Logger.Info("Server certificate expired, initiating cleanup server cert and restart ovpn StateFullSet")

		// Remove openvpn-pki-server
		input.PatchCollector.Delete("v1", "Secret", "d8-openvpn", "openvpn-pki-server") // Server cert
		input.PatchCollector.Delete("v1", "Secret", "d8-openvpn", "openvpn-pki-ca")     // CA cert
		input.Logger.Info("Secret openvpn-pki-server, openvpn-pki-ca scheduled for deletion")

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

		input.PatchCollector.PatchWithMerge(patch, "apps/v1", "StatefulSet", "d8-openvpn", "openvpn")
		input.Logger.Info("StatefulSet openvpn scheduled for restart")
	} else {
		input.Logger.Info("Server CA certificate is valid", slog.Time("until", *caCertNotAfter))
	}

	return nil
}

func certNotAfter(certInfo CertInfo, input *go_hook.HookInput) *time.Time {
	block, _ := pem.Decode(certInfo.CertData)
	if block == nil || block.Type != "CERTIFICATE" {
		input.Logger.Error("Failed to decode PEM block from tls.crt")
		return nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		input.Logger.Error("Failed to parse certificate", log.Err(err))
		return nil
	}

	return &cert.NotAfter
}
