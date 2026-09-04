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

package module_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	packageruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	packagestatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/module"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

const (
	moduleName = "test-module"

	versionName     = "deckhouse-test-module-v1.0.1"
	nextVersionName = "deckhouse-test-module-v1.0.2"

	// devDigest is what the stub resolves the mutable dev tag to, staleDigest the one a
	// repushed dev module was last handed over on.
	devDigest   = "sha256:c0ffee"
	staleDigest = "sha256:stale"
)

// testRemote is the registry.Remote every fixture's PackageRepository builds into.
var testRemote = registry.Remote{
	Name:         "deckhouse",
	Repository:   "registry.example.com/test",
	DockerConfig: "test-docker-cfg",
	CA:           "test-ca",
	Scheme:       "https",
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	reconcilertest.Suite

	ctr     reconcile.Reconciler
	manager *packageManagerStub
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{
			&v1alpha2.Module{},
			&v1alpha1.ModulePackage{},
			&v1alpha1.ModulePackageVersion{},
		},
		// The used flag is what the reconciler releases, so a fixture that starts out
		// used has to survive seeding — otherwise every detach assertion passes vacuously.
		SeedStatusSubresources: []client.Object{&v1alpha1.ModulePackageVersion{}},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha2.SchemeGroupVersion.WithKind("Module"),
			v1alpha1.SchemeGroupVersion.WithKind("ModulePackage"),
			v1alpha1.SchemeGroupVersion.WithKind("ModulePackageVersion"),
			v1alpha1.SchemeGroupVersion.WithKind("PackageRepository"),
		},
		GoldenMode:    reconcilertest.PerDocument,
		SeedViaCreate: true,
	})
}

func (suite *ControllerTestSuite) setupController(filename string) {
	suite.Seed(filename)

	suite.manager = newPackageManagerStub(suite.T())
	suite.ctr = reconcilerFor(suite.T(), suite.Client(), suite.manager)
}

// reconcilerFor registers the controller behind an init gate that is already open, which
// is the state a running controller reconciles in.
func reconcilerFor(t *testing.T, cl client.Client, manager *packageManagerStub) reconcile.Reconciler {
	t.Helper()

	return registerController(t, cl, manager, new(sync.WaitGroup))
}

// registerController runs the package's only exported entry point against a manager stub
// and picks up the reconciler it registered.
func registerController(
	t *testing.T,
	cl client.Client,
	manager *packageManagerStub,
	init *sync.WaitGroup,
) reconcile.Reconciler {
	t.Helper()

	mgr := &managerStub{scheme: testScheme(t), client: cl}
	require.NoError(t, module.RegisterController(init, mgr, manager, log.NewNop()))

	require.Len(t, mgr.runnables, 1, "the controller must be the only runnable registered")

	reconciler, ok := mgr.runnables[0].(reconcile.Reconciler)
	require.True(t, ok, "the registered runnable must be the controller")

	return reconciler
}

// managerStub is the part of ctrlmanager.Manager that RegisterController touches. The
// embedded interface leaves everything else nil, so an unexpected new dependency on the
// manager fails loudly instead of being silently satisfied.
type managerStub struct {
	ctrlmanager.Manager

	scheme    *runtime.Scheme
	client    client.Client
	runnables []ctrlmanager.Runnable
}

func (m *managerStub) GetClient() client.Client   { return m.client }
func (m *managerStub) GetScheme() *runtime.Scheme { return m.scheme }
func (m *managerStub) GetLogger() logr.Logger     { return logr.Discard() }
func (m *managerStub) GetCache() cache.Cache      { return nil }

// GetControllerOptions skips the name check: every test registers the same controller name.
func (m *managerStub) GetControllerOptions() ctrlconfig.Controller {
	return ctrlconfig.Controller{SkipNameValidation: ptr.To(true)}
}

func (m *managerStub) Add(runnable ctrlmanager.Runnable) error {
	m.runnables = append(m.runnables, runnable)

	return nil
}

