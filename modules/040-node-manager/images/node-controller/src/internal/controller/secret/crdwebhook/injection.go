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

package crdwebhook

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// webhookSecretConfig maps a Secret name to the CRDs it manages and the
// conversion webhook service configuration.
type webhookSecretConfig struct {
	// CRDNames is the list of CRDs whose conversion webhook CA bundle should be patched.
	CRDNames []string
	// ServiceName is the name of the conversion webhook Service.
	ServiceName string
	// ServicePath is the path on the conversion webhook Service.
	ServicePath string
	// ServicePort is the port of the conversion webhook Service (0 means omit from patch).
	ServicePort int32
	// ConversionReviewVersions is the list of supported ConversionReview versions.
	ConversionReviewVersions []string
}

const webhookNamespace = "d8-cloud-instance-manager"

// secretConfigs defines the mapping from TLS secret names to the CRDs and
// webhook service configuration they manage.
var secretConfigs = map[string]webhookSecretConfig{
	"capi-webhook-tls": {
		CRDNames: []string{
			"clusters.cluster.x-k8s.io",
			"machines.cluster.x-k8s.io",
			"machinesets.cluster.x-k8s.io",
			"machinedeployments.cluster.x-k8s.io",
			"machinepools.cluster.x-k8s.io",
			"machinehealthchecks.cluster.x-k8s.io",
			"machinedrainrules.cluster.x-k8s.io",
			"extensionconfigs.runtime.cluster.x-k8s.io",
		},
		ServiceName:              "capi-webhook-service",
		ServicePath:              "/convert",
		ServicePort:              443,
		ConversionReviewVersions: []string{"v1", "v1beta1"},
	},
	"caps-controller-manager-webhook-tls": {
		CRDNames: []string{
			"sshcredentials.deckhouse.io",
		},
		ServiceName:              "caps-controller-manager-webhook-service",
		ServicePath:              "/convert",
		ServicePort:              0, // omit port for CAPS
		ConversionReviewVersions: []string{"v1"},
	},
	"node-controller-webhook-tls": {
		CRDNames: []string{
			"nodegroups.deckhouse.io",
		},
		ServiceName:              "node-controller-webhook",
		ServicePath:              "/convert",
		ServicePort:              443,
		ConversionReviewVersions: []string{"v1"},
	},
}

// patchCRDsCABundle patches the conversion webhook CA bundle on all CRDs
// associated with the given secret name. It skips CRDs that already have the
// correct CA bundle.
func patchCRDsCABundle(ctx context.Context, c client.Client, log logr.Logger, secretName string, caBundle []byte) error {
	cfg, ok := secretConfigs[secretName]
	if !ok {
		return fmt.Errorf("unknown webhook secret: %s", secretName)
	}

	encodedCA := base64.StdEncoding.EncodeToString(caBundle)

	for _, crdName := range cfg.CRDNames {
		if err := patchSingleCRD(ctx, c, log, crdName, encodedCA, cfg); err != nil {
			return fmt.Errorf("patch CRD %s: %w", crdName, err)
		}
	}

	return nil
}

// patchSingleCRD patches the conversion webhook configuration on a single CRD.
func patchSingleCRD(ctx context.Context, c client.Client, log logr.Logger, crdName, encodedCA string, cfg webhookSecretConfig) error {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := c.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("CRD not found, skipping", "crd", crdName)
			return nil
		}
		return fmt.Errorf("get CRD %s: %w", crdName, err)
	}

	// Check if CA bundle is already up to date.
	if crd.Spec.Conversion != nil &&
		crd.Spec.Conversion.Webhook != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig.CABundle != nil {
		existingCA := string(crd.Spec.Conversion.Webhook.ClientConfig.CABundle)
		if existingCA == encodedCA {
			log.V(1).Info("CA bundle already up to date", "crd", crdName)
			return nil
		}
	}

	patch := client.MergeFrom(crd.DeepCopy())

	svcRef := &apiextensionsv1.ServiceReference{
		Namespace: webhookNamespace,
		Name:      cfg.ServiceName,
		Path:      ToPtr(cfg.ServicePath),
	}
	if cfg.ServicePort > 0 {
		svcRef.Port = ToPtr(cfg.ServicePort)
	}

	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				CABundle: []byte(encodedCA),
				Service:  svcRef,
			},
			ConversionReviewVersions: cfg.ConversionReviewVersions,
		},
	}

	if err := c.Patch(ctx, crd, patch); err != nil {
		return fmt.Errorf("patch CRD %s conversion webhook: %w", crdName, err)
	}

	log.Info("patched CRD conversion webhook CA bundle", "crd", crdName, "service", cfg.ServiceName)
	return nil
}

func ToPtr[T any](v T) *T {
	return &v
}
