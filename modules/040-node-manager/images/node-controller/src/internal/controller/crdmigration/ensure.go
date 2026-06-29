/*
Copyright 2026 Flant JSC

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

package crdmigration

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	caBundlePollInterval = 5 * time.Second
	caBundlePollTimeout  = 10 * time.Minute

	nodeControllerWebhookSecretName  = "node-controller-webhook-tls"
	nodeControllerWebhookServiceName = "node-controller-webhook"
)

var conversionCRDNames = []string{
	"nodegroups.deckhouse.io",
	"instances.deckhouse.io",
}

func EnsureCRDs(ctx context.Context, c client.Client) error {
	// 1. Patch conversion webhook on deckhouse CRDs (nodegroups, instances).
	// This must happen first to unblock API server — the old conversion-webhook-handler
	// may not support new API versions served by updated CRDs.
	if err := ensureConversionWebhooks(ctx, c); err != nil {
		return fmt.Errorf("ensuring conversion webhooks: %w", err)
	}

	// 2. Ensure CAPI CRDs (cluster.x-k8s.io).
	crds, err := loadCRDs(crdDir)
	if err != nil {
		return fmt.Errorf("loading CRD manifests: %w", err)
	}

	// Best-effort: do not block startup if capi-webhook-tls is absent (e.g. fresh static
	// cluster). CRDs are created without CABundle; the reconciler fills the CA later.
	caBundle := bestEffortCABundle(ctx, c, webhookSecretName)

	for crdName, embeddedCRD := range crds {
		if !capiCRDNames[crdName] {
			continue
		}
		if err := ensureSingleCRD(ctx, c, crdName, embeddedCRD, caBundle); err != nil {
			return fmt.Errorf("ensuring CRD %s: %w", crdName, err)
		}
	}

	klog.Info("all CAPI CRDs ensured")
	return nil
}

func ensureConversionWebhooks(ctx context.Context, c client.Client) error {
	// Check if any conversion CRD actually exists and needs conversion.
	// On bootstrap, CRDs don't exist yet — skip.
	needsConversion := false
	for _, crdName := range conversionCRDNames {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := c.Get(ctx, types.NamespacedName{Name: crdName}, crd); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("get CRD %s: %w", crdName, err)
		}
		if len(crd.Spec.Versions) > 1 {
			needsConversion = true
			break
		}
	}
	if !needsConversion {
		klog.Info("no conversion CRDs found or all have single version, skipping conversion webhook patch")
		return nil
	}

	klog.Info("waiting for node-controller webhook CA secret ", nodeControllerWebhookSecretName)
	caBundle, err := waitForCABundle(ctx, c, nodeControllerWebhookSecretName)
	if err != nil {
		return fmt.Errorf("waiting for %s CA bundle: %w", nodeControllerWebhookSecretName, err)
	}
	klog.Info("node-controller webhook CA secret ready")

	for _, crdName := range conversionCRDNames {
		if err := patchConversionWebhook(ctx, c, crdName, caBundle); err != nil {
			return fmt.Errorf("patching conversion webhook on %s: %w", crdName, err)
		}
	}

	return nil
}

func patchConversionWebhook(ctx context.Context, c client.Client, crdName string, caBundle []byte) error {
	existing := &apiextensionsv1.CustomResourceDefinition{}
	if err := c.Get(ctx, types.NamespacedName{Name: crdName}, existing); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("CRD %s not found, skipping conversion webhook patch", crdName)
			return nil
		}
		return fmt.Errorf("getting CRD: %w", err)
	}

	if isConversionWebhookCurrent(existing, caBundle) {
		klog.V(1).Infof("CRD %s conversion webhook already up to date", crdName)
		return nil
	}

	patch := existing.DeepCopy()
	patch.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Name:      nodeControllerWebhookServiceName,
					Namespace: capiNamespace,
					Path:      ptrString("/convert"),
					Port:      ptrInt32(443),
				},
				CABundle: caBundle,
			},
			ConversionReviewVersions: []string{"v1"},
		},
	}

	if err := c.Patch(ctx, patch, client.MergeFrom(existing)); err != nil {
		return fmt.Errorf("patching: %w", err)
	}

	klog.Infof("CRD %s conversion webhook patched to node-controller-webhook", crdName)
	return nil
}

func isConversionWebhookCurrent(crd *apiextensionsv1.CustomResourceDefinition, caBundle []byte) bool {
	conv := crd.Spec.Conversion
	if conv == nil || conv.Strategy != apiextensionsv1.WebhookConverter {
		return false
	}
	wh := conv.Webhook
	if wh == nil || wh.ClientConfig == nil || wh.ClientConfig.Service == nil {
		return false
	}
	svc := wh.ClientConfig.Service
	if svc.Name != nodeControllerWebhookServiceName || svc.Namespace != capiNamespace {
		return false
	}
	return string(wh.ClientConfig.CABundle) == string(caBundle)
}

// bestEffortCABundle reads the CA bundle once without blocking, returning nil if absent.
func bestEffortCABundle(ctx context.Context, c client.Client, secretName string) []byte {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: capiNamespace}, secret); err != nil {
		klog.Infof("CA bundle secret %s/%s not available yet, creating CRDs without CABundle: %v", capiNamespace, secretName, err)
		return nil
	}

	ca := secret.Data["ca.crt"]
	if len(ca) == 0 {
		klog.Infof("CA bundle secret %s/%s has empty ca.crt, creating CRDs without CABundle", capiNamespace, secretName)
		return nil
	}

	klog.Info("CA bundle secret ready")
	return ca
}

func waitForCABundle(ctx context.Context, c client.Client, secretName string) ([]byte, error) {
	var caBundle []byte

	err := wait.PollUntilContextTimeout(ctx, caBundlePollInterval, caBundlePollTimeout, true, func(ctx context.Context) (bool, error) {
		secret := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: capiNamespace,
		}, secret); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		ca := secret.Data["ca.crt"]
		if len(ca) == 0 {
			return false, nil
		}

		caBundle = ca
		return true, nil
	})

	return caBundle, err
}

func ensureSingleCRD(ctx context.Context, c client.Client, crdName string, embeddedCRD *apiextensionsv1.CustomResourceDefinition, caBundle []byte) error {
	existing := &apiextensionsv1.CustomResourceDefinition{}
	err := c.Get(ctx, types.NamespacedName{Name: crdName}, existing)
	if errors.IsNotFound(err) {
		newCRD := embeddedCRD.DeepCopy()
		setMigrationSpec(newCRD, caBundle)
		if err := c.Create(ctx, newCRD); err != nil {
			if errors.IsAlreadyExists(err) {
				return nil
			}
			return fmt.Errorf("creating: %w", err)
		}
		klog.Infof("CRD %s created", crdName)
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting: %w", err)
	}

	// Full apply: take entire spec from embedded CRD, then apply migration on top.
	patch := existing.DeepCopy()
	patch.Spec = *embeddedCRD.Spec.DeepCopy()
	setMigrationSpec(patch, caBundle)
	if err := c.Patch(ctx, patch, client.MergeFrom(existing)); err != nil {
		return fmt.Errorf("patching: %w", err)
	}

	klog.Infof("CRD %s ensured", crdName)
	return nil
}
