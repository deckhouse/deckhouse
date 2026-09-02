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

package override

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func newTestReconciler(t *testing.T, objects ...client.Object) *reconciler {
	t.Helper()

	sc, err := project.Scheme()
	require.NoError(t, err)

	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithStatusSubresource(&v1alpha2.Module{}, &v1alpha2.ModulePullOverride{}).
		WithObjects(objects...).
		Build()

	r := &reconciler{
		init:                new(sync.WaitGroup),
		client:              cl,
		log:                 log.NewNop(),
		dependencyContainer: dependency.NewMockedContainer(),
	}

	return r
}

func testOverride(module, tag string) *v1alpha2.ModulePullOverride {
	return &v1alpha2.ModulePullOverride{
		ObjectMeta: metav1.ObjectMeta{Name: module},
		Spec:       v1alpha2.ModulePullOverrideSpec{ImageTag: tag},
	}
}

func testModule(name, repository, version string) *v1alpha2.Module {
	return &v1alpha2.Module{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: repository, PackageVersion: version},
	}
}

func enabledByManager(module *v1alpha2.Module) *v1alpha2.Module {
	module.Status.Conditions = []metav1.Condition{{
		Type:               v1alpha1.ModuleConditionEnabledByModuleManager,
		Status:             metav1.ConditionTrue,
		Reason:             v1alpha1.ModuleReasonEnabled,
		LastTransitionTime: metav1.Now(),
	}}

	return module
}

func testConfig(module string, enabled *bool) *v1alpha1.ModuleConfig {
	return &v1alpha1.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{Name: module},
		Spec:       v1alpha1.ModuleConfigSpec{Enabled: enabled},
	}
}

func getModule(t *testing.T, r *reconciler, name string) *v1alpha2.Module {
	t.Helper()

	module := new(v1alpha2.Module)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: name}, module))

	return module
}

func TestModuleEnabled(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		objects []client.Object
		module  *v1alpha2.Module
		want    bool
	}{
		{name: "config enables", objects: []client.Object{testConfig("echo", ptr.To(true))}, want: true},
		{name: "config disables a running module", objects: []client.Object{testConfig("echo", ptr.To(false))}, module: enabledByManager(testModule("echo", "external", "v1.0.0")), want: false},
		{name: "config without the flag defers to the manager", objects: []client.Object{testConfig("echo", nil)}, module: enabledByManager(testModule("echo", "external", "v1.0.0")), want: true},
		{name: "no config: the manager decides", module: enabledByManager(testModule("echo", "external", "v1.0.0")), want: true},
		{name: "no config: a module the manager left off", module: testModule("echo", "external", "v1.0.0"), want: false},
		{name: "no config and no module", want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newTestReconciler(t, c.objects...)

			got, err := r.moduleEnabled(ctx, "echo", c.module)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestEnsureDevModule(t *testing.T) {
	ctx := context.Background()

	t.Run("module without an object is created on the tag", func(t *testing.T) {
		r := newTestReconciler(t)

		require.NoError(t, r.ensureDevModule(ctx, testOverride("echo", "main"), "external"))

		module := getModule(t, r, "echo")
		assert.True(t, module.IsDev())
		assert.Equal(t, "external", module.Spec.PackageRepositoryName)
		assert.Equal(t, "main", module.Spec.PackageVersion)
		assert.True(t, module.IsCondition(v1alpha1.ModuleConditionIsOverridden, metav1.ConditionTrue))
	})

	t.Run("released module moves onto the tag", func(t *testing.T) {
		module := testModule("echo", "deckhouse-modules", "v1.2.3")
		module.Spec.Enabled = ptr.To(true)
		r := newTestReconciler(t, module)

		require.NoError(t, r.ensureDevModule(ctx, testOverride("echo", "pr42"), "deckhouse-modules"))

		module = getModule(t, r, "echo")
		assert.True(t, module.IsDev())
		assert.Equal(t, "pr42", module.Spec.PackageVersion)
		assert.Equal(t, ptr.To(true), module.Spec.Enabled, "the config fields stay")
		assert.True(t, module.IsCondition(v1alpha1.ModuleConditionIsOverridden, metav1.ConditionTrue))
	})
}

func TestDeleteModuleOverride(t *testing.T) {
	ctx := context.Background()

	module := testModule("echo", "external", "main")
	module.Annotations = map[string]string{v1alpha2.ModuleAnnotationDev: "true"}
	mpo := testOverride("echo", "main")
	mpo.Finalizers = []string{v1alpha1.ModulePullOverrideFinalizer}
	mpo.DeletionTimestamp = ptr.To(metav1.Now())

	r := newTestReconciler(t, module, mpo)

	_, err := r.deleteModuleOverride(ctx, mpo)
	require.NoError(t, err)

	module = getModule(t, r, "echo")
	assert.False(t, module.IsDev(), "the dev mark dies with the override")
	assert.True(t, module.IsCondition(v1alpha1.ModuleConditionIsOverridden, metav1.ConditionFalse))
	assert.Equal(t, "main", module.Spec.PackageVersion, "the version is left for the sync to place")
}

func TestHandleModuleOverrideGates(t *testing.T) {
	ctx := context.Background()

	t.Run("embedded module is reported", func(t *testing.T) {
		module := testModule("echo", "embedded", "v1.80.0")
		module.Annotations = map[string]string{v1alpha2.ModuleAnnotationEmbedded: "true"}
		mpo := testOverride("echo", "main")
		r := newTestReconciler(t, module, mpo)

		res, err := r.handleModuleOverride(ctx, mpo)
		require.NoError(t, err)
		assert.Equal(t, defaultRequeueAfter, res.RequeueAfter)
		assert.Equal(t, v1alpha1.ModulePullOverrideMessageModuleEmbedded, mpo.Status.Message)
	})

	t.Run("module without a config and an object is off", func(t *testing.T) {
		mpo := testOverride("echo", "main")
		r := newTestReconciler(t, mpo)

		res, err := r.handleModuleOverride(ctx, mpo)
		require.NoError(t, err)
		assert.Equal(t, defaultRequeueAfter, res.RequeueAfter)
		assert.Equal(t, v1alpha1.ModulePullOverrideMessageModuleDisabled, mpo.Status.Message)
	})

	t.Run("enabled module nobody offers has no source", func(t *testing.T) {
		mpo := testOverride("echo", "main")
		r := newTestReconciler(t, mpo, testConfig("echo", ptr.To(true)))

		res, err := r.handleModuleOverride(ctx, mpo)
		require.NoError(t, err)
		assert.Equal(t, defaultRequeueAfter, res.RequeueAfter)
		assert.Equal(t, v1alpha1.ModulePullOverrideMessageNoSource, mpo.Status.Message)
	})
}
