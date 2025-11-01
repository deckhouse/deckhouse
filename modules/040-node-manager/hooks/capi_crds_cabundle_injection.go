/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
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

// List of Cluster API CRDs that require conversion webhooks.
var capiCRDs = []string{
	"clusters.cluster.x-k8s.io",
	"machines.cluster.x-k8s.io",
	"machinesets.cluster.x-k8s.io",
	"machinedeployments.cluster.x-k8s.io",
	"machinepools.cluster.x-k8s.io",
	"machinehealthchecks.cluster.x-k8s.io",
	"machinedrainrules.cluster.x-k8s.io",
	"extensionconfigs.cluster.x-k8s.io",
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	OnAfterAll: &go_hook.OrderedConfig{Order: 20},
	Queue:      "/modules/node-manager/capi-crds-conversions",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "capicrds",
			ApiVersion:                   "apiextensions.k8s.io/v1",
			Kind:                         "CustomResourceDefinition",
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(false),
			NameSelector: &types.NameSelector{
				MatchNames: capiCRDs,
			},
			FilterFunc: applyCAPICRDFilter,
		},
		{
			Name:                         "cabundle",
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
				MatchNames: []string{"capi-webhook-tls"},
			},
			FilterFunc: applyCAPICertFilter,
		},
		{
			Name:                         "webhook-service",
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
				MatchNames: []string{"capi-webhook-service"},
			},
			FilterFunc: applyCAPIServiceFilter,
		},
	},
}, injectCAIntoCAPICRDs)

type CRDCAPI struct {
	Name     string
	CABundle string
}

func applyCAPICRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crd apiextensionsv1.CustomResourceDefinition
	if err := sdk.FromUnstructured(obj, &crd); err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	var caBundle string
	if crd.Spec.Conversion != nil &&
		crd.Spec.Conversion.Webhook != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig.CABundle != nil {
		caBundle = string(crd.Spec.Conversion.Webhook.ClientConfig.CABundle)
	}

	if len(caBundle) > 0 {
		decoded, err := base64.StdEncoding.DecodeString(caBundle)
		if err != nil {
			caBundle = ""
		} else {
			caBundle = string(decoded)
		}
	}

	return CRDCAPI{
		Name:     crd.Name,
		CABundle: caBundle,
	}, nil
}

func applyCAPICertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, err
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}, nil
}

func applyCAPIServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var svc v1.Service

	err := sdk.FromUnstructured(obj, &svc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return Svc{
		Name: svc.Name,
	}, nil
}

func injectCAIntoCAPICRDs(_ context.Context, input *go_hook.HookInput) error {
	if len(input.Snapshots.Get("capicrds")) == 0 {
		return nil
	}

	if len(input.Snapshots.Get("webhook-service")) == 0 {
		return nil
	}

	if len(input.Snapshots.Get("cabundle")) > 0 {
		var crd CRD
		var bundle certificate.Certificate

		if err := input.Snapshots.Get("cabundle")[0].UnmarshalTo(&bundle); err != nil {
			return fmt.Errorf("failed to unmarshal capi-webhook-tls Secret: %w", err)
		}

		if err := input.Snapshots.Get("capicrds")[0].UnmarshalTo(&crd); err != nil {
			return fmt.Errorf("failed to unmarshal CAPI crd: %w", err)
		}

		if crd.CABundle == bundle.CA {
			return nil
		}

		for _, crdName := range capiCRDs {
			patch := map[string]interface{}{
				"spec": map[string]interface{}{
					"conversion": map[string]interface{}{
						"strategy": "Webhook",
						"webhook": map[string]interface{}{
							"clientConfig": map[string]interface{}{
								"caBundle": base64.StdEncoding.EncodeToString([]byte(bundle.CA)),
								"service": map[string]interface{}{
									"namespace": "d8-cloud-instance-manager",
									"name":      "capi-webhook-service",
									"path":      "/convert",
									"port":      443,
								},
							},
							"conversionReviewVersions": []string{"v1", "v1beta1"},
						},
					},
				},
			}

			input.PatchCollector.PatchWithMerge(patch,
				"apiextensions.k8s.io/v1",
				"CustomResourceDefinition",
				"",
				crdName,
			)
		}

	}

	return nil
}
