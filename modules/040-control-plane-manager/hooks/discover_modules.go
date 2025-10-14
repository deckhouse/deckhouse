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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "auth-cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn", "d8-user-authz", "d8-runtime-audit-engine"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"control-plane-configurator": "",
				},
			},
			FilterFunc: discoveryFilterSecrets,
		},
	},
}, handleAuthDiscoveryModules)

func discoveryFilterSecrets(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap

	err := sdk.FromUnstructured(unstructured, &cm)
	if err != nil {
		return nil, err
	}

	return discoveryCM{Namespace: cm.Namespace, Data: cm.Data}, nil
}

type discoveryCM struct {
	Namespace string
	Data      map[string]string
}

func handleAuthDiscoveryModules(_ context.Context, input *go_hook.HookInput) error {
	authCMs, err := sdkobjectpatch.UnmarshalToStruct[discoveryCM](input.Snapshots, "auth-cm")
	if err != nil {
		return fmt.Errorf("cannot unmarshal auth-cm: %w", err)
	}

	var authZData, authNData, auditData map[string]string

	for _, cm := range authCMs {
		switch cm.Namespace {
		case "d8-user-authn":
			authNData = cm.Data

		case "d8-user-authz":
			authZData = cm.Data

		case "d8-runtime-audit-engine":
			auditData = cm.Data
		}
	}

	const (
		userAuthzWebhookURLPath = "controlPlaneManager.apiserver.authz.webhookURL"
		userAuthzWebhookCAPath  = "controlPlaneManager.apiserver.authz.webhookCA"

		userAuthenticationWebhookURLPath = "controlPlaneManager.apiserver.authn.webhookURL"
		userAuthenticationWebhookCAPath  = "controlPlaneManager.apiserver.authn.webhookCA"

		userAuthnOIDCIssuerURLPath     = "controlPlaneManager.apiserver.authn.oidcIssuerURL"
		userAuthnOIDCIssuerAddressPath = "controlPlaneManager.apiserver.authn.oidcIssuerAddress"
		userAuthnOIDCIssuerCAPath      = "controlPlaneManager.apiserver.authn.oidcCA"

		runtimeAuditWebhookURLPath = "controlPlaneManager.internal.audit.webhookURL"
		runtimeAuditWebhookCAPath  = "controlPlaneManager.internal.audit.webhookCA"
	)

	authzWebhookURLExists := input.ConfigValues.Exists(userAuthzWebhookURLPath)
	authzWebhookCAExists := input.ConfigValues.Exists(userAuthzWebhookCAPath)

	if !authzWebhookURLExists && !authzWebhookCAExists {
		// nothing was configured by hand
		if len(authZData) > 0 {
			input.Values.Set(userAuthzWebhookURLPath, authZData["url"])
			input.Values.Set(userAuthzWebhookCAPath, authZData["ca"])
		} else {
			input.Values.Remove(userAuthzWebhookURLPath)
			input.Values.Remove(userAuthzWebhookCAPath)
		}
	}

	authnOIDCIssuerExists := input.ConfigValues.Exists(userAuthnOIDCIssuerURLPath)
	authnOIDCCAExists := input.ConfigValues.Exists(userAuthnOIDCIssuerCAPath)

	if !authnOIDCIssuerExists && !authnOIDCCAExists {
		// nothing was configured by hand
		if issuerURL, ok := authNData["oidcIssuerURL"]; ok {
			input.Values.Set(userAuthnOIDCIssuerURLPath, issuerURL)

			if address, ok := authNData["oidcIssuerAddress"]; ok {
				input.Values.Set(userAuthnOIDCIssuerAddressPath, address)
			}

			ca, ok := authNData["oidcCA"]
			if !ok {
				ca = input.Values.Get("global.discovery.kubernetesCA").String()
			}
			input.Values.Set(userAuthnOIDCIssuerCAPath, ca)
		} else {
			input.Values.Remove(userAuthnOIDCIssuerURLPath)
			input.Values.Remove(userAuthnOIDCIssuerCAPath)
			input.Values.Remove(userAuthnOIDCIssuerAddressPath)
		}
	}

	authnWebhookURLExists := input.ConfigValues.Exists(userAuthenticationWebhookURLPath)
	authnWebhookCAExists := input.ConfigValues.Exists(userAuthenticationWebhookCAPath)

	if !authnWebhookURLExists && !authnWebhookCAExists {
		// nothing was configured by hand
		if webhookURL, ok := authNData["url"]; ok {
			input.Values.Set(userAuthenticationWebhookURLPath, webhookURL)
			input.Values.Set(userAuthenticationWebhookCAPath, authNData["ca"])
		} else {
			input.Values.Remove(userAuthenticationWebhookURLPath)
			input.Values.Remove(userAuthenticationWebhookCAPath)
		}
	}

	if len(auditData) > 0 {
		input.Values.Set(runtimeAuditWebhookURLPath, auditData["url"])
		input.Values.Set(runtimeAuditWebhookCAPath, auditData["ca"])
	} else {
		input.Values.Remove(runtimeAuditWebhookURLPath)
		input.Values.Remove(runtimeAuditWebhookCAPath)
	}
	return nil
}
