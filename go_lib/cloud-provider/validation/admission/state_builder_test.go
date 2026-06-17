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
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func testStateBuilderConfig() StateBuilderConfig {
	return StateBuilderConfig{
		ModuleName:            "cloud-provider-test",
		NamespaceName:         "d8-cloud-provider-test",
		InstanceClassKind:     "TestInstanceClass",
	}
}

func TestRuntimeStateBuilderListCredentialSecretsIgnoresOrdinaryModuleSecrets(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cpapi.CredentialSecretName,
				Namespace: cfg.NamespaceName,
			},
			Type: cpapi.CredentialsSecretType,
			StringData: map[string]string{
				cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeKubeconfig),
				cpapi.CredentialSecretSecretKey:     "token",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "validation-webhook-tls",
				Namespace: cfg.NamespaceName,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"tls.crt": []byte("cert"),
				"tls.key": []byte("key"),
			},
		},
	), cfg)

	state, err := builder.buildBaseState(context.Background())
	if err != nil {
		t.Fatalf("buildBaseState() error = %v", err)
	}

	if len(state.CredentialSecrets) != 1 {
		t.Fatalf("buildBaseState() credential secrets = %d, want 1", len(state.CredentialSecrets))
	}
	if state.CredentialSecrets[0].Name != cpapi.CredentialSecretName {
		t.Fatalf("buildBaseState() credential secret = %q, want %q", state.CredentialSecrets[0].Name, cpapi.CredentialSecretName)
	}
}

func TestRuntimeStateBuilderBuildForCredentialSecretIgnoresOrdinaryModuleSecret(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cpapi.CredentialSecretName,
				Namespace: cfg.NamespaceName,
			},
			Type: cpapi.CredentialsSecretType,
			StringData: map[string]string{
				cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeKubeconfig),
				cpapi.CredentialSecretSecretKey:     "token",
			},
		},
	), cfg)

	state, err := builder.BuildForCredentialSecret(context.Background(), admissionv1.Update, cpapi.CredentialSecret{
		ObjectMeta: cpapi.ObjectMeta{
			Name:      "validation-webhook-tls",
			Namespace: cfg.NamespaceName,
		},
		Type: "kubernetes.io/tls",
	})
	if err != nil {
		t.Fatalf("BuildForCredentialSecret() error = %v", err)
	}

	if len(state.CredentialSecrets) != 1 || state.CredentialSecrets[0].Name != cpapi.CredentialSecretName {
		t.Fatalf("BuildForCredentialSecret() credential secrets = %#v, want only %q", state.CredentialSecrets, cpapi.CredentialSecretName)
	}
}

func TestRuntimeStateBuilderIsMigrationPendingReadsConfigMap(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cpapi.MigrationConfigMapName,
				Namespace: cfg.NamespaceName,
			},
		},
	), cfg)

	pending, err := builder.IsMigrationPending(context.Background())
	if err != nil {
		t.Fatalf("IsMigrationPending() error = %v", err)
	}
	if !pending {
		t.Fatal("IsMigrationPending() = false, want true")
	}
}

func TestConfigInstanceClassGVKs(t *testing.T) {
	t.Parallel()

	cfg := StateBuilderConfig{InstanceClassKind: "TestInstanceClass"}
	if got := cfg.instanceClassGVK(); got.Kind != "TestInstanceClass" || got.Group != "deckhouse.io" {
		t.Fatalf("instanceClassGVK() = %#v", got)
	}
	if got := cfg.instanceClassListGVK(); got.Kind != "TestInstanceClassList" {
		t.Fatalf("instanceClassListGVK() = %#v", got)
	}
}

func TestIsMigrationPendingNotFound(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t), cfg)

	pending, err := builder.IsMigrationPending(context.Background())
	if err != nil {
		t.Fatalf("IsMigrationPending() error = %v", err)
	}
	if pending {
		t.Fatal("IsMigrationPending() = true, want false when ConfigMap is absent")
	}
}

