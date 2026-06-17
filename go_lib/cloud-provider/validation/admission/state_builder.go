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

package admission

import (
	"context"
	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
)

var (
	moduleConfigGVK  = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
	nodeGroupListGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroupList"}
)

// StateBuilderConfig holds provider-specific settings for StateBuilder.
type StateBuilderConfig struct {
	// InstanceClassKind is the provider InstanceClass resource kind.
	InstanceClassKind string
	// NamespaceName is the module namespace used for credential Secrets and migration markers.
	NamespaceName string
	// ModuleName is the cloud-provider ModuleConfig name.
	ModuleName string
}

// instanceClassGVK returns the GroupVersionKind for the configured InstanceClass kind.
func (c StateBuilderConfig) instanceClassGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    c.InstanceClassKind,
	}
}

// instanceClassListGVK returns the GroupVersionKind for the configured InstanceClass list kind.
func (c StateBuilderConfig) instanceClassListGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    c.InstanceClassKind + "List",
	}
}

// StateBuilder loads cluster state and applies admission object changes on top of it.
type StateBuilder struct {
	client client.Client
	config StateBuilderConfig
}

// NewStateBuilder creates a state builder for the given provider configuration.
func NewStateBuilder(client client.Client, config StateBuilderConfig) *StateBuilder {
	return &StateBuilder{
		client: client,
		config: config,
	}
}

// IsMigrationPending reports whether the migration marker ConfigMap is present in the module namespace.
func (b *StateBuilder) IsMigrationPending(ctx context.Context) (bool, error) {
	// Runtime admission uses the migration marker ConfigMap created by the module hook
	// while ProviderClusterConfiguration is still present. The dhctl validator instead
	// derives MigrationStatus from the incoming PCC payload and resource completeness.
	cm := &corev1.ConfigMap{}
	err := b.client.Get(
		ctx, client.ObjectKey{
			Namespace: b.config.NamespaceName,
			Name:      cpapi.MigrationConfigMapName,
		}, cm,
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("get migration ConfigMap: %w", err)
	}

	return true, nil
}

