/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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

// This hook patches the CAPCD CRDs to inject the CA bundle for the conversion webhook.
//
// It watches:
// - The CAPCD CRDs (vcdclusters, vcdmachines, vcdmachinetemplates, vcdclustertemplates)
// - The capcd-controller-manager-webhook-tls secret
// - The capcd-controller-manager-webhook-service service
//
// When all resources exist and the CA bundle differs, it patches the CRDs
// to include the CA bundle for the conversion webhook.

var capiCRDNames = []string{
	"vcdclusters.infrastructure.cluster.x-k8s.io",
	"vcdmachines.infrastructure.cluster.x-k8s.io",
	"vcdmachinetemplates.infrastructure.cluster.x-k8s.io",
	"vcdclustertemplates.infrastructure.cluster.x-k8s.io",
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Queue:      "/modules/cloud-provider-vcd/capcd-crd-conversion",
	OnAfterAll: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "capcd-crds",
			ApiVersion:                   "apiextensions.k8s.io/v1",
			Kind:                         "CustomResourceDefinition",
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(false),
			NameSelector: &types.NameSelector{
				MatchNames: capiCRDNames,
			},
			FilterFunc: applyCAPCDCRDFilter,
		},
		{
			Name:                         "capcd-webhook-tls",
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-provider-vcd"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"capcd-controller-manager-webhook-tls"},
			},
			FilterFunc: applyCAPCDWebhookTLSFilter,
		},
		{
			Name:                         "capcd-webhook-service",
			ApiVersion:                   "v1",
			Kind:                         "Service",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-provider-vcd"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"capcd-controller-manager-webhook-service"},
			},
			FilterFunc: applyCAPCDServiceFilter,
		},
	},
}, injectCAToCAPCDCRDs)

// CAPCDCRD holds the CRD name and current CA bundle
type CAPCDCRD struct {
	Name     string
	CABundle string
}

// CAPCDService holds the service name
type CAPCDService struct {
	Name string
}

func applyCAPCDCRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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

	return CAPCDCRD{Name: crd.Name, CABundle: caBundle}, nil
}

func applyCAPCDWebhookTLSFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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

func applyCAPCDServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var svc v1.Service

	err := sdk.FromUnstructured(obj, &svc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return CAPCDService{
		Name: svc.Name,
	}, nil
}

func injectCAToCAPCDCRDs(_ context.Context, input *go_hook.HookInput) error {
	// Check if CRDs exist
	if len(input.Snapshots.Get("capcd-crds")) == 0 {
		input.Logger.Info("CAPCD CRDs not found, skipping")
		return nil
	}

	// Check if webhook service exists
	if len(input.Snapshots.Get("capcd-webhook-service")) == 0 {
		input.Logger.Info("capcd-controller-manager-webhook-service not found, skipping")
		return nil
	}

	// Check if CA bundle secret exists
	if len(input.Snapshots.Get("capcd-webhook-tls")) == 0 {
		input.Logger.Info("capcd-controller-manager-webhook-tls secret not found, skipping")
		return nil
	}

	var bundle certificate.Certificate

	// Get CA bundle from secret
	err := input.Snapshots.Get("capcd-webhook-tls")[0].UnmarshalTo(&bundle)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'capcd-webhook-tls' snapshot: %w", err)
	}

	// Patch each CRD
	for _, snapshot := range input.Snapshots.Get("capcd-crds") {
		var crd CAPCDCRD
		err = snapshot.UnmarshalTo(&crd)
		if err != nil {
			return fmt.Errorf("failed to unmarshal 'capcd-crds' snapshot: %w", err)
		}

		// Check if CA bundle is already up to date
		if crd.CABundle == bundle.CA {
			input.Logger.Debug("CA bundle already up to date, skipping patch", "crd", crd.Name)
			continue
		}

		input.Logger.Info("Patching CRD conversion webhook", "crd", crd.Name)

		// Patch the CRD to include CA bundle
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"conversion": map[string]interface{}{
					"strategy": "Webhook",
					"webhook": map[string]interface{}{
						"clientConfig": map[string]interface{}{
							"caBundle": base64.StdEncoding.EncodeToString([]byte(bundle.CA)),
							"service": map[string]interface{}{
								"namespace": "d8-cloud-provider-vcd",
								"name":      "capcd-controller-manager-webhook-service",
								"path":      "/convert",
							},
						},
						"conversionReviewVersions": []string{"v1", "v1beta1"},
					},
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", crd.Name)

		input.Logger.Info("Successfully patched CRD conversion webhook", "crd", crd.Name)
	}

	return nil
}