func (suite *ControllerTestSuite) TestReconcile() {
	ctx := context.Background()

	suite.Run("resource not found", func() {
		suite.setupController("resource-not-found.yaml")

		result, err := suite.ctr.Reconcile(ctx, request("non-existent-module"))
		require.NoError(suite.T(), err)
		assert.True(suite.T(), result.IsZero())

		assert.Empty(suite.T(), suite.manager.updated)
	})

	suite.Run("successful reconcile hands the module to the runtime", func() {
		suite.setupController("successful-reconcile.yaml")

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.True(suite.T(), result.IsZero(), "a settled module must not be requeued")

		require.Len(suite.T(), suite.manager.updated, 1)
		assert.Equal(suite.T(), testRemote, suite.manager.updated[0].repo)
		assert.False(suite.T(), suite.manager.updated[0].forced,
			"a released module changes version through its spec, so nothing has to be forced")
		assert.Equal(suite.T(), packageruntime.Module{
			Name:            moduleName,
			Definition:      modules.Definition{Name: moduleName, Version: "v1.0.1"},
			Settings:        addonutils.Values{"replicas": float64(2), "ingress": map[string]any{"host": "module.example.com"}},
			SettingsVersion: 2,
			Maintenance:     "NoResourceReconciliation",
			Enabled:         ptr.To(true),
		}, suite.manager.updated[0].module)

		assert.True(suite.T(), suite.getVersion(versionName).Status.Used)

		mod := suite.getModule(moduleName)
		assert.Contains(suite.T(), mod.Finalizers, v1alpha2.ModuleFinalizerStatisticRegistered)
		assert.Equal(suite.T(), versionName, ownerRefName(mod, v1alpha1.ModulePackageVersionKind))
		assert.Equal(suite.T(), moduleName, ownerRefName(mod, v1alpha1.ModulePackageKind))
	})

	suite.Run("missing package requeues and claims the finalizer", func() {
		suite.setupController("package-not-found.yaml")

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 30*time.Second, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated, "an unresolved module must not reach the runtime")
		// The finalizer is claimed before the package lookup, so deletion is guarded
		// even for a module that never resolved.
		assert.Contains(suite.T(), suite.getModule(moduleName).Finalizers,
			v1alpha2.ModuleFinalizerStatisticRegistered)
	})

	suite.Run("missing version requeues", func() {
		suite.setupController("version-not-found.yaml")

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 30*time.Second, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated)
	})

	suite.Run("draft version requeues", func() {
		suite.setupController("version-is-draft.yaml")

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 30*time.Second, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated, "a draft version must not reach the runtime")
		assert.False(suite.T(), suite.getVersion(versionName).Status.Used)
	})

	suite.Run("missing repository leaves the used flag untouched", func() {
		suite.setupController("repository-not-found.yaml")

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 30*time.Second, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated)
		// The repository is read before the relink, so a missing one cannot leave the
		// versions half-updated.
		assert.False(suite.T(), suite.getVersion(versionName).Status.Used)
	})

	suite.Run("version switch releases the previous version", func() {
		suite.setupController("version-switch.yaml")

		_, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)

		assert.False(suite.T(), suite.getVersion(versionName).Status.Used)
		assert.True(suite.T(), suite.getVersion(nextVersionName).Status.Used)

		assert.Equal(suite.T(), nextVersionName,
			ownerRefName(suite.getModule(moduleName), v1alpha1.ModulePackageVersionKind))
	})

	suite.Run("stale owner references are replaced and foreign ones kept", func() {
		suite.setupController("stale-owner-references.yaml")

		_, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)

		mod := suite.getModule(moduleName)
		assert.Equal(suite.T(), versionName, ownerRefName(mod, v1alpha1.ModulePackageVersionKind))
		assert.Equal(suite.T(), moduleName, ownerRefName(mod, v1alpha1.ModulePackageKind))
		assert.Equal(suite.T(), "unrelated", ownerRefName(mod, "ConfigMap"),
			"references of other kinds must survive the relink")
	})

	suite.Run("repeated reconcile settles on the same references", func() {
		suite.setupController("repeated-reconcile.yaml")

		for range 2 {
			_, err := suite.ctr.Reconcile(ctx, request(moduleName))
			require.NoError(suite.T(), err)
		}

		assert.Len(suite.T(), suite.getModule(moduleName).OwnerReferences, 2)
		assert.True(suite.T(), suite.getVersion(versionName).Status.Used)
	})

	suite.Run("registry spec changed annotation is cleared", func() {
		suite.setupController("registry-spec-changed.yaml")

		_, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)

		annotations := suite.getModule(moduleName).Annotations
		assert.NotContains(suite.T(), annotations, v1alpha2.ModuleAnnotationRegistrySpecChanged)
		assert.Contains(suite.T(), annotations, "packages.deckhouse.io/keep-me")
	})

	suite.Run("embedded module reaches the runtime without a repository", func() {
		suite.setupController("embedded.yaml")

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.True(suite.T(), result.IsZero())

		// The released path would resolve this fixture too, so reaching the runtime as an
		// embedded module — with no Definition, which is what the image already ships — is
		// what tells the two apart.
		assert.Empty(suite.T(), suite.manager.updated,
			"the image ships an embedded module, so nothing is pulled for it")
		assert.Equal(suite.T(), []packageruntime.Module{{
			Name:            moduleName,
			Settings:        addonutils.Values{"replicas": float64(2)},
			SettingsVersion: 1,
			Maintenance:     "NoResourceReconciliation",
			Enabled:         ptr.To(true),
		}}, suite.manager.embedded)

		// A module the image started shipping after it had already been released still
		// points at the downloaded version, which the relink has to drop.
		assert.False(suite.T(), suite.getVersion("deckhouse-test-module-v0.9.0").Status.Used)
		assert.Equal(suite.T(), "embedded-test-module-v1.0.1",
			ownerRefName(suite.getModule(moduleName), v1alpha1.ModulePackageVersionKind))
	})

	suite.Run("embedded wins over dev", func() {
		suite.setupController("embedded-and-dev.yaml")

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.True(suite.T(), result.IsZero(), "an embedded module follows no mutable tag to re-resolve")

		assert.Len(suite.T(), suite.manager.embedded, 1)
		assert.Empty(suite.T(), suite.manager.digestCalls,
			"the embedded annotation is read first, the way the bootstrap and the runtime do")
	})

	suite.Run("dev module is forced onto the resolved digest", func() {
		suite.setupController("dev.yaml")

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 15*time.Second, result.RequeueAfter,
			"a repush under the same tag moves nothing in the API server")

		require.Len(suite.T(), suite.manager.updated, 1)
		assert.Equal(suite.T(), testRemote, suite.manager.updated[0].repo)
		assert.True(suite.T(), suite.manager.updated[0].forced,
			"change detection sees the same tag either way, so only force carries a repush through")
		assert.Equal(suite.T(), packageruntime.Module{
			Name:            moduleName,
			Definition:      modules.Definition{Name: moduleName, Version: "main"},
			Settings:        addonutils.Values{"replicas": float64(2)},
			SettingsVersion: 1,
			Maintenance:     "NoResourceReconciliation",
			Enabled:         ptr.To(true),
		}, suite.manager.updated[0].module)
		assert.Equal(suite.T(), []digestCall{{repo: testRemote, name: moduleName, tag: "main"}},
			suite.manager.digestCalls,
			"the tag is what decides which image the digest is resolved from")

		annotations := suite.getModule(moduleName).Annotations
		assert.Equal(suite.T(), devDigest, annotations[v1alpha2.ModuleAnnotationHash],
			"the digest is recorded only after the handover, so a failure re-forces rather than skips")
		assert.NotContains(suite.T(), annotations, v1alpha2.ModuleAnnotationRegistrySpecChanged)
	})

	suite.Run("dev module on an untouched digest is not forced", func() {
		suite.setupController("dev-unchanged-digest.yaml")

		_, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)

		require.Len(suite.T(), suite.manager.updated, 1)
		assert.False(suite.T(), suite.manager.updated[0].forced)
	})

	suite.Run("dev module releases the version it was released on", func() {
		suite.setupController("dev-released-version.yaml")

		_, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)

		// No version backs a mutable tag, so a reference left over from the module's
		// released past would block its owner's deletion for ever.
		assert.False(suite.T(), suite.getVersion(versionName).Status.Used)

		mod := suite.getModule(moduleName)
		assert.Empty(suite.T(), ownerRefName(mod, v1alpha1.ModulePackageVersionKind))
		assert.Empty(suite.T(), ownerRefName(mod, v1alpha1.ModulePackageKind))
	})

	suite.Run("dev module with no repository name requeues", func() {
		suite.setupController("dev-without-repository.yaml")

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 30*time.Second, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated)
		assert.Empty(suite.T(), suite.manager.digestCalls)
	})

	suite.Run("deleted module is torn down, detached and finished", func() {
		suite.setupController("delete.yaml")

		require.NoError(suite.T(), suite.Client().Delete(ctx, suite.getModule(moduleName)))

		result, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)
		assert.True(suite.T(), result.IsZero())

		assert.Equal(suite.T(), []removedModule{{name: moduleName}}, suite.manager.removed)
		assert.False(suite.T(), suite.getVersion(versionName).Status.Used)

		err = suite.Client().Get(ctx, client.ObjectKey{Name: moduleName}, new(v1alpha2.Module))
		assert.True(suite.T(), apierrors.IsNotFound(err), "the finalizer must be released")
	})

	suite.Run("deleted embedded module undeploys nothing", func() {
		suite.setupController("delete-embedded.yaml")

		require.NoError(suite.T(), suite.Client().Delete(ctx, suite.getModule(moduleName)))

		_, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)

		assert.Equal(suite.T(), []removedModule{{name: moduleName, embedded: true}}, suite.manager.removed)
		assert.False(suite.T(), suite.getVersion("embedded-test-module-v1.0.1").Status.Used)
	})

	suite.Run("deleted module is detached from the version it was attached to", func() {
		suite.setupController("delete-after-version-edit.yaml")

		require.NoError(suite.T(), suite.Client().Delete(ctx, suite.getModule(moduleName)))

		_, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err)

		// The spec was bumped to v1.0.2 before the deletion was handled, so detaching by
		// spec would leave the version the module actually ran from marked as used.
		assert.False(suite.T(), suite.getVersion(versionName).Status.Used)
	})

	suite.Run("deleted module whose version is already gone is still finished", func() {
		suite.setupController("delete-missing-version.yaml")

		require.NoError(suite.T(), suite.Client().Delete(ctx, suite.getModule(moduleName)))

		_, err := suite.ctr.Reconcile(ctx, request(moduleName))
		require.NoError(suite.T(), err, "a version that is gone needs no cleanup")

		err = suite.Client().Get(ctx, client.ObjectKey{Name: moduleName}, new(v1alpha2.Module))
		assert.True(suite.T(), apierrors.IsNotFound(err), "the finalizer must be released")
	})
}

