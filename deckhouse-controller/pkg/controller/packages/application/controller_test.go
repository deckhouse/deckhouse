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

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	packageruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	packagestatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

const (
	appName      = "test-app"
	appNamespace = "foobar"

	packageName = "test"
	versionName = "deckhouse-test-v1.0.1"
)

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	reconcilertest.Suite

	ctr     *harness
	manager *packageManagerStub
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{
			&v1alpha1.Application{},
			&v1alpha1.ApplicationPackage{},
			&v1alpha1.ApplicationPackageVersion{},
		},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha1.SchemeGroupVersion.WithKind("Application"),
			v1alpha1.SchemeGroupVersion.WithKind("ApplicationPackage"),
			v1alpha1.SchemeGroupVersion.WithKind("ApplicationPackageVersion"),
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

// harness is the controller as RegisterController leaves it behind in the manager: the
// reconcile entry point and the preflight runnable. Tests drive the package through these
// two and through nothing else.
type harness struct {
	reconcile.Reconciler

	preflight ctrlmanager.Runnable
}

// reconcilerFor registers the controller and opens the preflight gate that Reconcile waits
// on, which is the state a running controller reconciles in.
func reconcilerFor(t *testing.T, cl client.Client, manager *packageManagerStub) *harness {
	t.Helper()

	h := registerController(t, cl, manager, modulesInited(true))
	require.NoError(t, h.preflight.Start(context.Background()))

	return h
}