// BuildForModuleConfig returns validation state with the admission ModuleConfig applied.
func (b *StateBuilder) BuildForModuleConfig(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (*cpval.State, error) {
	state, err := b.buildBaseState(ctx)
	if err != nil {
		return nil, fmt.Errorf("build base state: %w", err)
	}

	switch operation {
	case admissionv1.Delete:
		state.ModuleConfig = nil
	default:
		objMap, err := runtimeObjectToMap(obj)
		if err != nil {
			return nil, fmt.Errorf("convert runtime object to map: %w", err)
		}

		moduleConfig, err := cpval.DecodeModuleConfig(objMap)
		if err != nil {
			return nil, fmt.Errorf("decode ModuleConfig: %w", err)
		}

		state.ModuleConfig = moduleConfig
	}

	return state, nil
}

// BuildForCredentialSecret returns validation state with the admission credential Secret applied.
func (b *StateBuilder) BuildForCredentialSecret(
	ctx context.Context,
	operation admissionv1.Operation,
	secret cpapi.CredentialSecret,
) (*cpval.State, error) {
	state, err := b.buildBaseState(ctx)
	if err != nil {
		return nil, fmt.Errorf("build base state: %w", err)
	}

	if !secret.IsManaged() {
		return state, nil
	}

	switch operation {
	case admissionv1.Delete:
		state.CredentialSecrets = removeCredentialSecret(
			state.CredentialSecrets,
			secret.Name,
			secret.Namespace,
		)
	default:
		state.CredentialSecrets = upsertCredentialSecret(state.CredentialSecrets, secret)
	}

	return state, nil
}

// BuildForNodeGroup returns validation state with the admission NodeGroup applied.
func (b *StateBuilder) BuildForNodeGroup(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (*cpval.State, error) {
	state, err := b.buildBaseState(ctx)
	if err != nil {
		return nil, fmt.Errorf("build base state: %w", err)
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("get NodeGroup metadata: %w", err)
	}

	switch operation {
	case admissionv1.Delete:
		state.NodeGroups = removeNodeGroup(state.NodeGroups, accessor.GetName())
	default:
		objMap, err := runtimeObjectToMap(obj)
		if err != nil {
			return nil, fmt.Errorf("convert runtime object to map: %w", err)
		}

		nodeGroup, err := cpval.DecodeNodeGroup(objMap)
		if err != nil {
			return nil, fmt.Errorf("decode NodeGroup: %w", err)
		}

		if nodeGroup.Spec.NodeType == cpapi.NodeTypeCloudPermanent {
			state.NodeGroups = upsertNodeGroup(state.NodeGroups, *nodeGroup)
		} else {
			state.NodeGroups = removeNodeGroup(state.NodeGroups, nodeGroup.Name)
		}
	}

	return state, nil
}

// BuildForInstanceClass returns validation state with the admission InstanceClass applied.
func (b *StateBuilder) BuildForInstanceClass(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (*cpval.State, *cpapi.InstanceClass, error) {
	state, err := b.buildBaseState(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build base state: %w", err)
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, nil, fmt.Errorf("get %s metadata: %w", b.config.InstanceClassKind, err)
	}

	objMap, err := runtimeObjectToMap(obj)
	if err != nil {
		return nil, nil, fmt.Errorf("convert runtime object to map: %w", err)
	}

	instanceClass, err := cpval.DecodeInstanceClass(objMap)
	if err != nil {
		return nil, nil, fmt.Errorf("decode %s: %w", b.config.InstanceClassKind, err)
	}

	switch operation {
	case admissionv1.Delete:
		state.InstanceClasses = removeInstanceClass(state.InstanceClasses, accessor.GetName())
		return state, instanceClass, nil
	default:
		state.InstanceClasses = upsertInstanceClass(state.InstanceClasses, *instanceClass)
		return state, nil, nil
	}
}

func (b *StateBuilder) buildBaseState(ctx context.Context) (*cpval.State, error) {
	state := &cpval.State{
		InstanceClassKind: b.config.InstanceClassKind,
		NamespaceName:     b.config.NamespaceName,
		ModuleName:        b.config.ModuleName,
	}

	moduleConfig, err := b.getModuleConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get ModuleConfig: %w", err)
	}
	state.ModuleConfig = moduleConfig

	secrets, err := b.listCredentialSecrets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list credential secrets: %w", err)
	}
	state.CredentialSecrets = secrets

	nodeGroups, err := b.listCloudPermanentNodeGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("list CloudPermanent NodeGroups: %w", err)
	}
	state.NodeGroups = nodeGroups

	instanceClasses, err := b.listInstanceClasses(ctx)
	if err != nil {
		return nil, fmt.Errorf("list InstanceClasses with kind %s: %w", b.config.InstanceClassKind, err)
	}
	state.InstanceClasses = instanceClasses

	migrationPending, err := b.IsMigrationPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("check migration pending: %w", err)
	}

	if migrationPending {
		state.MigrationStatus = cpapi.MigrationStatus{
			LegacyPCCPresent: true,
			MigrationPending: true,
		}
	}

	return state, nil
}

func (b *StateBuilder) getModuleConfig(ctx context.Context) (*cpapi.ModuleConfig, error) {
	obj := newUnstructured(moduleConfigGVK)
	err := b.client.Get(ctx, client.ObjectKey{Name: b.config.ModuleName}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ModuleConfig: %w", err)
	}

	moduleConfig, err := cpval.DecodeModuleConfig(obj.Object)
	if err != nil {
		return nil, fmt.Errorf("decode ModuleConfig: %w", err)
	}

	return moduleConfig, nil
}