func TestBuildBaseStateLoadsClusterResources(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		testModuleConfigObject(cfg),
		testNodeGroupObject("master", cpapi.NodeTypeCloudPermanent),
		testNodeGroupObject("static", "CloudStatic"),
		testInstanceClassObject(cfg, "master-dvp"),
		testCredentialSecretObject(cfg, cpapi.CredentialSecretName),
	), cfg)

	state, err := builder.buildBaseState(context.Background())
	if err != nil {
		t.Fatalf("buildBaseState() error = %v", err)
	}
	if state.ModuleConfig == nil || state.ModuleConfig.Name != cfg.ModuleName {
		t.Fatalf("buildBaseState() module config = %#v", state.ModuleConfig)
	}
	if len(state.NodeGroups) != 1 || state.NodeGroups[0].Name != "master" {
		t.Fatalf("buildBaseState() node groups = %#v, want only CloudPermanent", state.NodeGroups)
	}
	if len(state.InstanceClasses) != 1 || state.InstanceClasses[0].Name != "master-dvp" {
		t.Fatalf("buildBaseState() instance classes = %#v", state.InstanceClasses)
	}
	if len(state.CredentialSecrets) != 1 {
		t.Fatalf("buildBaseState() credential secrets = %d, want 1", len(state.CredentialSecrets))
	}
}

func TestBuildBaseStateSetsMigrationStatus(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cpapi.MigrationConfigMapName,
				Namespace: cfg.NamespaceName,
			},
		},
	), cfg)

	state, err := builder.buildBaseState(context.Background())
	if err != nil {
		t.Fatalf("buildBaseState() error = %v", err)
	}
	if !state.MigrationStatus.MigrationPending || !state.MigrationStatus.LegacyPCCPresent {
		t.Fatalf("buildBaseState() migration status = %#v", state.MigrationStatus)
	}
}

func TestBuildForModuleConfigDeleteClearsModuleConfig(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t, testModuleConfigObject(cfg)), cfg)

	state, err := builder.BuildForModuleConfig(context.Background(), admissionv1.Delete, testModuleConfigObject(cfg))
	if err != nil {
		t.Fatalf("BuildForModuleConfig() error = %v", err)
	}
	if state.ModuleConfig != nil {
		t.Fatalf("BuildForModuleConfig(delete) module config = %#v, want nil", state.ModuleConfig)
	}
}

func TestBuildForModuleConfigUpdateUsesAdmissionObject(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t), cfg)

	updated := testModuleConfigObject(cfg)
	updated.Object["spec"] = map[string]any{"enabled": true, "version": int64(3)}
	state, err := builder.BuildForModuleConfig(context.Background(), admissionv1.Update, updated)
	if err != nil {
		t.Fatalf("BuildForModuleConfig() error = %v", err)
	}
	if state.ModuleConfig == nil || state.ModuleConfig.Spec.Version != 3 {
		t.Fatalf("BuildForModuleConfig() module config = %#v", state.ModuleConfig)
	}
}

func TestBuildForCredentialSecretUpsertAndDelete(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		testCredentialSecretObject(cfg, cpapi.CredentialSecretName),
	), cfg)

	updated := cpapi.CredentialSecret{
		ObjectMeta: cpapi.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: cfg.NamespaceName},
		Type:       cpapi.CredentialsSecretType,
		StringData: cpapi.CredentialSecretStringData{
			AuthScheme: cpapi.AuthSchemeKubeconfig,
			Secret:     "updated",
		},
	}
	state, err := builder.BuildForCredentialSecret(context.Background(), admissionv1.Update, updated)
	if err != nil {
		t.Fatalf("BuildForCredentialSecret(update) error = %v", err)
	}
	if state.CredentialSecrets[0].StringData.Secret != "updated" {
		t.Fatalf("BuildForCredentialSecret(update) secret = %#v", state.CredentialSecrets[0])
	}

	state, err = builder.BuildForCredentialSecret(context.Background(), admissionv1.Delete, updated)
	if err != nil {
		t.Fatalf("BuildForCredentialSecret(delete) error = %v", err)
	}
	if len(state.CredentialSecrets) != 0 {
		t.Fatalf("BuildForCredentialSecret(delete) secrets = %#v, want empty", state.CredentialSecrets)
	}
}

func TestBuildForNodeGroupCreateAddsCloudPermanent(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		testNodeGroupObject("master", cpapi.NodeTypeCloudPermanent),
	), cfg)

	worker := testNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent)
	state, err := builder.BuildForNodeGroup(context.Background(), admissionv1.Create, worker)
	if err != nil {
		t.Fatalf("BuildForNodeGroup(create) error = %v", err)
	}
	if len(state.NodeGroups) != 2 {
		t.Fatalf("BuildForNodeGroup(create) node groups = %#v, want 2", state.NodeGroups)
	}
}

func TestBuildForNodeGroupUpdateRemovesNonCloudPermanent(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		testNodeGroupObject("master", cpapi.NodeTypeCloudPermanent),
		testNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent),
	), cfg)

	static := testNodeGroupObject("master", "CloudStatic")
	state, err := builder.BuildForNodeGroup(context.Background(), admissionv1.Update, static)
	if err != nil {
		t.Fatalf("BuildForNodeGroup(update static) error = %v", err)
	}
	if len(state.NodeGroups) != 1 || state.NodeGroups[0].Name != "worker" {
		t.Fatalf("BuildForNodeGroup(update static) node groups = %#v, want only worker", state.NodeGroups)
	}
}

