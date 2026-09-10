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

package capi

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Cloud-provider modules publish their MCM/CAPI templates into these Secrets at the
// 030 step, before node-controller starts. Reading templates from the cluster (instead
// of baking the provider tree into the node-controller image) keeps node-controller
// generic and works for out-of-tree providers.
const (
	providerTemplateSecretNamespace = "kube-system"
	engineMCMTemplates              = "mcm"
	engineCAPITemplates             = "capi"
)

// readProviderTemplate returns a single template file (by its basename key, e.g.
// "machine-class.yaml", "machine-template.yaml" or "cluster.yaml") from the cloud-provider template
// Secret d8-cloud-provider-<type>-<engine>, served watch-fresh from the kube-system
// Secret informer.
func (b *BaseWithReader) readProviderTemplate(ctx context.Context, cloudType, engine, key string) ([]byte, error) {
	data, found, err := b.readProviderTemplateIfPresent(ctx, cloudType, engine, key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("template %q not found in secret %s/d8-cloud-provider-%s-%s",
			key, providerTemplateSecretNamespace, cloudType, engine)
	}
	return data, nil
}

// readProviderTemplateIfPresent is readProviderTemplate for a contract file whose absence is a
// legitimate answer, such as capi/template.yaml while a provider still uses the legacy machine
// template contract.
func (b *BaseWithReader) readProviderTemplateIfPresent(ctx context.Context, cloudType, engine, key string) ([]byte, bool, error) {
	if cloudType == "" {
		return nil, false, fmt.Errorf("cloud type not set")
	}
	name := fmt.Sprintf("d8-cloud-provider-%s-%s", cloudType, engine)
	secret := &corev1.Secret{}
	if err := b.Client.Get(ctx, types.NamespacedName{Namespace: providerTemplateSecretNamespace, Name: name}, secret); err != nil {
		return nil, false, fmt.Errorf("get provider template secret %s: %w", name, err)
	}
	data, ok := secret.Data[key]
	if !ok {
		return nil, false, nil
	}
	return data, true, nil
}
