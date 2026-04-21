// Copyright 2025 Flant JSC
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

package packagerepositoryoperation

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	internalRegistryClient "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/client"
	registryService "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/service"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/service/mock"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
	fakeRegistry "github.com/deckhouse/deckhouse/pkg/registry/fake"
)

// errorInjectingClient wraps a client.Client and returns errors for Create on specific object names
type errorInjectingClient struct {
	client.Client
	createErrorNames map[string]error
}

func (c *errorInjectingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if c.createErrorNames != nil {
		if err, ok := c.createErrorNames[obj.GetName()]; ok {
			return err
		}
	}
	return c.Client.Create(ctx, obj, opts...)
}

// ----- fake registry helpers -----

const registryHost = "registry.example.com/test"

// newInternalClient creates a registry.Client backed by in-memory fake registries.
func newInternalClient(registries ...*fakeRegistry.Registry) registry.Client {
	return internalRegistryClient.NewFromRegistryClient(fakeRegistry.NewClient(registries...))
}

// createFakePSM creates a PackageServiceManager that returns a PackagesService
// backed by the given registry.Client.
func createFakePSM(ic registry.Client) registryService.ServiceManagerInterface[registryService.PackagesService] {
	psm := mock.NewServiceManagerMock[registryService.PackagesService](&testing.T{})
	svc := registryService.NewPackagesService(ic, log.NewNop())
	psm.ServiceMock.Return(svc, nil)
	return psm
}

// applicationVersionImage builds a version image with Application type label and package.yaml.
func applicationVersionImage() *fakeRegistry.ImageBuilder {
	return fakeRegistry.NewImageBuilder().
		WithLabel("io.deckhouse.package.type", "Application").
		WithFile("package.yaml", "type: Application\n")
}

// moduleVersionImage builds a version image with Module type label and package.yaml.
func moduleVersionImage() *fakeRegistry.ImageBuilder {
	return fakeRegistry.NewImageBuilder().
		WithLabel("io.deckhouse.package.type", "Module").
		WithFile("package.yaml", "type: Module\n")
}

// invalidTypeVersionImage builds a version image with an unrecognized package type.
func invalidTypeVersionImage() *fakeRegistry.ImageBuilder {
	return fakeRegistry.NewImageBuilder().
		WithLabel("io.deckhouse.package.type", "Garbage").
		WithFile("package.yaml", "type: Garbage\n")
}

// ----- error injection wrappers -----

// errorListTagsClient wraps a registry.Client and forces ListTags to
// return an error. Used by the "package listing failed" test.
type errorListTagsClient struct {
	registry.Client
}

func (c *errorListTagsClient) WithSegment(segments ...string) registry.Client {
	return &errorListTagsClient{Client: c.Client.WithSegment(segments...)}
}

func (c *errorListTagsClient) ListTags(_ context.Context, _ ...registry.ListTagsOption) ([]string, error) {
	return nil, assert.AnError
}

// legacyRegistryClient wraps a registry.Client and overrides ListTags
// on the "version" segment to return a NAME_UNKNOWN transport error, simulating
// a legacy registry that has no /version path.
type legacyRegistryClient struct {
	registry.Client
	segments []string
}

func (c *legacyRegistryClient) WithSegment(segments ...string) registry.Client {
	return &legacyRegistryClient{
		Client:   c.Client.WithSegment(segments...),
		segments: append(append([]string{}, c.segments...), segments...),
	}
}

func (c *legacyRegistryClient) ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error) {
	for _, s := range c.segments {
		if s == "version" {
			return nil, &transport.Error{
				Errors: []transport.Diagnostic{{
					Code:    transport.NameUnknownErrorCode,
					Message: "repository name not known to registry",
				}},
				StatusCode: http.StatusNotFound,
			}
		}
	}
	return c.Client.ListTags(ctx, opts...)
}

var (
	golden     bool
	mDelimiter *regexp.Regexp
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
	mDelimiter = regexp.MustCompile("(?m)^---$")
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite

	kubeClient       client.Client
	ctr              *reconciler
	testDataFileName string
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
}

