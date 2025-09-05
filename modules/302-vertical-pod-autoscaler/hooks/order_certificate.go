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

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

type vpaCertSecretData struct {
	CACert     string `json:"CACert"`
	CAKey      string `json:"CAKey"`
	ServerCert string `json:"serverCert"`
	ServerKey  string `json:"serverKey"`
}

const (
	initValuesString       = `{"global": {}, "verticalPodAutoscaler":{"internal":{}}}`
	initConfigValuesString = `{}`
)

func applyVpaCertSecretRuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert vpa-tls-certs to secret: %v", err)
	}

	s := &vpaCertSecretData{}
	CACert, ok := secret.Data["caCert.pem"]
	if !ok {
		return nil, fmt.Errorf("'caCert.pem' field not found")
	}
	CAKey, ok := secret.Data["caKey.pem"]
	if !ok {
		return nil, fmt.Errorf("'caKey.pem' field not found")
	}
	ServerCert, ok := secret.Data["serverCert.pem"]
	if !ok {
		return nil, fmt.Errorf("'serverCert.pem' field not found")
	}
	ServerKey, ok := secret.Data["serverKey.pem"]
	if !ok {
		return nil, fmt.Errorf("'serverKey.pem' field not found")
	}

	s.CACert = string(CACert)
	s.CAKey = string(CAKey)
	s.ServerCert = string(ServerCert)
	s.ServerKey = string(ServerKey)

	return s, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "vpaCertCron", Crontab: "15 10 * * *"},
	},
	Queue: "/modules/vertical-pod-autoscaler",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "VPACertSecret",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"vpa-tls-certs"}},
			FilterFunc:        applyVpaCertSecretRuleFilter,
		},
	},
}, vpaCertHandler)

func vpaCertHandler(_ context.Context, input *go_hook.HookInput) error {
	var (
		vpaCert vpaCertSecretData
		err     error
	)

	snapshots := input.Snapshots.Get("VPACertSecret")

	shouldGenerateNewCert := true

	for vpaCert, err = range sdkobjectpatch.SnapshotIter[vpaCertSecretData](snapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'VPACertSecret' snapshots: %w", err)
		}

		shouldGenerateNewCert, err = certificate.IsCertificateExpiringSoon([]byte(vpaCert.ServerCert), time.Hour*7*24)
		if err != nil {
			return err
		}
	}

	if shouldGenerateNewCert {
		selfSignedCA, err := certificate.GenerateCA(input.Logger, "vpa_webhook")
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned ca: %v", err)
		}

		cert, err := certificate.GenerateSelfSignedCert(input.Logger,
			"vpa-webhook",
			selfSignedCA,
			certificate.WithSANs("vpa-webhook.kube-system", "vpa-webhook.kube-system.svc"),
		)
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned cert: %v", err)
		}

		vpaCert.CACert = selfSignedCA.Cert
		vpaCert.CAKey = selfSignedCA.Key
		vpaCert.ServerKey = cert.Key
		vpaCert.ServerCert = cert.Cert
	}

	input.Values.Set("verticalPodAutoscaler.internal", vpaCert)
	return nil
}