// registerController runs the package's only exported entry point against a manager stub
// and picks up the runnables it registered.
func registerController(t *testing.T, cl client.Client, manager *packageManagerStub, modules modulesInited) *harness {
	t.Helper()

	mgr := &managerStub{scheme: testScheme(t), client: cl}
	require.NoError(t, application.RegisterController(mgr, manager, modules, log.NewNop()))

	h := new(harness)
	for _, runnable := range mgr.runnables {
		if reconciler, ok := runnable.(reconcile.Reconciler); ok {
			h.Reconciler = reconciler
			continue
		}

		h.preflight = runnable
	}

	require.NotNil(t, h.Reconciler, "the controller must be registered in the manager")
	require.NotNil(t, h.preflight, "the preflight must be registered in the manager")

	return h
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

		_, err := suite.ctr.Reconcile(ctx, suite.Request("non-existent-app", appNamespace))
		require.NoError(suite.T(), err)

		assert.Empty(suite.T(), suite.manager.updated)
	})

	suite.Run("successful reconcile hands the application to the runtime", func() {
		suite.setupController("successful-reconcile.yaml")

		result, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)
		assert.True(suite.T(), result.IsZero(), "a settled application must not be requeued")

		require.Len(suite.T(), suite.manager.updated, 1)
		assert.Equal(suite.T(), registry.Remote{
			Name:         "deckhouse",
			Repository:   "registry.example.com/test",
			DockerConfig: "test-docker-cfg",
			CA:           "test-ca",
			Scheme:       "https",
		}, suite.manager.updated[0].repo)
		assert.Equal(suite.T(), packageruntime.App{
			Name:       appName,
			Namespace:  appNamespace,
			Definition: apps.Definition{Name: packageName, Version: "v1.0.1"},
			Settings:   map[string]any{"host": "app.example.com"},
		}, suite.manager.updated[0].app)
	})

	suite.Run("maintenance mode reaches the runtime", func() {
		suite.setupController("maintenance-mode.yaml")

		_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)

		require.Len(suite.T(), suite.manager.updated, 1)
		assert.Equal(suite.T(), "NoResourceReconciliation", suite.manager.updated[0].app.Maintenance)
	})

	suite.Run("missing package requeues and claims the finalizer", func() {
		suite.setupController("package-not-found.yaml")

		result, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 30*time.Second, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated, "an unresolved application must not reach the runtime")
		// The finalizer is claimed before the package lookup, so deletion is guarded
		// even for an application that never resolved.
		assert.Contains(suite.T(), suite.getApplication(appName, appNamespace).Finalizers,
			v1alpha1.ApplicationFinalizerStatisticRegistered)
	})

	suite.Run("missing version requeues", func() {
		suite.setupController("version-not-found.yaml")

		result, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 30*time.Second, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated)
		assert.Empty(suite.T(), suite.getApplicationPackage(packageName).Status.UsedBy)
	})

	suite.Run("draft version requeues", func() {
		suite.setupController("version-is-draft.yaml")

		result, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 30*time.Second, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated, "a draft version must not reach the runtime")
		assert.Empty(suite.T(), suite.getApplicationPackage(packageName).Status.UsedBy)
	})

	suite.Run("missing repository leaves the installed lists untouched", func() {
		suite.setupController("repository-not-found.yaml")

		result, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 30*time.Second, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated)
		assert.Empty(suite.T(), suite.getApplicationPackage(packageName).Status.UsedBy)
		assert.Empty(suite.T(), suite.getApplicationPackageVersion(versionName).Status.UsedBy)
	})

	suite.Run("version switch releases the previous version", func() {
		suite.setupController("version-switch.yaml")

		_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)

		previous := suite.getApplicationPackageVersion(versionName)
		assert.Empty(suite.T(), previous.Status.UsedBy)
		assert.Equal(suite.T(), int32(0), previous.Status.UsedByCount)

		current := suite.getApplicationPackageVersion("deckhouse-test-v1.0.2")
		assert.True(suite.T(), current.IsAppInstalled(appNamespace, appName))
		assert.Equal(suite.T(), int32(1), current.Status.UsedByCount)

		pkg := suite.getApplicationPackage(packageName)
		assert.Equal(suite.T(), "v1.0.2", pkg.GetAppVersion(appNamespace, appName),
			"the package must record the version the application moved to")
		assert.Equal(suite.T(), int32(1), pkg.Status.UsedByCount, "a version bump must not inflate the count")

		app := suite.getApplication(appName, appNamespace)
		assert.Equal(suite.T(), "deckhouse-test-v1.0.2",
			ownerRefName(app, v1alpha1.ApplicationPackageVersionKind))
	})

	suite.Run("package switch releases the previous package", func() {
		suite.setupController("package-switch.yaml")

		_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)

		previousPkg := suite.getApplicationPackage("old-test")
		assert.Empty(suite.T(), previousPkg.Status.UsedBy)
		assert.Equal(suite.T(), int32(0), previousPkg.Status.UsedByCount)

		previousVersion := suite.getApplicationPackageVersion("deckhouse-old-test-v1.0.1")
		assert.Empty(suite.T(), previousVersion.Status.UsedBy)
		assert.Equal(suite.T(), int32(0), previousVersion.Status.UsedByCount)

		assert.True(suite.T(), suite.getApplicationPackage(packageName).IsAppInstalled(appNamespace, appName))
		assert.True(suite.T(), suite.getApplicationPackageVersion(versionName).IsAppInstalled(appNamespace, appName))

		app := suite.getApplication(appName, appNamespace)
		assert.Equal(suite.T(), packageName, ownerRefName(app, v1alpha1.ApplicationPackageKind))
		assert.Equal(suite.T(), versionName, ownerRefName(app, v1alpha1.ApplicationPackageVersionKind))
	})

	suite.Run("stale owner references are replaced and foreign ones kept", func() {
		suite.setupController("stale-owner-references.yaml")

		_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)

		app := suite.getApplication(appName, appNamespace)
		assert.Equal(suite.T(), packageName, ownerRefName(app, v1alpha1.ApplicationPackageKind))
		assert.Equal(suite.T(), versionName, ownerRefName(app, v1alpha1.ApplicationPackageVersionKind))
		assert.Equal(suite.T(), "unrelated", ownerRefName(app, "ConfigMap"),
			"references of other kinds must survive the relink")
	})

	suite.Run("repeated reconcile does not duplicate the installed lists", func() {
		suite.setupController("repeated-reconcile.yaml")

		for range 2 {
			_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
			require.NoError(suite.T(), err)
		}

		pkg := suite.getApplicationPackage(packageName)
		assert.Len(suite.T(), pkg.Status.UsedBy, 1)
		assert.Equal(suite.T(), int32(1), pkg.Status.UsedByCount)

		version := suite.getApplicationPackageVersion(versionName)
		assert.Len(suite.T(), version.Status.UsedBy, 1)
		assert.Equal(suite.T(), int32(1), version.Status.UsedByCount)

		assert.Len(suite.T(), suite.getApplication(appName, appNamespace).OwnerReferences, 2)
	})

	suite.Run("registry spec changed annotation is cleared", func() {
		suite.setupController("registry-spec-changed.yaml")

		_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)

		annotations := suite.getApplication(appName, appNamespace).Annotations
		assert.NotContains(suite.T(), annotations, v1alpha1.ApplicationAnnotationRegistrySpecChanged)
		assert.Contains(suite.T(), annotations, "packages.deckhouse.io/keep-me")
	})

	suite.Run("deleted application is detached and unregistered", func() {
		suite.setupController("delete.yaml")

		_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)

		// The finalizer added by the first reconcile keeps the object around with a
		// deletion timestamp, which is what the second reconcile has to handle.
		require.NoError(suite.T(), suite.Client().Delete(ctx, suite.getApplication(appName, appNamespace)))

		_, err = suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)

		assert.Equal(suite.T(),
			[]types.NamespacedName{{Namespace: appNamespace, Name: appName}}, suite.manager.removed)
		assert.Empty(suite.T(), suite.getApplicationPackage(packageName).Status.UsedBy)
		assert.Empty(suite.T(), suite.getApplicationPackageVersion(versionName).Status.UsedBy)

		err = suite.Client().Get(ctx, client.ObjectKey{Namespace: appNamespace, Name: appName},
			new(v1alpha1.Application))
		assert.True(suite.T(), apierrors.IsNotFound(err), "the finalizer must be released")
	})

	suite.Run("deleted application is detached from the version it was attached to", func() {
		suite.setupController("delete-after-version-edit.yaml")

		require.NoError(suite.T(), suite.Client().Delete(ctx, suite.getApplication(appName, appNamespace)))

		_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)

		// The spec was bumped to v1.0.2 before the deletion was handled, so detaching by
		// spec would leave the version the application actually ran from marked as used.
		attached := suite.getApplicationPackageVersion(versionName)
		assert.Empty(suite.T(), attached.Status.UsedBy)
		assert.Equal(suite.T(), int32(0), attached.Status.UsedByCount)

		assert.Empty(suite.T(), suite.getApplicationPackage(packageName).Status.UsedBy)
	})

	suite.Run("deleted application whose lists were already released is still finished", func() {
		suite.setupController("delete-already-released.yaml")

		require.NoError(suite.T(), suite.Client().Delete(ctx, suite.getApplication(appName, appNamespace)))

		_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err, "a retried deletion must not fail on lists it has already released")

		assert.True(suite.T(), suite.getApplicationPackage(packageName).IsAppInstalled(appNamespace, "other-app"))
		assert.True(suite.T(), suite.getApplicationPackageVersion(versionName).IsAppInstalled(appNamespace, "other-app"))

		err = suite.Client().Get(ctx, client.ObjectKey{Namespace: appNamespace, Name: appName},
			new(v1alpha1.Application))
		assert.True(suite.T(), apierrors.IsNotFound(err), "the finalizer must be released")
	})
}

