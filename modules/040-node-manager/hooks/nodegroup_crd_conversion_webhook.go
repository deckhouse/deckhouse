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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

// This hook patches the nodegroups.deckhouse.io CRD to use the node-controller
// conversion webhook instead of the default conversion-webhook-handler.
//
// It watches:
// - The nodegroups CRD
// - The node-controller webhook TLS secret
// - The node-controller webhook service
//
// When all resources exist and the CA bundle differs, it patches the CRD
// to point conversion webhook to node-controller-webhook service.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Queue:      "/modules/node-manager/nodegroup-crd-conversion",
	OnAfterAll: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodegroups-crd",
			ApiVersion:                   "apiextensions.k8s.io/v1",
			Kind:                         "CustomResourceDefinition",
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(false),
			NameSelector: &types.NameSelector{
				MatchNames: []string{"nodegroups.deckhouse.io"},
			},
			FilterFunc: applyNodeGroupCRDFilter,
		},
		{
			Name:                         "node-controller-webhook-tls",
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"node-controller-webhook-tls"},
			},
			FilterFunc: applyNodeControllerWebhookTLSFilter,
		},
		{
			Name:                         "node-controller-webhook",
			ApiVersion:                   "v1",
			Kind:                         "Service",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"node-controller-webhook"},
			},
			FilterFunc: applyNodeControllerServiceFilter,
		},
	},
}, injectCAToNodeGroupCRD)

// NodeGroupCRD holds the CRD name and current CA bundle
type NodeGroupCRD struct {
	Name     string
	CABundle string
}

// NodeControllerService holds the service name
type NodeControllerService struct {
	Name string
}

func applyNodeGroupCRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crd apiextensionsv1.CustomResourceDefinition

	err := sdk.FromUnstructured(obj, &crd)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	var caBundle string
	if crd.Spec.Conversion != nil &&
		crd.Spec.Conversion.Webhook != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig.CABundle != nil {
		caBundle = string(crd.Spec.Conversion.Webhook.ClientConfig.CABundle)
	}

	// Decode base64 if present
	if len(caBundle) > 0 {
		decoded, err := base64.StdEncoding.DecodeString(caBundle)
		if err != nil {
			caBundle = ""
		} else {
			caBundle = string(decoded)
		}
	}

	return NodeGroupCRD{Name: crd.Name, CABundle: caBundle}, nil
}

func applyNodeControllerWebhookTLSFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}, nil
}

func applyNodeControllerServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var svc v1.Service

	err := sdk.FromUnstructured(obj, &svc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return NodeControllerService{
		Name: svc.Name,
	}, nil
}

func injectCAToNodeGroupCRD(_ context.Context, input *go_hook.HookInput) error {
	// Check if CRD exists
	if len(input.Snapshots.Get("nodegroups-crd")) == 0 {
		input.Logger.Info("nodegroups.deckhouse.io CRD not found, skipping")
		return nil
	}

	// Check if webhook service exists
	if len(input.Snapshots.Get("node-controller-webhook")) == 0 {
		input.Logger.Info("node-controller-webhook service not found, skipping")
		return nil
	}

	// Check if CA bundle secret exists
	if len(input.Snapshots.Get("node-controller-webhook-tls")) == 0 {
		input.Logger.Info("node-controller-webhook-tls secret not found, skipping")
		return nil
	}

	var crd NodeGroupCRD
	var bundle certificate.Certificate

	// Get CA bundle from secret
	err := input.Snapshots.Get("node-controller-webhook-tls")[0].UnmarshalTo(&bundle)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'node-controller-webhook-tls' snapshot: %w", err)
	}

	// Get current CRD state
	err = input.Snapshots.Get("nodegroups-crd")[0].UnmarshalTo(&crd)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'nodegroups-crd' snapshot: %w", err)
	}

	// Check if CA bundle is already up to date
	if crd.CABundle == bundle.CA {
		input.Logger.Debug("CA bundle already up to date, skipping patch")
		return nil
	}

	input.Logger.Info("Patching nodegroups.deckhouse.io CRD conversion webhook")

	// Patch the CRD to use node-controller conversion webhook
	// This replaces the default conversion-webhook-handler with node-controller-webhook
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"conversion": map[string]interface{}{
				"strategy": "Webhook",
				"webhook": map[string]interface{}{
					"clientConfig": map[string]interface{}{
						"caBundle": base64.StdEncoding.EncodeToString([]byte(bundle.CA)),
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

	input.PatchCollector.PatchWithMerge(patch, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", crd.Name)

	input.Logger.Info("Successfully patched nodegroups.deckhouse.io CRD conversion webhook",
		"service", "node-controller-webhook",
		"namespace", "d8-cloud-instance-manager",
		"path", "/convert",
	)

	return nil
}