func (suite *ControllerTestSuite) getModule(name string) *v1alpha2.Module {
	mod := new(v1alpha2.Module)
	require.NoError(suite.T(), suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, mod))

	return mod
}

func (suite *ControllerTestSuite) getVersion(name string) *v1alpha1.ModulePackageVersion {
	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(suite.T(), suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, mpv))

	return mpv
}

func TestReconcileRequeuesOnGetError(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return errors.New("api server is unreachable")
			},
		}).
		Build()

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	// A read that fails is not a module that is gone, so the pass is retried rather
	// than dropped.
	result, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err)
	assert.Equal(t, 1*time.Second, result.RequeueAfter)
	assert.Empty(t, manager.updated)
}

func TestRelinkFailureKeepsTheModuleOutOfTheRuntime(t *testing.T) {
	cl := seedFakeClient(t, "successful-reconcile.yaml", interceptor.Funcs{
		SubResourcePatch: func(context.Context, client.Client, string, client.Object, client.Patch,
			...client.SubResourcePatchOption) error {
			return errors.New("status patch rejected")
		},
	})

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	result, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)

	// The used flag is what keeps the version from being deleted under the module, so a
	// module that failed to claim it must not be handed over.
	assert.Empty(t, manager.updated)
}

func TestRelinkDoesNotClaimTheNewVersionWhenTheOldOneIsStuck(t *testing.T) {
	cl := seedFakeClient(t, "version-switch.yaml", interceptor.Funcs{
		SubResourcePatch: failStatusPatchOf(versionName),
	})

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	result, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)

	// Release before claim: a bump that could not release the old version must not mark
	// the new one, or the module ends up holding both for ever.
	assert.True(t, versionUsed(t, cl, versionName))
	assert.False(t, versionUsed(t, cl, nextVersionName))
	assert.Empty(t, manager.updated)
}

