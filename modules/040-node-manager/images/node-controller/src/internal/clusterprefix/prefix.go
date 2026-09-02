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
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
//
// The global ModuleConfig is optional and read fail-open (absent -> empty, fall
// through to the secret). The secret is the base source of truth and always
// present in a real cluster, so it is read fail-closed: an unreadable
// configuration surfaces as an error rather than an empty prefix. This matters
// because the prefix is part of every MachineDeployment name; an empty prefix
// read fail-open would make the stale-object prune treat every real
// "<prefix>-<ng>-<hash>" MachineDeployment as stale and delete it. A
// configuration that parses and simply carries no prefix is legitimate and
// returns an empty string. The ModuleConfig read short-circuits, so the secret
// is only read when the ModuleConfig has no prefix.
func Resolve(ctx context.Context, reader client.Reader) (string, error) {
	if prefix, err := FromModuleConfig(ctx, reader); err != nil {
		return "", err
	} else if prefix != "" {
		return prefix, nil
	}
	return FromClusterConfigurationSecret(ctx, reader)
}

// FromModuleConfig returns spec.settings.prefix from the global ModuleConfig, or
// an empty string when the ModuleConfig or the field is absent. The ModuleConfig
// is optional, so a missing object (or a missing CRD/kind) is not an error.
func FromModuleConfig(ctx context.Context, reader client.Reader) (string, error) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(ModuleConfigGVK())
	if err := reader.Get(ctx, types.NamespacedName{Name: GlobalModuleConfigName}, mc); err != nil {
		if errors.IsNotFound(err) || meta.IsNoMatchError(err) || runtime.IsNotRegisteredError(err) {
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
// the d8-cluster-configuration secret. It fails closed: a secret that cannot be
// read or that lacks the cluster-configuration.yaml key is an error, not an
// empty prefix (see Resolve for why). A configuration that parses and simply
// carries no cloud.prefix returns an empty string.
func FromClusterConfigurationSecret(ctx context.Context, reader client.Reader) (string, error) {
	secret := &corev1.Secret{}
	if err := reader.Get(ctx, types.NamespacedName{
		Name:      clusterConfigSecretName,
		Namespace: clusterConfigSecretNamespace,
	}, secret); err != nil {
		return "", fmt.Errorf("get cluster-configuration secret: %w", err)
	}

	raw, ok := secret.Data[clusterConfigSecretKey]
	if !ok {
		return "", fmt.Errorf("cluster-configuration secret has no %s key", clusterConfigSecretKey)
	}

	// The cluster-configuration.yaml value is stored base64-encoded in some
	// installations; fall back to the raw bytes when it is not (plain YAML is
	// never valid base64, so this never corrupts an already-decoded document).
	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		decoded = raw
	}

	var cfg struct {
		Cloud struct {
			Prefix string `json:"prefix"`
		} `json:"cloud"`
	}
	if err := sigsyaml.Unmarshal(decoded, &cfg); err != nil {
		return "", fmt.Errorf("unmarshal cluster configuration: %w", err)
	}
	return cfg.Cloud.Prefix, nil
}