func (suite *ControllerTestSuite) SetupSubTest() {
	dependency.TestDC.HTTPClient.DoMock.
		Expect(&http.Request{}).
		Return(&http.Response{
			StatusCode: http.StatusOK,
		}, nil)
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	if suite.T().Skipped() {
		return
	}

	goldenFile := filepath.Join("./testdata", "golden", suite.testDataFileName)
	gotB := suite.fetchResults()

	if golden {
		err := os.WriteFile(goldenFile, gotB, 0o666)
		require.NoError(suite.T(), err)
	} else {
		got := singleDocToManifests(gotB)

		expB, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		exp := singleDocToManifests(expB)
		// "there is no authorization data"
		assert.Equal(suite.T(), len(got), len(exp), "The number of `got` manifests must be equal to the number of `exp` manifests")

		for i := range got {
			if assert.YAMLEq(suite.T(), exp[i], got[i], "Got and exp manifests must match") {
				suite.T().Logf("test data file: %s", goldenFile)
			}
		}
	}
}

type reconcilerOption func(*reconciler)

func withPackageServiceManager(psm registryService.ServiceManagerInterface[registryService.PackagesService]) reconcilerOption {
	return func(r *reconciler) {
		r.psm = psm
	}
}

func (suite *ControllerTestSuite) setupController(filename string, options ...reconcilerOption) {
	suite.testDataFileName = filename
	suite.ctr, suite.kubeClient = setupFakeController(suite.T(), filename)

	for _, opt := range options {
		opt(suite.ctr)
	}
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	var operationList v1alpha1.PackageRepositoryOperationList
	err := suite.kubeClient.List(context.TODO(), &operationList)
	require.NoError(suite.T(), err)

	for _, item := range operationList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var appList v1alpha1.ApplicationList
	err = suite.kubeClient.List(context.TODO(), &appList)
	require.NoError(suite.T(), err)

	for _, item := range appList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var apvList v1alpha1.ApplicationPackageVersionList
	err = suite.kubeClient.List(context.TODO(), &apvList)
	require.NoError(suite.T(), err)

	for _, item := range apvList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var mpvList v1alpha1.ModulePackageVersionList
	err = suite.kubeClient.List(context.TODO(), &mpvList)
	require.NoError(suite.T(), err)

	for _, item := range mpvList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var mpList v1alpha1.ModulePackageList
	err = suite.kubeClient.List(context.TODO(), &mpList)
	require.NoError(suite.T(), err)

	for _, item := range mpList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	var repoList v1alpha1.PackageRepositoryList
	err = suite.kubeClient.List(context.TODO(), &repoList)
	require.NoError(suite.T(), err)

	for _, item := range repoList.Items {
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	// Normalize timestamps for consistent golden files
	resultStr := result.String()
	resultStr = regexp.MustCompile(`startTime: "[^"]*"`).ReplaceAllString(resultStr, `startTime: "2025-10-31T12:00:00Z"`)
	resultStr = regexp.MustCompile(`syncTime: "[^"]*"`).ReplaceAllString(resultStr, `syncTime: "2025-10-31T12:00:00Z"`)
	resultStr = regexp.MustCompile(`completionTime: "[^"]*"`).ReplaceAllString(resultStr, `completionTime: "2025-10-31T12:00:00Z"`)

	return []byte(resultStr)
}

func singleDocToManifests(doc []byte) []string {
	split := mDelimiter.Split(string(doc), -1)

	result := make([]string, 0, len(split))
	for i := range split {
		if split[i] != "" {
			result = append(result, split[i])
		}
	}

	return result
}

func setupFakeController(t *testing.T, filename string) (*reconciler, client.Client) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	kubeClient := k8sfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects().
		WithStatusSubresource(&v1alpha1.PackageRepositoryOperation{}).
		WithStatusSubresource(&v1alpha1.ApplicationPackage{}).
		WithStatusSubresource(&v1alpha1.ApplicationPackageVersion{}).
		WithStatusSubresource(&v1alpha1.ModulePackage{}).
		WithStatusSubresource(&v1alpha1.ModulePackageVersion{}).
		WithStatusSubresource(&v1alpha1.PackageRepository{}).
		Build()

	ctr := &reconciler{
		client: kubeClient,
		logger: log.NewLogger(log.WithLevel(slog.LevelDebug)), // return nop
		dc:     dependency.NewMockedContainer(),
	}

	// Load test data from file
	testDataPath := filepath.Join("./testdata", filename)
	if _, err := os.Stat(testDataPath); err == nil {
		data, err := os.ReadFile(testDataPath)
		require.NoError(t, err)

		// Parse and create objects
		manifests := singleDocToManifests(data)
		for _, manifest := range manifests {
			if manifest == "" {
				continue
			}

			var obj metav1.PartialObjectMetadata
			err := yaml.Unmarshal([]byte(manifest), &obj)
			require.NoError(t, err)

			switch obj.Kind {
			case "PackageRepositoryOperation":
				var operation v1alpha1.PackageRepositoryOperation
				err := yaml.Unmarshal([]byte(manifest), &operation)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &operation))
			case "Application":
				var app v1alpha1.Application
				err := yaml.Unmarshal([]byte(manifest), &app)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &app))
			case "ApplicationPackageVersion":
				var apv v1alpha1.ApplicationPackageVersion
				err := yaml.Unmarshal([]byte(manifest), &apv)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &apv))
			case "PackageRepository":
				var repo v1alpha1.PackageRepository
				err := yaml.Unmarshal([]byte(manifest), &repo)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &repo))
			case "ModulePackageVersion":
				var mpv v1alpha1.ModulePackageVersion
				err := yaml.Unmarshal([]byte(manifest), &mpv)
				require.NoError(t, err)
				savedStatus := mpv.Status
				require.NoError(t, kubeClient.Create(context.TODO(), &mpv))
				mpv.Status = savedStatus
				require.NoError(t, kubeClient.Status().Update(context.TODO(), &mpv))
			case "ModulePackage":
				var mp v1alpha1.ModulePackage
				err := yaml.Unmarshal([]byte(manifest), &mp)
				require.NoError(t, err)
				require.NoError(t, kubeClient.Create(context.TODO(), &mp))
			}
		}
	}

	return ctr, kubeClient
}