func TestCommitFailureLeavesTheRuntimeAheadOfTheAPI(t *testing.T) {
	cl := seedFakeClient(t, "version-switch.yaml", interceptor.Funcs{Patch: failModulePatch(1)})

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	result, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)

	// The used flags move before the owner references, so a rejected commit leaves the
	// module referencing a version it has already released — the requeue is what closes it.
	assert.Len(t, manager.updated, 1)
	assert.False(t, versionUsed(t, cl, versionName))
	assert.True(t, versionUsed(t, cl, nextVersionName))

	mod := new(v1alpha2.Module)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	assert.Equal(t, versionName, ownerRefName(mod, v1alpha1.ModulePackageVersionKind))
}

func TestDevHashIsNotRecordedWhenThePatchFails(t *testing.T) {
	cl := seedFakeClient(t, "dev-released-version.yaml", interceptor.Funcs{Patch: failModulePatch(1)})

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	result, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)

	require.Len(t, manager.updated, 1)
	assert.True(t, manager.updated[0].forced)

	// The digest is written in the same patch that was rejected, so the next pass still
	// sees the old one and forces again instead of skipping the repush.
	mod := new(v1alpha2.Module)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	assert.NotContains(t, mod.Annotations, v1alpha2.ModuleAnnotationHash)
}