func (suite *ControllerTestSuite) getApplication(name, namespace string) *v1alpha1.Application {
	app := new(v1alpha1.Application)
	require.NoError(suite.T(),
		suite.Client().Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, app))

	return app
}

func (suite *ControllerTestSuite) getApplicationPackage(name string) *v1alpha1.ApplicationPackage {
	pkg := new(v1alpha1.ApplicationPackage)
	require.NoError(suite.T(), suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, pkg))

	return pkg
}

func (suite *ControllerTestSuite) getApplicationPackageVersion(name string) *v1alpha1.ApplicationPackageVersion {
	apv := new(v1alpha1.ApplicationPackageVersion)
	require.NoError(suite.T(), suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, apv))

	return apv
}

func TestReconcileFailsOnGetError(t *testing.T) {
	getErr := errors.New("api server is unreachable")

	cl := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return getErr
			},
		}).
		Build()

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	result, err := ctr.Reconcile(context.Background(), request(appName, appNamespace))
	require.ErrorIs(t, err, getErr, "a transient read failure must be retried by the queue, not swallowed")
	assert.True(t, result.IsZero())
	assert.Empty(t, manager.updated)
}

func TestRelinkFailureKeepsTheApplicationOutOfTheRuntime(t *testing.T) {
	patchErr := errors.New("status patch rejected")

	cl := seedFakeClient(t, "successful-reconcile.yaml", interceptor.Funcs{
		SubResourcePatch: func(context.Context, client.Client, string, client.Object, client.Patch,
			...client.SubResourcePatchOption) error {
			return patchErr
		},
	})

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	result, err := ctr.Reconcile(context.Background(), request(appName, appNamespace))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, result.RequeueAfter)

	// The installed lists are the record of what the runtime is allowed to run, so an
	// application that failed to claim them must not be handed over.
	assert.Empty(t, manager.updated)
}

