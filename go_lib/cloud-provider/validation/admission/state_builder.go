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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

var (
	nodeGroupListGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroupList"}
	moduleConfigGVK  = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
)

// StateBuilderConfig holds provider-specific settings shared by every state builder.
type StateBuilderConfig struct {
	// InstanceClassGVK is the provider InstanceClass resource group, version, kind.
	InstanceClassGVK schema.GroupVersionKind
	// NamespaceName is the module namespace used for credential Secrets and migration markers.
	NamespaceName string
	// ModuleName is the cloud-provider ModuleConfig name.
	ModuleName string
}

// StateBuilderFactory produces a fresh StateBuilder per admission request.
//
// The factory holds the immutable provider context — client and configuration — while the
// builder it creates owns the mutable state, so concurrent admission requests never share it.
type StateBuilderFactory[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
] struct {
	client client.Client
	config StateBuilderConfig
}

// NewStateBuilderFactory creates a state builder factory for the given provider configuration.
func NewStateBuilderFactory[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
](client client.Client, config StateBuilderConfig) *StateBuilderFactory[IC, S, PCC] {
	return &StateBuilderFactory[IC, S, PCC]{
		client: client,
		config: config,
	}
}

// CreateBuilder returns a builder holding a state with the module identity filled in.
//
// Each surface then adds only the resources its rules inspect:
//
//	factory.CreateBuilder().AddNodeGroup(ctx, obj).AddAssociatedInstanceClasses(ctx, name).Build(ctx)
func (f *StateBuilderFactory[IC, S, PCC]) CreateBuilder() *StateBuilder[IC, S, PCC] {
	return &StateBuilder[IC, S, PCC]{
		client: f.client,
		config: f.config,
		state: &cpvalapi.State[IC, S, PCC]{
			NamespaceName: f.config.NamespaceName,
			ModuleName:    f.config.ModuleName,
		},
	}
}

// StateBuilder assembles a validation State for a single admission request.
//
// Every Add* method returns the builder so steps can be chained, and the first failure is
// remembered and returned by Build: a chain never has to be interrupted by error checks.
// Steps that take the reviewed object accept a context as well, so that every step in a chain
// reads the same, even though only the cluster-reading ones use it.
type StateBuilder[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
] struct {
	client client.Client
	config StateBuilderConfig
	state  *cpvalapi.State[IC, S, PCC]
	err    error
}

// Build returns the assembled state, or the first error a step ran into.
//
// It also resolves the migration status: while the d8-module-is-migrating ConfigMap exists the
// migration is still expected to happen and the new-model resources do not exist yet, so there
// is nothing to validate against and the gate stays closed. Rules check it through
// cpapi.ShouldSkipNewModelValidation.
func (b *StateBuilder[IC, S, PCC]) Build(ctx context.Context) (*cpvalapi.State[IC, S, PCC], error) {
	if b.err != nil {
		return nil, b.err
	}

	migrationPending, err := b.migrationPending(ctx)
	if err != nil {
		return nil, err
	}

	if migrationPending {
		b.state.MigrationStatus = cpapi.MigrationStatus{
			LegacyPCCPresent: true,
			MigrationPending: true,
		}
	}

	return b.state, nil
}

// SetModuleConfig puts the reviewed ModuleConfig into the state.
func (b *StateBuilder[IC, S, PCC]) SetModuleConfig(_ context.Context, obj runtime.Object) *StateBuilder[IC, S, PCC] {
	if b.err != nil {
		return b
	}

	if obj == nil {
		return b
	}

	moduleConfig, err := DecodeModuleConfigObject[S](b.config.ModuleName, obj)
	if err != nil {
		b.err = err
		return b
	}

	b.state.ModuleConfig = moduleConfig

	return b
}

