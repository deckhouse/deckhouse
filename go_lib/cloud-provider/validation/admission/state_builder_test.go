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
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	"github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/internal/testprovider"
)

type testStateBuilderFactory = StateBuilderFactory[*testprovider.InstanceClass, *testprovider.Settings, *testprovider.ProviderClusterConfig]

type testState = cpvalapi.State[*testprovider.InstanceClass, *testprovider.Settings, *testprovider.ProviderClusterConfig]

func newTestFactory(c client.Client, config StateBuilderConfig) *testStateBuilderFactory {
	return NewStateBuilderFactory[*testprovider.InstanceClass, *testprovider.Settings, *testprovider.ProviderClusterConfig](c, config)
}

// The helpers below assemble the same states the webhook surfaces do, so the specs describe one
// surface each instead of repeating the chain.

func buildForCredentialSecret(
	factory *testStateBuilderFactory,
	ctx context.Context,
	operation admissionv1.Operation,
	secret *corev1.Secret,
) (*testState, error) {
	builder := factory.CreateBuilder()
	if operation != admissionv1.Delete {
		builder = builder.SetCredentialSecret(ctx, secret)
	}

	return builder.Build(ctx)
}

func buildForNodeGroup(
	factory *testStateBuilderFactory,
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (*testState, error) {
	builder := factory.CreateBuilder()
	if operation != admissionv1.Delete {
		nodeGroup, err := DecodeNodeGroupObject(obj)
		if err != nil {
			return nil, err
		}

		builder = builder.
			SetNodeGroup(ctx, obj).
			AddAssociatedInstanceClasses(ctx, nodeGroup.Name)
	}

	return builder.Build(ctx)
}

func buildForInstanceClass(
	factory *testStateBuilderFactory,
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (*testState, *testprovider.InstanceClass, error) {
	instanceClass, err := DecodeInstanceClassObject[*testprovider.InstanceClass](obj)
	if err != nil {
		return nil, nil, err
	}

	builder := factory.CreateBuilder().AddAssociatedNodeGroups(ctx, instanceClass.GetName())

	var deletedClass *testprovider.InstanceClass
	if operation == admissionv1.Delete {
		deletedClass = instanceClass
	} else {
		builder = builder.SetInstanceClass(ctx, obj)
	}

	state, err := builder.Build(ctx)
	if err != nil {
		return nil, nil, err
	}

	return state, deletedClass, nil
}

func buildForModuleConfig(
	factory *testStateBuilderFactory,
	ctx context.Context,
	operation admissionv1.Operation,
	obj runtime.Object,
) (*testState, error) {
	builder := factory.CreateBuilder()
	if operation != admissionv1.Delete {
		builder = builder.SetModuleConfig(ctx, obj)
	}

	return builder.Build(ctx)
}

func testStateBuilderConfig() StateBuilderConfig {
	return StateBuilderConfig{
		ModuleName:    "cloud-provider-test",
		NamespaceName: "d8-cloud-provider-test",
		InstanceClassGVK: schema.GroupVersionKind{
			Group:   "deckhouse.io",
			Version: "v1",
			Kind:    "TestInstanceClass",
		},
	}
}

func TestBuildForCredentialSecretIgnoresNonManagedSecret(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	state, err := buildForCredentialSecret(factory, context.Background(), admissionv1.Update, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "validation-webhook-tls",
			Namespace: cfg.NamespaceName,
		},
		Type: "kubernetes.io/tls",
	})
	if err != nil {
		t.Fatalf("BuildForCredentialSecret() error = %v", err)
	}

	if len(state.CredentialSecrets) != 0 {
		t.Fatalf("BuildForCredentialSecret() credential secrets = %#v, want none", state.CredentialSecrets)
	}
}

func TestBuildForCredentialSecretUsesAdmissionObjectOnly(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t,
		testCredentialSecretObject(cfg, "other-credentials"),
	), cfg)

	state, err := buildForCredentialSecret(factory, context.Background(), admissionv1.Update, testCredentialSecretObject(cfg, cpapi.CredentialSecretName))
	if err != nil {
		t.Fatalf("BuildForCredentialSecret() error = %v", err)
	}

	if len(state.CredentialSecrets) != 1 || state.CredentialSecrets[0].Name != cpapi.CredentialSecretName {
		t.Fatalf("BuildForCredentialSecret() credential secrets = %#v, want admitted secret only", state.CredentialSecrets)
	}
}