func TestReconcileWaitsForInit(t *testing.T) {
	cl := seedFakeClient(t, "successful-reconcile.yaml", interceptor.Funcs{})

	init := new(sync.WaitGroup)
	init.Add(1)

	manager := newPackageManagerStub(t)
	ctr := registerController(t, cl, manager, init)

	done := make(chan struct{})
	go func() {
		defer close(done)

		_, err := ctr.Reconcile(context.Background(), request(moduleName))
		assert.NoError(t, err)
	}()

	// Reconciling before the runtime has loaded its packages would hand the module over
	// to a manager that cannot place it yet.
	select {
	case <-done:
		require.Fail(t, "the reconcile must block until init is done")
	case <-time.After(50 * time.Millisecond):
	}

	init.Done()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "the reconcile must return once init is done")
	}

	assert.Len(t, manager.updated, 1)
}

func TestFinalizerFailureKeepsTheModuleOutOfTheRuntime(t *testing.T) {
	cl := seedFakeClient(t, "successful-reconcile.yaml", interceptor.Funcs{
		Patch: func(context.Context, client.WithWatch, client.Object, client.Patch,
			...client.PatchOption) error {
			return errors.New("patch rejected")
		},
	})

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	result, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)

	// The finalizer is what guarantees the deletion path gets to tear the module down, so
	// nothing may reach the runtime before it is on disk.
	assert.Empty(t, manager.updated)

	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: versionName}, mpv))
	assert.False(t, mpv.Status.Used)
}

func TestDeleteIsFinishedWhileAForeignFinalizerHolds(t *testing.T) {
	cl := seedFakeClient(t, "delete-foreign-finalizer.yaml", interceptor.Funcs{})

	mod := new(v1alpha2.Module)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	require.NoError(t, cl.Delete(context.Background(), mod))

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	_, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err)

	// The teardown and the detach both run before the finalizer is looked at, so a module
	// someone else's finalizer keeps alive is still released by the runtime.
	assert.Equal(t, []removedModule{{name: moduleName}}, manager.removed)

	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: versionName}, mpv))
	assert.False(t, mpv.Status.Used)

	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	assert.Equal(t, []string{"test.deckhouse.io/keep"}, mod.Finalizers)
}