// AddModuleConfig reads the module ModuleConfig from the cluster.
//
// Surfaces whose reviewed object is not the ModuleConfig itself need it when a rule reads
// settings — the NodeGroup surface, for instance, validates node counts against
// settings.nodes.parameters.externalIPAddresses. An absent ModuleConfig leaves the state field
// nil, which the rules report on their own.
func (b *StateBuilder[IC, S, PCC]) AddModuleConfig(ctx context.Context) *StateBuilder[IC, S, PCC] {
	if b.err != nil {
		return b
	}

	obj := newUnstructured(moduleConfigGVK)
	err := b.client.Get(ctx, client.ObjectKey{Name: b.config.ModuleName}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return b
		}

		b.err = fmt.Errorf("get ModuleConfig %q: %w", b.config.ModuleName, err)
		return b
	}

	moduleConfig, err := cpval.DecodeModuleConfig[S](b.config.ModuleName, obj.Object)
	if err != nil {
		b.err = fmt.Errorf("decode ModuleConfig %q: %w", b.config.ModuleName, err)
		return b
	}

	b.state.ModuleConfig = moduleConfig

	return b
}

// SetCredentialSecret puts the reviewed credential Secret into the state.
// Secrets that the module does not manage are ignored.
func (b *StateBuilder[IC, S, PCC]) SetCredentialSecret(_ context.Context, secret *corev1.Secret) *StateBuilder[IC, S, PCC] {
	if b.err != nil {
		return b
	}

	if secret == nil {
		return b
	}

	if len(b.state.CredentialSecrets) == 0 {
		b.state.CredentialSecrets = make([]cpapi.CredentialSecret, 0, 1)
	}

	credentialSecret := secretToCredentialSecret(secret)
	if !credentialSecret.IsManaged() {
		return b
	}

	b.state.CredentialSecrets = append(b.state.CredentialSecrets, credentialSecret)

	return b
}

// SetNodeGroup puts the reviewed NodeGroup into the state.
// Only CloudPermanent NodeGroups reach the state, as State.NodeGroups documents.
func (b *StateBuilder[IC, S, PCC]) SetNodeGroup(_ context.Context, obj runtime.Object) *StateBuilder[IC, S, PCC] {
	if b.err != nil {
		return b
	}

	if obj == nil {
		return b
	}

	if len(b.state.NodeGroups) == 0 {
		b.state.NodeGroups = make([]cpapi.NodeGroup, 0, 1)
	}

	nodeGroup, err := DecodeNodeGroupObject(obj)
	if err != nil {
		b.err = err
		return b
	}

	if nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
		return b
	}

	b.state.NodeGroups = append(b.state.NodeGroups, *nodeGroup)

	return b
}

// AddNodeGroups reads every CloudPermanent NodeGroup from the cluster.
//
// The ModuleConfig surface needs all of them at once: rules that compare settings against node
// groups — external IP addressing, for one — would otherwise see no node groups at all.
func (b *StateBuilder[IC, S, PCC]) AddNodeGroups(ctx context.Context) *StateBuilder[IC, S, PCC] {
	if b.err != nil {
		return b
	}

	list := newUnstructuredList(nodeGroupListGVK)
	if err := b.client.List(ctx, list); err != nil {
		b.err = fmt.Errorf("list NodeGroups: %w", err)
		return b
	}

	for i := range list.Items {
		nodeGroup, err := cpval.DecodeNodeGroup(list.Items[i].Object)
		if err != nil {
			b.err = fmt.Errorf("decode NodeGroup: %w", err)
			return b
		}

		if nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
			continue
		}

		b.state.NodeGroups = append(b.state.NodeGroups, *nodeGroup)
	}

	return b
}

// AddAssociatedNodeGroups reads the CloudPermanent NodeGroups that reference the given
// InstanceClass, so deletion and etcd-disk rules see the class consumers.
func (b *StateBuilder[IC, S, PCC]) AddAssociatedNodeGroups(ctx context.Context, className string) *StateBuilder[IC, S, PCC] {
	if b.err != nil {
		return b
	}

	className = strings.TrimSpace(className)
	if className == "" {
		return b
	}

	list := newUnstructuredList(nodeGroupListGVK)
	if err := b.client.List(ctx, list); err != nil {
		b.err = fmt.Errorf("list NodeGroups: %w", err)
		return b
	}

	for i := range list.Items {
		nodeGroup, err := cpval.DecodeNodeGroup(list.Items[i].Object)
		if err != nil {
			b.err = fmt.Errorf("decode NodeGroup: %w", err)
			return b
		}

		if nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
			continue
		}

		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		classRef := nodeGroup.Spec.CloudInstances.ClassReference
		if classRef.Kind != b.config.InstanceClassGVK.Kind || classRef.Name != className {
			continue
		}

		b.state.NodeGroups = append(b.state.NodeGroups, *nodeGroup)
	}

	return b
}