func TestDeleteFailureKeepsTheFinalizer(t *testing.T) {
	patchErr := errors.New("status patch rejected")

	// Only the package write fails, so the deletion gets half-way: the version is
	// released and the package is not.
	cl := seedFakeClient(t, "delete-after-version-edit.yaml", interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, cl client.Client, name string, obj client.Object,
			patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if _, ok := obj.(*v1alpha1.ApplicationPackage); ok {
				return patchErr
			}

			return cl.Status().Patch(ctx, obj, patch)
		},
	})

	app := new(v1alpha1.Application)
	require.NoError(t, cl.Get(context.Background(), objectKey(appName, appNamespace), app))
	require.NoError(t, cl.Delete(context.Background(), app))

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	_, err := ctr.Reconcile(context.Background(), request(appName, appNamespace))
	require.ErrorIs(t, err, patchErr)

	// Releasing the finalizer here would drop the application while the package still
	// counts it as an installation, and nothing would ever come back to fix that.
	require.NoError(t, cl.Get(context.Background(), objectKey(appName, appNamespace), app))
	assert.Contains(t, app.Finalizers, v1alpha1.ApplicationFinalizerStatisticRegistered)
	assert.Empty(t, manager.removed, "the runtime must keep the application until it is detached")
}

func TestDeleteDistinguishesAMissingVersionFromAnUnreadableOne(t *testing.T) {
	cl := seedFakeClient(t, "delete-after-version-edit.yaml", interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object,
			opts ...client.GetOption) error {
			if _, ok := obj.(*v1alpha1.ApplicationPackageVersion); ok {
				return apierrors.NewInternalError(errors.New("etcd is unavailable"))
			}

			return cl.Get(ctx, key, obj, opts...)
		},
	})

	app := new(v1alpha1.Application)
	require.NoError(t, cl.Get(context.Background(), objectKey(appName, appNamespace), app))
	require.NoError(t, cl.Delete(context.Background(), app))

	manager := newPackageManagerStub(t)
	ctr := reconcilerFor(t, cl, manager)

	// A version that is gone needs no cleanup, but a version that cannot be read is not
	// gone: treating the two alike would leak the installation entry.
	_, err := ctr.Reconcile(context.Background(), request(appName, appNamespace))
	require.Error(t, err)
	assert.Empty(t, manager.removed)
}

func TestPreflightPreservesEveryApplication(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(
			newApplication(appName, appNamespace, packageName, "v1.0.1"),
			newApplication("other-app", "baz", "other-test", "v2.0.0"),
		).
		Build()

	manager := newPackageManagerStub(t)
	ctr := registerController(t, cl, manager, modulesInited(true))
	require.NoError(t, ctr.preflight.Start(context.Background()))

	require.Len(t, manager.cleanups, 1)
	assert.ElementsMatch(t, []packageruntime.PreservePackage{
		{
			PackageName:      packageName,
			Repository:       "deckhouse",
			Version:          "v1.0.1",
			ReleaseName:      appNamespace + "." + appName,
			ReleaseNamespace: appNamespace,
		},
		{
			PackageName:      "other-test",
			Repository:       "deckhouse",
			Version:          "v2.0.0",
			ReleaseName:      "baz.other-app",
			ReleaseNamespace: "baz",
		},
	}, manager.cleanups[0])
}