func TestDeleteWaitsForRuntimeTeardown(t *testing.T) {
	cl := seedFakeClient(t, "delete.yaml", interceptor.Funcs{})

	mod := new(v1alpha2.Module)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	require.NoError(t, cl.Delete(context.Background(), mod))

	manager := newPackageManagerStub(t)
	manager.removalDone = false
	ctr := reconcilerFor(t, cl, manager)

	result, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, result.RequeueAfter)
	assert.Equal(t, []removedModule{{name: moduleName}}, manager.removed)

	// Releasing the version before the teardown finishes lets it be garbage collected —
	// and its package files removed from disk — while the uninstall still needs them.
	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: versionName}, mpv))
	assert.True(t, mpv.Status.Used)

	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	assert.Contains(t, mod.Finalizers, v1alpha2.ModuleFinalizerStatisticRegistered)
}

func TestDeleteFailureKeepsTheFinalizerUntilTheRetry(t *testing.T) {
	cl := seedFakeClient(t, "delete.yaml", interceptor.Funcs{Patch: failModulePatch(1)})

	mod := new(v1alpha2.Module)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	require.NoError(t, cl.Delete(context.Background(), mod))

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	_, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.Error(t, err)

	// The teardown and the detach are already done; dropping the finalizer is all that is
	// left, so the module has to stay in Terminating until that write lands.
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	assert.Contains(t, mod.Finalizers, v1alpha2.ModuleFinalizerStatisticRegistered)
	assert.False(t, versionUsed(t, cl, versionName))

	_, err = ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err, "the retry must not trip over the version it has already released")

	err = cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestDeleteFailsOnUnreadableVersion(t *testing.T) {
	getErr := apierrors.NewInternalError(errors.New("etcd is unavailable"))

	cl := seedFakeClient(t, "delete.yaml", interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object,
			opts ...client.GetOption) error {
			if _, ok := obj.(*v1alpha1.ModulePackageVersion); ok {
				return getErr
			}

			return cl.Get(ctx, key, obj, opts...)
		},
	})

	mod := new(v1alpha2.Module)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	require.NoError(t, cl.Delete(context.Background(), mod))

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	// A version that is gone needs no cleanup, but a version that cannot be read is not
	// gone: treating the two alike would leak the used flag.
	_, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.ErrorIs(t, err, getErr)
	assert.Equal(t, []removedModule{{name: moduleName}}, manager.removed)

	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	assert.Contains(t, mod.Finalizers, v1alpha2.ModuleFinalizerStatisticRegistered)
}

func TestDevModuleIsNotHandedOverOnAnUnresolvedDigest(t *testing.T) {
	cl := seedFakeClient(t, "dev.yaml", interceptor.Funcs{})

	manager := newPackageManagerStub(t)
	manager.digestErr = errors.New("manifest unknown")
	ctr := reconcilerFor(t, cl, manager)

	result, err := ctr.Reconcile(context.Background(), request(moduleName))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)

	assert.Empty(t, manager.updated)

	// The digest is the dev module's only change signal, so recording one that was never
	// handed over would skip the next repush instead of forcing it.
	mod := new(v1alpha2.Module)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: moduleName}, mod))
	assert.Equal(t, staleDigest, mod.Annotations[v1alpha2.ModuleAnnotationHash])
}

// seedFakeClient builds a client from a fixture and wraps it with funcs, for the paths that
// only show up when a write or a read fails. Seeding uses Create plus a status Update for a
// version that starts out used, so funcs passed here must leave those two alone.
func seedFakeClient(t *testing.T, fixture string, funcs interceptor.Funcs) client.Client {
	t.Helper()

	scheme := testScheme(t)

	raw, err := reconcilertest.LoadFixture("./testdata", fixture)
	require.NoError(t, err)

	objs, err := reconcilertest.Decode(scheme, raw)
	require.NoError(t, err)

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1alpha2.Module{}, &v1alpha1.ModulePackage{},
			&v1alpha1.ModulePackageVersion{}).
		WithInterceptorFuncs(funcs).
		Build()

	for _, obj := range objs {
		// Create strips the status of a status subresource, and the used flag a fixture
		// starts out with is what the detach paths act on.
		mpv, used := obj.(*v1alpha1.ModulePackageVersion)
		used = used && mpv.Status.Used

		require.NoError(t, cl.Create(context.TODO(), obj))

		if used {
			mpv.Status.Used = true
			require.NoError(t, cl.Status().Update(context.TODO(), mpv))
		}
	}

	return cl
}