// SetInstanceClass puts the reviewed InstanceClass into the state.
func (b *StateBuilder[IC, S, PCC]) SetInstanceClass(_ context.Context, obj runtime.Object) *StateBuilder[IC, S, PCC] {
	if b.err != nil {
		return b
	}

	if obj == nil {
		return b
	}

	if b.state.InstanceClasses == nil {
		b.state.InstanceClasses = make([]IC, 0, 1)
	}

	instanceClass, err := DecodeInstanceClassObject[IC](obj)
	if err != nil {
		b.err = fmt.Errorf("decode %s: %w", b.config.InstanceClassGVK.Kind, err)
		return b
	}

	b.state.InstanceClasses = append(b.state.InstanceClasses, instanceClass)

	return b
}

// AddAssociatedInstanceClasses reads the InstanceClass referenced by the given NodeGroup.
//
// A NodeGroup references at most one class, so at most one is added; a reference to another
// provider's kind, an empty name or a missing object leave the state untouched.
func (b *StateBuilder[IC, S, PCC]) AddAssociatedInstanceClasses(ctx context.Context, nodeGroupName string) *StateBuilder[IC, S, PCC] {
	if b.err != nil {
		return b
	}

	if b.state.InstanceClasses == nil {
		b.state.InstanceClasses = make([]IC, 0, 1)
	}

	nodeGroup, found := b.state.FindNodeGroup(nodeGroupName)
	if !found {
		return b
	}

	if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
		return b
	}

	classRef := nodeGroup.Spec.CloudInstances.ClassReference
	if classRef.Kind != b.config.InstanceClassGVK.Kind {
		return b
	}

	className := strings.TrimSpace(classRef.Name)
	if className == "" {
		return b
	}

	obj := newUnstructured(b.config.InstanceClassGVK)
	err := b.client.Get(ctx, client.ObjectKey{Name: className}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return b
		}

		b.err = fmt.Errorf("get %s %q: %w", b.config.InstanceClassGVK.Kind, className, err)
		return b
	}

	instanceClass, err := cpval.DecodeInstanceClass[IC](obj.Object)
	if err != nil {
		b.err = fmt.Errorf("decode %s %q: %w", b.config.InstanceClassGVK.Kind, className, err)
		return b
	}

	b.state.InstanceClasses = append(b.state.InstanceClasses, instanceClass)

	return b
}

func (b *StateBuilder[IC, S, PCC]) migrationPending(ctx context.Context) (bool, error) {
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

// secretToCredentialSecret converts a Kubernetes Secret into a typed CredentialSecret.
func secretToCredentialSecret(secret *corev1.Secret) cpapi.CredentialSecret {
	if secret == nil {
		return cpapi.CredentialSecret{}
	}

	return cpapi.CredentialSecret{
		TypeMeta: cpapi.TypeMeta{
			APIVersion: secret.APIVersion,
			Kind:       secret.Kind,
		},
		ObjectMeta: cpapi.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
		Type: string(secret.Type),
		Data: cpapi.CredentialSecretData{
			AuthScheme: secret.Data[cpapi.CredentialSecretAuthSchemeKey],
			Identity:   secret.Data[cpapi.CredentialSecretIdentityKey],
			Secret:     secret.Data[cpapi.CredentialSecretSecretKey],
		},
		StringData: cpapi.CredentialSecretStringData{
			AuthScheme: cpapi.AuthScheme(secret.StringData[cpapi.CredentialSecretAuthSchemeKey]),
			Identity:   secret.StringData[cpapi.CredentialSecretIdentityKey],
			Secret:     secret.StringData[cpapi.CredentialSecretSecretKey],
		},
	}
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