// The migration gate is observable through the built state: while the marker ConfigMap exists,
// Build reports a pending migration and the rules skip new-model validation.
func TestBuildReportsPendingMigrationFromConfigMap(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cpapi.MigrationConfigMapName,
				Namespace: cfg.NamespaceName,
			},
		},
	), cfg)

	state, err := factory.CreateBuilder().Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !state.MigrationStatus.MigrationPending || !state.MigrationStatus.LegacyPCCPresent {
		t.Fatalf("Build() migration status = %#v, want pending migration", state.MigrationStatus)
	}
	if !cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		t.Fatal("ShouldSkipNewModelValidation() = false, want true while the marker ConfigMap exists")
	}
}

func TestBuildReportsNoMigrationWithoutConfigMap(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	state, err := factory.CreateBuilder().Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if state.MigrationStatus != (cpapi.MigrationStatus{}) {
		t.Fatalf("Build() migration status = %#v, want empty when the ConfigMap is absent", state.MigrationStatus)
	}
}

func TestBuildForNodeGroupLoadsReferencedInstanceClass(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t,
		testInstanceClassObject(cfg, "master-dvp"),
	), cfg)

	state, err := buildForNodeGroup(factory,
		context.Background(),
		admissionv1.Update,
		testNodeGroupWithClassRef(cfg, "master", cpapi.NodeTypeCloudPermanent, "master-dvp"),
	)
	if err != nil {
		t.Fatalf("BuildForNodeGroup() error = %v", err)
	}
	if len(state.NodeGroups) != 1 || state.NodeGroups[0].Name != "master" {
		t.Fatalf("BuildForNodeGroup() node groups = %#v, want admitted master", state.NodeGroups)
	}
	if len(state.InstanceClasses) != 1 || state.InstanceClasses[0].GetName() != "master-dvp" {
		t.Fatalf("BuildForNodeGroup() instance classes = %#v", state.InstanceClasses)
	}
}

func TestBuildForCredentialSecretUpsertAndDelete(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t,
		testCredentialSecretObject(cfg, cpapi.CredentialSecretName),
	), cfg)

	updated := testCredentialSecretObject(cfg, cpapi.CredentialSecretName)
	updated.StringData[cpapi.CredentialSecretSecretKey] = "updated"
	state, err := buildForCredentialSecret(factory, context.Background(), admissionv1.Update, updated)
	if err != nil {
		t.Fatalf("BuildForCredentialSecret(update) error = %v", err)
	}
	if state.CredentialSecrets[0].StringData.Secret != "updated" {
		t.Fatalf("BuildForCredentialSecret(update) secret = %#v", state.CredentialSecrets[0])
	}

	state, err = buildForCredentialSecret(factory, context.Background(), admissionv1.Delete, updated)
	if err != nil {
		t.Fatalf("BuildForCredentialSecret(delete) error = %v", err)
	}
	if len(state.CredentialSecrets) != 0 {
		t.Fatalf("BuildForCredentialSecret(delete) secrets = %#v, want empty", state.CredentialSecrets)
	}
}

func TestBuildForNodeGroupCreateKeepsAdmittedCloudPermanentOnly(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	worker := testNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent)
	state, err := buildForNodeGroup(factory, context.Background(), admissionv1.Create, worker)
	if err != nil {
		t.Fatalf("BuildForNodeGroup(create) error = %v", err)
	}
	if len(state.NodeGroups) != 1 || state.NodeGroups[0].Name != "worker" {
		t.Fatalf("BuildForNodeGroup(create) node groups = %#v, want admitted worker only", state.NodeGroups)
	}
	if len(state.InstanceClasses) != 0 {
		t.Fatalf("BuildForNodeGroup(create) instance classes = %#v, want none", state.InstanceClasses)
	}
}

func TestBuildForNodeGroupUpdateSkipsNonCloudPermanent(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	static := testNodeGroupObject("master", "CloudStatic")
	state, err := buildForNodeGroup(factory, context.Background(), admissionv1.Update, static)
	if err != nil {
		t.Fatalf("BuildForNodeGroup(update static) error = %v", err)
	}
	if len(state.NodeGroups) != 0 {
		t.Fatalf("BuildForNodeGroup(update static) node groups = %#v, want none", state.NodeGroups)
	}
}