func TestBuildForNodeGroupDeleteRemovesGroup(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		testNodeGroupObject("master", cpapi.NodeTypeCloudPermanent),
		testNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent),
	), cfg)

	state, err := builder.BuildForNodeGroup(context.Background(), admissionv1.Delete, testNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent))
	if err != nil {
		t.Fatalf("BuildForNodeGroup(delete) error = %v", err)
	}
	if len(state.NodeGroups) != 1 || state.NodeGroups[0].Name != "master" {
		t.Fatalf("BuildForNodeGroup(delete) node groups = %#v, want only master", state.NodeGroups)
	}
}

func TestBuildForInstanceClassOperations(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		testInstanceClassObject(cfg, "master-dvp"),
	), cfg)

	created := testInstanceClassObject(cfg, "worker-dvp")
	state, deleted, err := builder.BuildForInstanceClass(context.Background(), admissionv1.Create, created)
	if err != nil {
		t.Fatalf("BuildForInstanceClass(create) error = %v", err)
	}
	if deleted != nil || len(state.InstanceClasses) != 2 {
		t.Fatalf("BuildForInstanceClass(create) classes = %#v, deleted = %#v, want 2 classes", state.InstanceClasses, deleted)
	}

	toDelete := testInstanceClassObject(cfg, "worker-dvp")
	state, deleted, err = builder.BuildForInstanceClass(context.Background(), admissionv1.Delete, toDelete)
	if err != nil {
		t.Fatalf("BuildForInstanceClass(delete) error = %v", err)
	}
	if deleted == nil || deleted.Name != "worker-dvp" || len(state.InstanceClasses) != 1 {
		t.Fatalf("BuildForInstanceClass(delete) classes = %#v, deleted = %#v", state.InstanceClasses, deleted)
	}
}

func TestBuildForNodeGroupInvalidObject(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t), cfg)

	_, err := builder.BuildForNodeGroup(context.Background(), admissionv1.Update, &metav1.Status{})
	if err == nil || !strings.Contains(err.Error(), "get NodeGroup metadata") {
		t.Fatalf("BuildForNodeGroup() error = %v, want metadata error", err)
	}
}

func TestRuntimeObjectHelpers(t *testing.T) {
	t.Parallel()

	if _, err := runtimeObjectToMap(nil); err != nil {
		t.Fatalf("runtimeObjectToMap(nil) error = %v", err)
	}

	obj := &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": "x"}}}
	got, err := runtimeObjectToMap(obj)
	if err != nil || got["metadata"] == nil {
		t.Fatalf("runtimeObjectToMap(unstructured) = %#v, err = %v", got, err)
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s"}}
	got, err = runtimeObjectToMap(secret)
	if err != nil || got["metadata"] == nil {
		t.Fatalf("runtimeObjectToMap(typed) = %#v, err = %v", got, err)
	}
}

func TestBuildForInstanceClassUpdateUpsertsExisting(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		testInstanceClassObject(cfg, "master-dvp"),
	), cfg)

	updated := testInstanceClassObject(cfg, "master-dvp")
	updated.Object["spec"] = map[string]any{"rootDiskSize": int64(50)}
	state, deleted, err := builder.BuildForInstanceClass(context.Background(), admissionv1.Update, updated)
	if err != nil {
		t.Fatalf("BuildForInstanceClass(update) error = %v", err)
	}
	if deleted != nil || len(state.InstanceClasses) != 1 {
		t.Fatalf("BuildForInstanceClass(update) classes = %#v, deleted = %#v", state.InstanceClasses, deleted)
	}
}

func TestBuildForNodeGroupUpdateUpsertsExisting(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		testNodeGroupObject("master", cpapi.NodeTypeCloudPermanent),
	), cfg)

	updated := testNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)
	updated.Object["spec"] = map[string]any{
		"nodeType": string(cpapi.NodeTypeCloudPermanent),
		"cloudInstances": map[string]any{
			"classReference": map[string]any{"kind": cfg.InstanceClassKind, "name": "master-dvp"},
		},
	}
	state, err := builder.BuildForNodeGroup(context.Background(), admissionv1.Update, updated)
	if err != nil {
		t.Fatalf("BuildForNodeGroup(update) error = %v", err)
	}
	if len(state.NodeGroups) != 1 {
		t.Fatalf("BuildForNodeGroup(update) node groups = %#v, want 1", state.NodeGroups)
	}
}

