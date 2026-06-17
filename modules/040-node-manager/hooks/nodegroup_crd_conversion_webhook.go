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
	"encoding/base64"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager/nodegroup-crd-conversion",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, patchNodeGroupCRDConversionWebhook)

func patchNodeGroupCRDConversionWebhook(_ context.Context, input *go_hook.HookInput) error {
	return patchCRDConversionWebhook(input, "nodegroups.deckhouse.io")
}

func patchCRDConversionWebhook(input *go_hook.HookInput, crdName string) error {
	ca, ok := input.Values.GetOk("nodeManager.internal.nodeControllerWebhookCert.ca")
	if !ok || ca.String() == "" {
		input.Logger.Info(fmt.Sprintf("nodeControllerWebhookCert.ca not set, skipping %s conversion webhook patch", crdName))
		return nil
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"conversion": map[string]interface{}{
				"strategy": "Webhook",
				"webhook": map[string]interface{}{
					"clientConfig": map[string]interface{}{
						"caBundle": base64.StdEncoding.EncodeToString([]byte(ca.String())),
						"service": map[string]interface{}{
							"namespace": "d8-cloud-instance-manager",
							"name":      "node-controller-webhook",
							"path":      "/convert",
							"port":      int64(443),
						},
					},
					"conversionReviewVersions": []string{"v1"},
				},
			},
		},
	}

	input.PatchCollector.PatchWithMerge(patch, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", crdName)

	input.Logger.Info(fmt.Sprintf("Patched %s CRD conversion webhook to node-controller-webhook", crdName))

	return nil
}
