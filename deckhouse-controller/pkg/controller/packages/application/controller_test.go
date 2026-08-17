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

package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	packageruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	packagestatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
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

	ctr     *reconciler
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

	suite.manager = new(packageManagerStub)
	suite.ctr = newReconciler(suite.Client(), suite.manager, modulesInited(true))
}

func newReconciler(cl client.Client, manager packageManager, modules moduleManager) *reconciler {
	return &reconciler{
		init:          new(sync.WaitGroup),
		client:        cl,
		manager:       manager,
		moduleManager: modules,
		logger:        log.NewNop(),
	}
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
		assert.Equal(suite.T(), defaultRequeueAfter, result.RequeueAfter)

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
		assert.Equal(suite.T(), defaultRequeueAfter, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated)
		assert.Empty(suite.T(), suite.getApplicationPackage(packageName).Status.UsedBy)
	})

	suite.Run("draft version requeues", func() {
		suite.setupController("version-is-draft.yaml")

		result, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), defaultRequeueAfter, result.RequeueAfter)

		assert.Empty(suite.T(), suite.manager.updated, "a draft version must not reach the runtime")
		assert.Empty(suite.T(), suite.getApplicationPackage(packageName).Status.UsedBy)
	})

	suite.Run("missing repository leaves the installed lists untouched", func() {
		suite.setupController("repository-not-found.yaml")

		result, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), defaultRequeueAfter, result.RequeueAfter)

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

	suite.Run("deleted application without owner references releases the finalizer", func() {
		suite.setupController("delete-not-attached.yaml")

		require.NoError(suite.T(), suite.Client().Delete(ctx, suite.getApplication(appName, appNamespace)))

		_, err := suite.ctr.Reconcile(ctx, suite.Request(appName, appNamespace))
		require.NoError(suite.T(), err)

		// Nothing to detach, and the entries another application owns stay in place.
		pkg := suite.getApplicationPackage(packageName)
		assert.Equal(suite.T(), int32(1), pkg.Status.UsedByCount)
		assert.True(suite.T(), pkg.IsAppInstalled(appNamespace, "other-app"))

		version := suite.getApplicationPackageVersion(versionName)
		assert.Equal(suite.T(), int32(1), version.Status.UsedByCount)
		assert.True(suite.T(), version.IsAppInstalled(appNamespace, "other-app"))

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

	manager := new(packageManagerStub)
	ctr := newReconciler(cl, manager, modulesInited(true))

	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: appNamespace, Name: appName}}

	result, err := ctr.Reconcile(context.Background(), request)
	require.ErrorIs(t, err, getErr, "a transient read failure must be retried by the queue, not swallowed")
	assert.True(t, result.IsZero())
	assert.Empty(t, manager.updated)
}

func TestPreflightPreservesEveryApplication(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(
			newApplication(appName, appNamespace, packageName, "v1.0.1"),
			newApplication("other-app", "baz", "other-test", "v2.0.0"),
		).
		Build()

	manager := new(packageManagerStub)
	ctr := newReconciler(cl, manager, modulesInited(true))
	// preflight closes the init gate that RegisterController opens.
	ctr.init.Add(1)

	require.NoError(t, ctr.preflight(context.Background()))

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

	manager := new(packageManagerStub)
	ctr := newReconciler(cl, manager, modulesInited(false))
	ctr.init.Add(1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, ctr.preflight(ctx), context.DeadlineExceeded)
	assert.Empty(t, manager.cleanups,
		"runtime state must not be dropped while the module manager is still initialising")
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

var _ packageManager = (*packageManagerStub)(nil)

// packageManagerStub records the calls the reconciler makes into the package runtime.
type packageManagerStub struct {
	updated  []updatedApp
	removed  []types.NamespacedName
	cleanups [][]packageruntime.PreservePackage
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
	return nil
}

func (s *packageManagerStub) Cleanup(_ context.Context, preserve []packageruntime.PreservePackage) {
	s.cleanups = append(s.cleanups, preserve)
}