func TestBuildForCredentialSecretUpsertsExisting(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t,
		testCredentialSecretObject(cfg, cpapi.CredentialSecretName),
	), cfg)

	updated := testCredentialSecretValue(cfg, cpapi.CredentialSecretName)
	updated.StringData.Secret = "rotated"
	state, err := builder.BuildForCredentialSecret(context.Background(), admissionv1.Update, updated)
	if err != nil {
		t.Fatalf("BuildForCredentialSecret(update existing) error = %v", err)
	}
	if state.CredentialSecrets[0].StringData.Secret != "rotated" {
		t.Fatalf("BuildForCredentialSecret(update existing) = %#v", state.CredentialSecrets)
	}
}

func TestIsMigrationPendingReturnsError(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	cli := clientfake.NewClientBuilder().
		WithScheme(mustTestScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if key.Name == cpapi.MigrationConfigMapName {
					return fmt.Errorf("apiserver unavailable")
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	builder := NewStateBuilder(cli, cfg)
	if _, err := builder.IsMigrationPending(context.Background()); err == nil || !strings.Contains(err.Error(), "get migration ConfigMap") {
		t.Fatalf("IsMigrationPending() error = %v", err)
	}
}

func TestBuildForModuleConfigDecodeError(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t), cfg)

	broken := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": cfg.ModuleName},
		"spec":     "invalid",
	}}
	if _, err := builder.BuildForModuleConfig(context.Background(), admissionv1.Create, broken); err == nil {
		t.Fatal("BuildForModuleConfig() error = nil, want decode error")
	}
}

func TestBuildForInstanceClassInvalidMetadata(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t), cfg)

	_, _, err := builder.BuildForInstanceClass(context.Background(), admissionv1.Create, &metav1.Status{})
	if err == nil || !strings.Contains(err.Error(), "get TestInstanceClass metadata") {
		t.Fatalf("BuildForInstanceClass() error = %v, want metadata error", err)
	}
}

func TestBuildForInstanceClassDecodeErrorOnCreate(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t), cfg)

	broken := testInstanceClassObject(cfg, "broken-dvp")
	broken.Object["spec"] = "invalid"
	if _, _, err := builder.BuildForInstanceClass(context.Background(), admissionv1.Create, broken); err == nil {
		t.Fatal("BuildForInstanceClass(create) error = nil, want decode error")
	}
}

func TestBuildForInstanceClassDecodeErrorOnDelete(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	builder := NewStateBuilder(newRuntimeBuilderTestClient(t), cfg)

	broken := testInstanceClassObject(cfg, "broken-dvp")
	broken.Object["spec"] = "invalid"
	if _, _, err := builder.BuildForInstanceClass(context.Background(), admissionv1.Delete, broken); err == nil ||
		!strings.Contains(err.Error(), "decode TestInstanceClass") {
		t.Fatalf("BuildForInstanceClass(delete) error = %v, want decode error", err)
	}
}

func mustTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	return scheme
}

func testModuleConfigObject(cfg StateBuilderConfig) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"})
	obj.SetName(cfg.ModuleName)
	obj.Object["spec"] = map[string]any{"enabled": true, "version": int64(2)}
	return obj
}

func testNodeGroupObject(name string, nodeType cpapi.NodeType) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"})
	obj.SetName(name)
	obj.Object["spec"] = map[string]any{"nodeType": string(nodeType)}
	return obj
}

func testInstanceClassObject(cfg StateBuilderConfig, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(cfg.instanceClassGVK())
	obj.SetName(name)
	return obj
}

func testCredentialSecretValue(cfg StateBuilderConfig, name string) cpapi.CredentialSecret {
	return cpapi.CredentialSecret{
		ObjectMeta: cpapi.ObjectMeta{Name: name, Namespace: cfg.NamespaceName},
		Type:       cpapi.CredentialsSecretType,
		StringData: cpapi.CredentialSecretStringData{
			AuthScheme: cpapi.AuthSchemeKubeconfig,
			Secret:     "token",
		},
	}
}

func testCredentialSecretObject(cfg StateBuilderConfig, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: cfg.NamespaceName},
		Type:       cpapi.CredentialsSecretType,
		StringData: map[string]string{
			cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeKubeconfig),
			cpapi.CredentialSecretSecretKey:     "token",
		},
	}
}

func newRuntimeBuilderTestClient(t *testing.T, objects ...runtime.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	return clientfake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
}
