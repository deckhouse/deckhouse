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

package copy_custom_certificate

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/module"
)

type CustomCertificate struct {
	Name string
	Data []byte
}

func applyCustomCertificateFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	cs := &CustomCertificate{}

	cs.Name = secret.GetName()
	cs.Data, err = yaml.Marshal(secret.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return cs, nil
}

func RegisterHook(moduleName string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:              "custom_certificates",
				ApiVersion:        "v1",
				Kind:              "Secret",
				NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}}},
				LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "owner",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"helm"},
					},
				}},
				FilterFunc: applyCustomCertificateFilter,
			},
		},
	}, copyCustomCertificatesHandler(moduleName))
}

func copyCustomCertificatesHandler(moduleName string) func(_ context.Context, input *go_hook.HookInput) error {
	return func(_ context.Context, input *go_hook.HookInput) error {
		snapshots := input.Snapshots.Get("custom_certificates")
		if len(snapshots) == 0 {
			input.Logger.Info("No custom certificates received, skipping setting values")
			return nil
		}

		customCertificates := make(map[string][]byte, len(snapshots))
		for cs, err := range sdkobjectpatch.SnapshotIter[CustomCertificate](snapshots) {
			if err != nil {
				continue
			}

			customCertificates[cs.Name] = cs.Data
		}

		httpsMode := module.GetHTTPSMode(moduleName, input)

		if httpsMode != "CustomCertificate" {
			input.Values.Remove(fmt.Sprintf("%s.internal.customCertificateData", moduleName))
			return nil
		}

		rawsecretName, _ := module.GetValuesFirstDefined(input, fmt.Sprintf("%s.https.customCertificate.secretName", moduleName), "global.modules.https.customCertificate.secretName")
		secretName := rawsecretName.String()

		if secretName == "" {
			return nil
		}

		secretData, ok := customCertificates[secretName]
		if !ok {
			return fmt.Errorf("custom certificate secret name is configured, but secret with this name doesn't exist")
		}

		var c cert
		err := yaml.Unmarshal(secretData, &c)
		if err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}

		path := fmt.Sprintf("%s.internal.customCertificateData", moduleName)
		input.Values.Set(path, c)
		return nil
	}
}

type cert struct {
	CA      string `json:"ca.crt,omitempty"`
	TLSKey  string `json:"tls.key,omitempty"`
	TLSCert string `json:"tls.crt,omitempty"`
}