func (suite *ControllerTestSuite) TestReconcile() {
	ctx := context.Background()

	suite.Run("resource not found", func() {
		suite.setupController("resource-not-found.yaml")
		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: k8stypes.NamespacedName{Name: "non-existent-operation"},
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("package repository not found", func() {
		suite.setupController("package-repository-not-found.yaml")
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("registry client creation failed", func() {
		// Use an empty PSM (no pre-configured services) - PackagesService will fail
		// because there's no service for the registry URL and it can't create one dynamically
		emptyPSM := registryService.NewPackageServiceManager(log.NewNop())

		suite.setupController("registry-client-failed.yaml", withPackageServiceManager(emptyPSM))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("package listing failed", func() {
		// ListTags at root level returns an error.
		reg := fakeRegistry.NewRegistry(registryHost)
		ic := &errorListTagsClient{Client: newInternalClient(reg)}
		psm := createFakePSM(ic)

		suite.setupController("package-listing-failed.yaml", withPackageServiceManager(psm))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("successful package discovery", func() {
		// Root has "test-package" (non-semver → 0 versions → discovery only, no version resources).
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("successful-discovery.yaml", withPackageServiceManager(psm))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("successful completion", func() {
		// Root has "v1.0.0" (treated as package name), version path also has "v1.0.0" (valid semver).
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "v1.0.0", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("v1.0.0/version", "v1.0.0", applicationVersionImage().MustBuild())
		reg.MustAddImage("v1.0.0", "v1.0.0", fakeRegistry.NewImageBuilder().MustBuild()) // bundle image

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("successful-completion.yaml", withPackageServiceManager(psm))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("successful module completion", func() {
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("test-package/version", "v1.0.0", moduleVersionImage().MustBuild())

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("successful-module-completion.yaml", withPackageServiceManager(psm))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("failed versions from registry", func() {
		// Root: "test-package", Versions: v1.0.0, v1.1.0, v1.2.0 (Application type).
		// k8s-level Create errors injected for v1.1.0 and v1.2.0.
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		appImg := applicationVersionImage().MustBuild()
		for _, v := range []string{"v1.0.0", "v1.1.0", "v1.2.0"} {
			reg.MustAddImage("test-package/version", v, appImg)
			reg.MustAddImage("test-package", v, fakeRegistry.NewImageBuilder().MustBuild()) // bundle
		}

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("failed-versions.yaml", withPackageServiceManager(psm))

		errorClient := &errorInjectingClient{
			Client: suite.kubeClient,
			createErrorNames: map[string]error{
				"deckhouse-test-package-v1.1.0": fmt.Errorf("simulated create error for v1.1.0"),
				"deckhouse-test-package-v1.2.0": fmt.Errorf("simulated create error for v1.2.0"),
			},
		}
		suite.ctr.client = errorClient

		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("failed module versions from registry", func() {
		// Same as "failed versions" but Module type (no bundle images needed).
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		modImg := moduleVersionImage().MustBuild()
		for _, v := range []string{"v1.0.0", "v1.1.0", "v1.2.0"} {
			reg.MustAddImage("test-package/version", v, modImg)
		}

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("failed-module-versions.yaml", withPackageServiceManager(psm))

		errorClient := &errorInjectingClient{
			Client: suite.kubeClient,
			createErrorNames: map[string]error{
				"deckhouse-test-package-v1.1.0": fmt.Errorf("simulated create error for v1.1.0"),
				"deckhouse-test-package-v1.2.0": fmt.Errorf("simulated create error for v1.2.0"),
			},
		}
		suite.ctr.client = errorClient

		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("incremental module scan", func() {
		// Pre-existing ModulePackageVersion v1.0.0 already processed.
		// Registry has v1.0.0 and v1.1.0 — incremental scan should only create v1.1.0.
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		modImg := moduleVersionImage().MustBuild()
		reg.MustAddImage("test-package/version", "v1.0.0", modImg)
		reg.MustAddImage("test-package/version", "v1.1.0", modImg)

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("incremental-module-scan.yaml", withPackageServiceManager(psm))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("version image without metadata", func() {
		// Version image has no labels and no package.yaml → errTooOldImage, skip.
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("test-package/version", "v1.0.0", fakeRegistry.NewImageBuilder().MustBuild())

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("legacy-module.yaml", withPackageServiceManager(psm))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})
			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("legacy module from old registry", func() {
		// /version path returns NAME_UNKNOWN → fallback to /release path.
		// /release has semver tags + channel names.
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("test-package/release", "v1.0.0", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("test-package/release", "stable", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("test-package/release", "early-access", fakeRegistry.NewImageBuilder().MustBuild())
		// Wrap with legacyRegistryClient to return NAME_UNKNOWN on /version
		ic := &legacyRegistryClient{Client: newInternalClient(reg)}
		psm := createFakePSM(ic)

		suite.setupController("legacy-module-old-registry.yaml", withPackageServiceManager(psm))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})
			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("invalid package type", func() {
		// Version image has label "Garbage" → errPackageTypeInvalid.
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("test-package/version", "v1.0.0", invalidTypeVersionImage().MustBuild())

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("invalid-package-type.yaml", withPackageServiceManager(psm))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})
			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("old image without any metadata", func() {
		// No labels, no package.yaml → errTooOldImage.
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("test-package/version", "v1.0.0", fakeRegistry.NewImageBuilder().MustBuild())

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("old-image-no-metadata.yaml", withPackageServiceManager(psm))
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})
			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("cleanup old operations", func() {
		suite.setupController("cleanup-old-operations.yaml")
		operation := suite.getPackageRepositoryOperation("deckhouse-scan-7")

		_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
			NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("no bundle image in registry", func() {
		// Version image exists at test-package/version but bundle image does NOT
		// exist at test-package. Pre-existing ApplicationPackageVersion is marked
		// exist-in-registry=false; CheckImageExists should confirm it's still missing.
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("test-package/version", "v1.0.0", applicationVersionImage().MustBuild())
		// Intentionally NOT adding reg.MustAddImage("test-package", "v1.0.0", ...) → bundle missing

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("no-bundle-image.yaml", withPackageServiceManager(psm))

		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})

	suite.Run("bundle image has arrived for failed package version", func() {
		// Same as "no bundle image" but the bundle IS present now.
		// Pre-existing ApplicationPackageVersion with exist-in-registry=false
		// should be updated to exist-in-registry=true.
		reg := fakeRegistry.NewRegistry(registryHost)
		reg.MustAddImage("", "test-package", fakeRegistry.NewImageBuilder().MustBuild())
		reg.MustAddImage("test-package/version", "v1.0.0", applicationVersionImage().MustBuild())
		reg.MustAddImage("test-package", "v1.0.0", fakeRegistry.NewImageBuilder().MustBuild()) // bundle exists

		psm := createFakePSM(newInternalClient(reg))

		suite.setupController("bundle-image-has-arrived-for-failed-package-version.yaml", withPackageServiceManager(psm))

		operation := suite.getPackageRepositoryOperation("deckhouse-scan-1571326380")

		err := repeat(func() error {
			_, err := suite.ctr.Reconcile(ctx, ctrl.Request{
				NamespacedName: k8stypes.NamespacedName{Name: operation.Name},
			})

			return err
		})

		require.NoError(suite.T(), err)
	})
}

// nolint:unparam
func (suite *ControllerTestSuite) getPackageRepositoryOperation(name string) *v1alpha1.PackageRepositoryOperation {
	var operation v1alpha1.PackageRepositoryOperation
	err := suite.kubeClient.Get(context.TODO(), k8stypes.NamespacedName{Name: name}, &operation)
	require.NoError(suite.T(), err)
	return &operation
}

const repeatTime = 20

func repeat(fn func() error) error {
	for i := 0; i < repeatTime; i++ {
		err := fn()
		if err != nil {
			return err
		}
	}

	return nil
}