func TestPreflightWaitsForModuleManager(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()

	manager := newPackageManagerStub(t)
	ctr := registerController(t, cl, manager, modulesInited(false))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, ctr.preflight.Start(ctx), context.DeadlineExceeded)
	assert.Empty(t, manager.cleanups,
		"runtime state must not be dropped while the module manager is still initialising")
}

// seedFakeClient builds a client from a fixture and wraps it with funcs, for the paths that
// only show up when a write or a read fails. Seeding itself goes through Create, which the
// interceptors used here leave alone.
func seedFakeClient(t *testing.T, fixture string, funcs interceptor.Funcs) client.Client {
	t.Helper()

	scheme := testScheme(t)

	raw, err := reconcilertest.LoadFixture("./testdata", fixture)
	require.NoError(t, err)

	objs, err := reconcilertest.Decode(scheme, raw)
	require.NoError(t, err)

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1alpha1.Application{}, &v1alpha1.ApplicationPackage{},
			&v1alpha1.ApplicationPackageVersion{}).
		WithInterceptorFuncs(funcs).
		Build()

	for _, obj := range objs {
		require.NoError(t, cl.Create(context.TODO(), obj))
	}

	return cl
}

func request(name, namespace string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}
}

func objectKey(name, namespace string) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: name}
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme, err := project.Scheme()
	require.NoError(t, err)

	return scheme
}

func newApplication(name, namespace, pkg, version string) *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.ApplicationSpec{
			PackageName:           pkg,
			PackageRepositoryName: "deckhouse",
			PackageVersion:        version,
		},
	}
}

// ownerRefName is the read-only counterpart of ctrlutils.OwnerRefName, kept local so the
// assertions do not depend on the helper under test.
func ownerRefName(app *v1alpha1.Application, kind string) string {
	for _, ref := range app.GetOwnerReferences() {
		if ref.Kind == kind {
			return ref.Name
		}
	}

	return ""
}

// modulesInited is a moduleManager whose readiness is fixed at construction.
type modulesInited bool

func (m modulesInited) AreModulesInited() bool { return bool(m) }

// packageManagerStub records the calls the reconciler makes into the package runtime.
// RegisterController requires it to satisfy the package's manager interface, which is the
// compile-time check that this stub still matches the real runtime.
type packageManagerStub struct {
	updated  []updatedApp
	removed  []types.NamespacedName
	cleanups [][]packageruntime.PreservePackage

	queue workqueue.TypedRateLimitingInterface[string]
}

// newPackageManagerStub hands out a real queue: RegisterController starts the status
// service on it, and that goroutine only exits once the queue is shut down.
func newPackageManagerStub(t *testing.T) *packageManagerStub {
	t.Helper()

	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	t.Cleanup(queue.ShutDown)

	return &packageManagerStub{queue: queue}
}

type updatedApp struct {
	repo registry.Remote
	app  packageruntime.App
}

func (s *packageManagerStub) UpdateApp(repo registry.Remote, app packageruntime.App) {
	s.updated = append(s.updated, updatedApp{repo: repo, app: app})
}

func (s *packageManagerStub) RemoveApp(namespace, name string) {
	s.removed = append(s.removed, types.NamespacedName{Namespace: namespace, Name: name})
}

func (s *packageManagerStub) GetStatus(string) packagestatus.Status {
	return packagestatus.Status{}
}

func (s *packageManagerStub) GetAppStatusQueue() workqueue.TypedRateLimitingInterface[string] {
	return s.queue
}

func (s *packageManagerStub) Cleanup(_ context.Context, preserve []packageruntime.PreservePackage) {
	s.cleanups = append(s.cleanups, preserve)
}