func TestBuildForNodeGroupDeleteLeavesStateEmpty(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t,
		testNodeGroupWithClassRef(cfg, "worker", cpapi.NodeTypeCloudPermanent, "worker-dvp"),
		testInstanceClassObject(cfg, "worker-dvp"),
	), cfg)

	state, err := buildForNodeGroup(factory, context.Background(), admissionv1.Delete, testNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent))
	if err != nil {
		t.Fatalf("BuildForNodeGroup(delete) error = %v", err)
	}
	if len(state.NodeGroups) != 0 {
		t.Fatalf("BuildForNodeGroup(delete) node groups = %#v, want none", state.NodeGroups)
	}
	if len(state.InstanceClasses) != 0 {
		t.Fatalf("BuildForNodeGroup(delete) instance classes = %#v, want none", state.InstanceClasses)
	}
}

func TestBuildForInstanceClassOperations(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	created := testInstanceClassObject(cfg, "worker-dvp")
	state, deleted, err := buildForInstanceClass(factory, context.Background(), admissionv1.Create, created)
	if err != nil {
		t.Fatalf("BuildForInstanceClass(create) error = %v", err)
	}
	if deleted != nil || len(state.InstanceClasses) != 1 || state.InstanceClasses[0].GetName() != "worker-dvp" {
		t.Fatalf("BuildForInstanceClass(create) classes = %#v, deleted = %#v", state.InstanceClasses, deleted)
	}
	if len(state.NodeGroups) != 0 {
		t.Fatalf("BuildForInstanceClass(create) node groups = %#v, want none", state.NodeGroups)
	}

	toDelete := testInstanceClassObject(cfg, "worker-dvp")
	state, deleted, err = buildForInstanceClass(factory, context.Background(), admissionv1.Delete, toDelete)
	if err != nil {
		t.Fatalf("BuildForInstanceClass(delete) error = %v", err)
	}
	if deleted == nil || deleted.GetName() != "worker-dvp" || len(state.InstanceClasses) != 0 {
		t.Fatalf("BuildForInstanceClass(delete) classes = %#v, deleted = %#v", state.InstanceClasses, deleted)
	}
}

func TestBuildForInstanceClassDeleteLoadsReferencingNodeGroups(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t,
		testNodeGroupWithClassRef(cfg, "master", cpapi.NodeTypeCloudPermanent, "master-dvp"),
	), cfg)

	state, deleted, err := buildForInstanceClass(factory, context.Background(), admissionv1.Delete, testInstanceClassObject(cfg, "master-dvp"))
	if err != nil {
		t.Fatalf("BuildForInstanceClass(delete) error = %v", err)
	}
	if deleted == nil || deleted.GetName() != "master-dvp" {
		t.Fatalf("BuildForInstanceClass(delete) deleted = %#v", deleted)
	}
	if len(state.NodeGroups) != 1 || state.NodeGroups[0].Name != "master" {
		t.Fatalf("BuildForInstanceClass(delete) node groups = %#v, want master consumer", state.NodeGroups)
	}
}

func TestBuildForNodeGroupInvalidObject(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	broken := testNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)
	broken.Object["spec"] = "invalid"
	_, err := buildForNodeGroup(factory, context.Background(), admissionv1.Update, broken)
	if err == nil || !strings.Contains(err.Error(), "decode NodeGroup") {
		t.Fatalf("BuildForNodeGroup() error = %v, want decode error", err)
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

func TestBuildForInstanceClassUpdateUsesAdmissionObject(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t,
		testNodeGroupWithClassRef(cfg, "master", cpapi.NodeTypeCloudPermanent, "master-dvp"),
	), cfg)

	updated := testInstanceClassObject(cfg, "master-dvp")
	updated.Object["spec"] = map[string]any{"rootDiskSize": int64(50)}
	state, deleted, err := buildForInstanceClass(factory, context.Background(), admissionv1.Update, updated)
	if err != nil {
		t.Fatalf("BuildForInstanceClass(update) error = %v", err)
	}
	if deleted != nil || len(state.InstanceClasses) != 1 {
		t.Fatalf("BuildForInstanceClass(update) classes = %#v, deleted = %#v", state.InstanceClasses, deleted)
	}
	if len(state.NodeGroups) != 1 || state.NodeGroups[0].Name != "master" {
		t.Fatalf("BuildForInstanceClass(update) node groups = %#v, want master consumer", state.NodeGroups)
	}
}

