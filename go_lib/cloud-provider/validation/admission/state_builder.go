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
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
)

var (
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

// BuildForCredentialSecret returns validation state with the admission credential Secret applied.
func (b *StateBuilder) BuildForCredentialSecret(
	ctx context.Context,
	operation admissionv1.Operation,
	secret cpapi.CredentialSecret,
) (*cpval.State, error) {
	state, err := b.newBaseState(ctx)
	if err != nil {
		return nil, fmt.Errorf("build base state: %w", err)
	}

	if !secret.IsManaged() {
		return state, nil
	}

	state.CredentialSecrets = make([]cpapi.CredentialSecret, 0)

	if operation != admissionv1.Delete {
		state.CredentialSecrets = append(state.CredentialSecrets, secret)
	}

	return state, nil
}

// BuildForNodeGroup returns validation state with the admission NodeGroup applied.
func (b *StateBuilder) BuildForNodeGroup(
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (*cpval.State, error) {
	state, err := b.newBaseState(ctx)
	if err != nil {
		return nil, fmt.Errorf("build base state: %w", err)
	}

	state.NodeGroups = make([]cpapi.NodeGroup, 0)

	if operation != admissionv1.Delete {
		objMap, err := runtimeObjectToMap(obj)
		if err != nil {
			return nil, fmt.Errorf("convert runtime object to map: %w", err)
		}

		nodeGroup, err := cpval.DecodeNodeGroup(objMap)
		if err != nil {
			return nil, fmt.Errorf("decode NodeGroup: %w", err)
		}

		if nodeGroup.Spec.NodeType == cpapi.NodeTypeCloudPermanent {
			state.NodeGroups = append(state.NodeGroups, *nodeGroup)

			instanceClass, err := b.getInstanceClassByNodeGroup(ctx, nodeGroup)
			if err != nil {
				return nil, err
			}
			if instanceClass != nil {
				state.InstanceClasses = []cpapi.InstanceClass{*instanceClass}
			}
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
	state, err := b.newBaseState(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build base state: %w", err)
	}

	objMap, err := runtimeObjectToMap(obj)
	if err != nil {
		return nil, nil, fmt.Errorf("convert runtime object to map: %w", err)
	}

	instanceClass, err := cpval.DecodeInstanceClass(objMap)
	if err != nil {
		return nil, nil, fmt.Errorf("decode %s: %w", b.config.InstanceClassKind, err)
	}

	nodeGroups, err := b.listNodeGroupsByInstanceClass(ctx, instanceClass.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("list NodeGroups referencing %s: %w", b.config.InstanceClassKind, err)
	}
	state.NodeGroups = nodeGroups

	state.InstanceClasses = make([]cpapi.InstanceClass, 0)
	if operation == admissionv1.Delete {
		return state, instanceClass, nil
	}

	state.InstanceClasses = append(state.InstanceClasses, *instanceClass)

	return state, nil, nil
}

func (b *StateBuilder) newBaseState(ctx context.Context) (*cpval.State, error) {
	state := &cpval.State{
		InstanceClassKind: b.config.InstanceClassKind,
		NamespaceName:     b.config.NamespaceName,
		ModuleName:        b.config.ModuleName,
	}

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

func (b *StateBuilder) getInstanceClassByNodeGroup(ctx context.Context, nodeGroup *cpapi.NodeGroup) (*cpapi.InstanceClass, error) {
	if nodeGroup == nil || nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
		return nil, nil
	}

	classRef := nodeGroup.Spec.CloudInstances.ClassReference
	if classRef.Kind != b.config.InstanceClassKind {
		return nil, nil
	}

	className := strings.TrimSpace(classRef.Name)
	if className == "" {
		return nil, nil
	}

	return b.getInstanceClassByName(ctx, className)
}

func (b *StateBuilder) getInstanceClassByName(ctx context.Context, name string) (*cpapi.InstanceClass, error) {
	obj := newUnstructured(b.config.instanceClassGVK())
	err := b.client.Get(ctx, client.ObjectKey{Name: name}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("get %s %q: %w", b.config.InstanceClassKind, name, err)
	}

	instanceClass, err := cpval.DecodeInstanceClass(obj.Object)
	if err != nil {
		return nil, fmt.Errorf("decode %s %q: %w", b.config.InstanceClassKind, name, err)
	}

	return instanceClass, nil
}

func (b *StateBuilder) listNodeGroupsByInstanceClass(ctx context.Context, className string) ([]cpapi.NodeGroup, error) {
	className = strings.TrimSpace(className)
	if className == "" {
		return nil, nil
	}

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

		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		classRef := nodeGroup.Spec.CloudInstances.ClassReference
		if classRef.Kind != b.config.InstanceClassKind || classRef.Name != className {
			continue
		}

		result = append(result, *nodeGroup)
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
