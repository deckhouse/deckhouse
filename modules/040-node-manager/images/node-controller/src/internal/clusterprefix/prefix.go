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

// Package clusterprefix is the single source of truth for resolving the cluster
// prefix inside node-controller. The prefix is being migrated from the
// deprecated ClusterConfiguration.cloud.prefix into the global ModuleConfig
// (spec.settings.prefix); every consumer (webhook, CAPI, migration controller)
// must resolve it the same way so they never diverge.
package clusterprefix

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"
)

const (
	GlobalModuleConfigName = "global"

	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigSecretNamespace = "kube-system"
	clusterConfigSecretKey       = "cluster-configuration.yaml"
)

// ModuleConfigGVK is the GVK used to read the global ModuleConfig as unstructured.
func ModuleConfigGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
}

// Resolve returns the effective cluster prefix: the global ModuleConfig
// spec.settings.prefix if set, otherwise the deprecated
// ClusterConfiguration.cloud.prefix from the d8-cluster-configuration secret.
// Returns an empty string when neither is set. The global ModuleConfig read
// short-circuits, so the secret is only read when the ModuleConfig has no prefix.
func Resolve(ctx context.Context, reader client.Reader) (string, error) {
	if prefix, err := FromModuleConfig(ctx, reader); err != nil {
		return "", err
	} else if prefix != "" {
		return prefix, nil
	}
	return FromClusterConfigurationSecret(ctx, reader)
}

// FromModuleConfig returns spec.settings.prefix from the global ModuleConfig, or
// an empty string when the ModuleConfig or the field is absent.
func FromModuleConfig(ctx context.Context, reader client.Reader) (string, error) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(ModuleConfigGVK())
	if err := reader.Get(ctx, types.NamespacedName{Name: GlobalModuleConfigName}, mc); err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get global ModuleConfig: %w", err)
	}
	prefix, _, err := unstructured.NestedString(mc.Object, "spec", "settings", "prefix")
	if err != nil {
		return "", fmt.Errorf("read spec.settings.prefix from global ModuleConfig: %w", err)
	}
	return prefix, nil
}

// FromClusterConfigurationSecret returns ClusterConfiguration.cloud.prefix from
// the d8-cluster-configuration secret, or an empty string when absent.
func FromClusterConfigurationSecret(ctx context.Context, reader client.Reader) (string, error) {
	secret := &corev1.Secret{}
	if err := reader.Get(ctx, types.NamespacedName{
		Name:      clusterConfigSecretName,
		Namespace: clusterConfigSecretNamespace,
	}, secret); err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get secret %s/%s: %w", clusterConfigSecretNamespace, clusterConfigSecretName, err)
	}

	data, ok := secret.Data[clusterConfigSecretKey]
	if !ok {
		return "", nil
	}

	var cfg struct {
		Cloud struct {
			Prefix string `json:"prefix"`
		} `json:"cloud"`
	}
	if err := sigsyaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", clusterConfigSecretKey, err)
	}
	return cfg.Cloud.Prefix, nil
}