func TestBuildForNodeGroupUpdateLoadsReferencedInstanceClass(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t,
		testInstanceClassObject(cfg, "master-dvp"),
	), cfg)

	state, err := buildForNodeGroup(factory,
		context.Background(),
		admissionv1.Update,
		testNodeGroupWithClassRef(cfg, "master", cpapi.NodeTypeCloudPermanent, "master-dvp"),
	)
	if err != nil {
		t.Fatalf("BuildForNodeGroup(update) error = %v", err)
	}
	if len(state.NodeGroups) != 1 {
		t.Fatalf("BuildForNodeGroup(update) node groups = %#v, want 1", state.NodeGroups)
	}
	if len(state.InstanceClasses) != 1 || state.InstanceClasses[0].GetName() != "master-dvp" {
		t.Fatalf("BuildForNodeGroup(update) instance classes = %#v", state.InstanceClasses)
	}
}

func TestBuildForCredentialSecretUsesAdmissionObjectOnUpdate(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	updated := testCredentialSecretObject(cfg, cpapi.CredentialSecretName)
	updated.StringData[cpapi.CredentialSecretSecretKey] = "rotated"
	state, err := buildForCredentialSecret(factory, context.Background(), admissionv1.Update, updated)
	if err != nil {
		t.Fatalf("BuildForCredentialSecret(update existing) error = %v", err)
	}
	if state.CredentialSecrets[0].StringData.Secret != "rotated" {
		t.Fatalf("BuildForCredentialSecret(update existing) = %#v", state.CredentialSecrets)
	}
}

func TestBuildReturnsErrorWhenMigrationConfigMapUnreadable(t *testing.T) {
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

	factory := newTestFactory(cli, cfg)
	if _, err := factory.CreateBuilder().Build(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "get migration ConfigMap") {
		t.Fatalf("Build() error = %v, want migration ConfigMap read error", err)
	}
}

func TestBuildForInstanceClassInvalidObject(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	broken := testInstanceClassObject(cfg, "broken-dvp")
	broken.Object["spec"] = "invalid"
	_, _, err := buildForInstanceClass(factory, context.Background(), admissionv1.Create, broken)
	if err == nil || !strings.Contains(err.Error(), "decode instance class") {
		t.Fatalf("buildForInstanceClass() error = %v, want decode error", err)
	}
}

func TestBuildForInstanceClassDecodeErrorOnCreate(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	broken := testInstanceClassObject(cfg, "broken-dvp")
	broken.Object["spec"] = "invalid"
	if _, _, err := buildForInstanceClass(factory, context.Background(), admissionv1.Create, broken); err == nil {
		t.Fatal("BuildForInstanceClass(create) error = nil, want decode error")
	}
}

func TestBuildForInstanceClassDecodeErrorOnDelete(t *testing.T) {
	t.Parallel()

	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)

	broken := testInstanceClassObject(cfg, "broken-dvp")
	broken.Object["spec"] = "invalid"
	if _, _, err := buildForInstanceClass(factory, context.Background(), admissionv1.Delete, broken); err == nil ||
		!strings.Contains(err.Error(), "decode instance class") {
		t.Fatalf("buildForInstanceClass(delete) error = %v, want decode error", err)
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

func testNodeGroupWithClassRef(cfg StateBuilderConfig, name string, nodeType cpapi.NodeType, className string) *unstructured.Unstructured {
	obj := testNodeGroupObject(name, nodeType)
	obj.Object["spec"] = map[string]any{
		"nodeType": string(nodeType),
		"cloudInstances": map[string]any{
			"classReference": map[string]any{
				"kind": cfg.InstanceClassGVK.Kind,
				"name": className,
			},
		},
	}
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
	obj.SetGroupVersionKind(cfg.InstanceClassGVK)
	obj.SetName(name)
	return obj
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

func TestBuildForNodeGroupLeavesProviderClusterConfigAbsent(t *testing.T) {
	t.Parallel()
	cfg := testStateBuilderConfig()
	factory := newTestFactory(newRuntimeBuilderTestClient(t), cfg)
	state, err := buildForNodeGroup(factory, context.Background(), admissionv1.Create, testNodeGroupWithClassRef(cfg, "master", cpapi.NodeTypeCloudPermanent, "master-test"))
	if err != nil {
		t.Fatalf("BuildForNodeGroup() error = %v", err)
	}
	if state.HasProviderClusterConfig() {
		t.Fatal("HasProviderClusterConfig() = true, want false for admission state")
	}
}