// failModulePatch rejects the first n patches of the Module itself, leaving status patches
// and every other object alone, so a test can pick out one write in the middle of a pass.
func failModulePatch(n int) func(context.Context, client.WithWatch, client.Object, client.Patch,
	...client.PatchOption) error {
	left := n

	return func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch,
		opts ...client.PatchOption) error {
		if _, ok := obj.(*v1alpha2.Module); ok && left > 0 {
			left--

			return errors.New("patch rejected")
		}

		return cl.Patch(ctx, obj, patch, opts...)
	}
}

// failStatusPatchOf rejects the status patch of one named version, so a relink can fail
// halfway.
func failStatusPatchOf(name string) func(context.Context, client.Client, string, client.Object,
	client.Patch, ...client.SubResourcePatchOption) error {
	return func(ctx context.Context, cl client.Client, _ string, obj client.Object,
		patch client.Patch, opts ...client.SubResourcePatchOption) error {
		if obj.GetName() == name {
			return errors.New("status patch rejected")
		}

		return cl.Status().Patch(ctx, obj, patch, opts...)
	}
}

func versionUsed(t *testing.T, cl client.Client, name string) bool {
	t.Helper()

	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: name}, mpv))

	return mpv.Status.Used
}

func request(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme, err := project.Scheme()
	require.NoError(t, err)

	return scheme
}

// ownerRefName is the read-only counterpart of ctrlutils.OwnerRefName, kept local so the
// assertions do not depend on the helper under test.
func ownerRefName(mod *v1alpha2.Module, kind string) string {
	for _, ref := range mod.GetOwnerReferences() {
		if ref.Kind == kind {
			return ref.Name
		}
	}

	return ""
}

// packageManagerStub records the calls the reconciler makes into the package runtime.
// RegisterController requires it to satisfy the package's manager interface, which is the
// compile-time check that this stub still matches the real runtime.
type packageManagerStub struct {
	updated     []updatedModule
	embedded    []packageruntime.Module
	removed     []removedModule
	digestCalls []digestCall

	digestErr   error
	removalDone bool

	queue workqueue.TypedRateLimitingInterface[string]
}

// newPackageManagerStub hands out a real queue: RegisterController starts the status
// service on it, and that goroutine only exits once the queue is shut down.
func newPackageManagerStub(t *testing.T) *packageManagerStub {
	t.Helper()

	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	t.Cleanup(queue.ShutDown)

	return &packageManagerStub{
		queue:       queue,
		removalDone: true,
	}
}

type updatedModule struct {
	repo   registry.Remote
	module packageruntime.Module
	forced bool
}

type removedModule struct {
	name     string
	embedded bool
}

type digestCall struct {
	repo registry.Remote
	name string
	tag  string
}

func (s *packageManagerStub) UpdateModule(repo registry.Remote, mod packageruntime.Module, force bool) {
	s.updated = append(s.updated, updatedModule{repo: repo, module: mod, forced: force})
}

func (s *packageManagerStub) UpdateEmbeddedModule(mod packageruntime.Module) {
	s.embedded = append(s.embedded, mod)
}

func (s *packageManagerStub) UpdateModulesSettings(string, int, addonutils.Values, string, *bool) {}

func (s *packageManagerStub) GetModuleDigest(_ context.Context, repo registry.Remote, name, tag string) (string, error) {
	s.digestCalls = append(s.digestCalls, digestCall{repo: repo, name: name, tag: tag})

	if s.digestErr != nil {
		return "", s.digestErr
	}

	return devDigest, nil
}

func (s *packageManagerStub) RemoveModule(name string) bool {
	s.removed = append(s.removed, removedModule{name: name})

	return s.removalDone
}

func (s *packageManagerStub) RemoveEmbeddedModule(name string) bool {
	s.removed = append(s.removed, removedModule{name: name, embedded: true})

	return s.removalDone
}

func (s *packageManagerStub) GetStatus(string) packagestatus.Status {
	return packagestatus.Status{}
}

func (s *packageManagerStub) GetModuleStatusQueue() workqueue.TypedRateLimitingInterface[string] {
	return s.queue
}