func (b *StateBuilder) listCredentialSecrets(ctx context.Context) ([]cpapi.CredentialSecret, error) {
	list := &corev1.SecretList{}
	if err := b.client.List(ctx, list, client.InNamespace(b.config.NamespaceName)); err != nil {
		return nil, fmt.Errorf("list credential Secrets: %w", err)
	}

	result := make([]cpapi.CredentialSecret, 0, len(list.Items))
	for i := range list.Items {
		secret := &list.Items[i]
		if !IsManagedCredentialSecret(secret) {
			continue
		}

		result = append(result, SecretToCredentialSecret(secret))
	}

	return result, nil
}

func (b *StateBuilder) listCloudPermanentNodeGroups(ctx context.Context) ([]cpapi.NodeGroup, error) {
	list := newUnstructuredList(nodeGroupListGVK)
	if err := b.client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list NodeGroups: %w", err)
	}

	result := make([]cpapi.NodeGroup, 0, len(list.Items))
	for i := range list.Items {
		nodeGroup, err := cpval.DecodeNodeGroup(list.Items[i].Object)
		if err != nil {
			return nil, fmt.Errorf("decode NodeGroup: %w", err)
		}

		if nodeGroup.Spec.NodeType == cpapi.NodeTypeCloudPermanent {
			result = append(result, *nodeGroup)
		}
	}

	return result, nil
}

func (b *StateBuilder) listInstanceClasses(ctx context.Context) ([]cpapi.InstanceClass, error) {
	list := newUnstructuredList(b.config.instanceClassListGVK())
	if err := b.client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list %s: %w", b.config.InstanceClassKind+"List", err)
	}

	result := make([]cpapi.InstanceClass, 0, len(list.Items))
	for i := range list.Items {
		instanceClass, err := cpval.DecodeInstanceClass(list.Items[i].Object)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", b.config.InstanceClassKind, err)
		}

		result = append(result, *instanceClass)
	}

	return result, nil
}

func newUnstructured(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	return obj
}

func newUnstructuredList(gvk schema.GroupVersionKind) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)
	return list
}

func runtimeObjectToMap(obj runtime.Object) (map[string]any, error) {
	if obj == nil {
		return nil, nil
	}

	if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
		return unstructuredObj.Object, nil
	}

	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal runtime object: %w", err)
	}

	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("unmarshal runtime object: %w", err)
	}

	return object, nil
}

func upsertCredentialSecret(items []cpapi.CredentialSecret, item cpapi.CredentialSecret) []cpapi.CredentialSecret {
	for i := range items {
		if items[i].Name == item.Name && items[i].Namespace == item.Namespace {
			items[i] = item
			return items
		}
	}

	return append(items, item)
}

func removeCredentialSecret(items []cpapi.CredentialSecret, name, namespace string) []cpapi.CredentialSecret {
	result := make([]cpapi.CredentialSecret, 0, len(items))
	for _, item := range items {
		if item.Name == name && item.Namespace == namespace {
			continue
		}
		result = append(result, item)
	}

	return result
}

func upsertNodeGroup(items []cpapi.NodeGroup, item cpapi.NodeGroup) []cpapi.NodeGroup {
	for i := range items {
		if items[i].Name == item.Name {
			items[i] = item
			return items
		}
	}

	return append(items, item)
}

func removeNodeGroup(items []cpapi.NodeGroup, name string) []cpapi.NodeGroup {
	result := make([]cpapi.NodeGroup, 0, len(items))
	for _, item := range items {
		if item.Name == name {
			continue
		}
		result = append(result, item)
	}

	return result
}

func upsertInstanceClass(items []cpapi.InstanceClass, item cpapi.InstanceClass) []cpapi.InstanceClass {
	for i := range items {
		if items[i].Name == item.Name {
			items[i] = item
			return items
		}
	}

	return append(items, item)
}

func removeInstanceClass(items []cpapi.InstanceClass, name string) []cpapi.InstanceClass {
	result := make([]cpapi.InstanceClass, 0, len(items))
	for _, item := range items {
		if item.Name == name {
			continue
		}
		result = append(result, item)
	}

	return result
}
