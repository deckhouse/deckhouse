// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	otattribute "go.opentelemetry.io/otel/attribute"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

type CloudProviderVars = providerdata.CloudProviderVars

const CloudProviderCredentialsSecretType = corev1.SecretType(providerdata.CloudProviderCredentialsSecretType)

var nodeGroupGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}

const (
	instanceClassAPIGroup = "deckhouse.io"
)

// cloudProviderNamespace returns the canonical d8-cloud-provider-<name>
// namespace that hosts the provider's ModuleConfig, credential Secret and
// other module-owned resources. All Deckhouse cloud-provider modules follow
// this naming, so the helper is the single source of truth.
func cloudProviderNamespace(providerName string) string {
	return "d8-" + providerdata.CloudProviderModuleName(providerName)
}

// CloudProviderVarsFromCluster fetches NodeGroups, InstanceClasses and credential
// Secrets from the cluster. Settings is intentionally left empty here — it is
// later populated by metaConfig.applyCloudProviderModuleSettings from the
// cloud-provider-<name> ModuleConfig loaded into metaConfig.ModuleConfigs,
// keeping the cluster-side and bootstrap-from-file flows symmetric.
func CloudProviderVarsFromCluster(ctx context.Context, kubeCl *client.KubernetesClient, providerName string) (*providerdata.CloudProviderVars, error) {
	ctx, span := telemetry.StartSpan(ctx, "CloudProviderVarsFromCluster")
	defer span.End()

	span.SetAttributes(otattribute.String("provider.name", providerName))

	nodeGroups, err := fetchCloudPermanentNodeGroupsFromCluster(ctx, kubeCl)
	if err != nil {
		return nil, err
	}

	instanceClasses, err := fetchInstanceClassesFromCluster(ctx, kubeCl, providerName)
	if err != nil {
		return nil, err
	}

	secrets, err := fetchCredentialSecretsFromCluster(ctx, kubeCl, providerName)
	if err != nil {
		return nil, err
	}

	span.SetAttributes(
		otattribute.Int("provider.nodeGroupsCount", len(nodeGroups)),
		otattribute.Int("provider.instanceClassesCount", len(instanceClasses)),
		otattribute.Int("provider.secretsCount", len(secrets)),
	)

	return &providerdata.CloudProviderVars{
		NodeGroups:      nodeGroups,
		InstanceClasses: instanceClasses,
		Secrets:         secrets,
	}, nil
}

func fetchCloudPermanentNodeGroupsFromCluster(ctx context.Context, kubeCl *client.KubernetesClient) (map[string]map[string]interface{}, error) {
	list, err := kubeCl.Dynamic().Resource(nodeGroupGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list node groups: %w", err)
	}

	result := make(map[string]map[string]interface{})
	for _, item := range list.Items {
		if !providerdata.IsCloudPermanentNodeGroup(item.Object) {
			continue
		}
		name := item.GetName()
		if name != "" {
			result[name] = item.Object
		}
	}
	return result, nil
}

// instanceClassAPIVersions lists the API versions to probe for
// <provider>instanceclasses. Legacy providers (yandex, vcd, aws, ...) register
// under v1; newer external providers (DVP) ship as v1alpha1. We try v1 first
// to keep the happy path one round-trip for the common case.
var instanceClassAPIVersions = []string{"v1", "v1alpha1"}

func fetchInstanceClassesFromCluster(ctx context.Context, kubeCl *client.KubernetesClient, providerName string) (map[string]map[string]interface{}, error) {
	if providerName == "" {
		return nil, nil
	}

	resource := strings.ToLower(providerName) + "instanceclasses"
	for _, version := range instanceClassAPIVersions {
		gvr := schema.GroupVersionResource{Group: instanceClassAPIGroup, Version: version, Resource: resource}
		list, err := kubeCl.Dynamic().Resource(gvr).List(ctx, metav1.ListOptions{})
		if err == nil {
			result := make(map[string]map[string]interface{}, len(list.Items))
			for _, item := range list.Items {
				if name := item.GetName(); name != "" {
					result[name] = item.Object
				}
			}
			return result, nil
		}
		if !apierrors.IsNotFound(err) && !apierrors.IsMethodNotSupported(err) {
			return nil, fmt.Errorf("list instance classes for provider %s (version %s): %w", providerName, version, err)
		}
	}
	return nil, nil
}

// fetchCredentialSecretsFromCluster lists provider credential Secrets from
// the provider's canonical d8-cloud-provider-<name> namespace. Both the
// namespace scoping and the apiserver-side FieldSelector on Secret.type
// (an indexed field) keep the query narrow, so least-privilege RBAC granting
// only namespace-scoped list over this Secret type still works.
func fetchCredentialSecretsFromCluster(ctx context.Context, kubeCl *client.KubernetesClient, providerName string) (map[string]map[string]interface{}, error) {
	if providerName == "" {
		return nil, nil
	}
	ns := cloudProviderNamespace(providerName)
	list, err := kubeCl.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{
		FieldSelector: "type=" + string(CloudProviderCredentialsSecretType),
	})
	if err != nil {
		return nil, fmt.Errorf("list credential secrets in %s: %w", ns, err)
	}

	result := make(map[string]map[string]interface{}, len(list.Items))
	for i := range list.Items {
		secret := &list.Items[i]
		key := secret.Namespace + "/" + secret.Name
		result[key] = secretToMap(secret)
	}
	return result, nil
}

func secretToMap(secret *corev1.Secret) map[string]interface{} {
	var stringData, data map[string]string
	for k, v := range secret.Data {
		if utf8.Valid(v) {
			if stringData == nil {
				stringData = make(map[string]string)
			}
			stringData[k] = string(v)
			continue
		}
		if data == nil {
			data = make(map[string]string)
		}
		data[k] = base64.StdEncoding.EncodeToString(v)
	}

	result := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      secret.Name,
			"namespace": secret.Namespace,
		},
		"type": string(secret.Type),
	}
	if stringData != nil {
		result["stringData"] = stringData
	}
	if data != nil {
		result["data"] = data
	}
	return result
}
