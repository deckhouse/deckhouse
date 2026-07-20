// Copyright 2024 Flant JSC
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

package docbuilder

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	docs_builder "github.com/deckhouse/deckhouse/go_lib/module/docs-builder"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/reconcilertest"
)

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	reconcilertest.Suite

	ctr    *reconciler
	tmpDir string
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.tmpDir = suite.T().TempDir()
	suite.T().Setenv(d8env.DownloadedModulesDir, suite.tmpDir)
	_ = os.MkdirAll(filepath.Join(suite.tmpDir, "modules"), 0o777)

	suite.Init(reconcilertest.Config{
		StatusSubresources: []client.Object{&v1alpha1.ModuleDocumentation{}},
		SnapshotKinds: []schema.GroupVersionKind{
			v1alpha1.SchemeGroupVersion.WithKind("ModuleDocumentation"),
		},
		ObjectNormalizers: []reconcilertest.ObjectNormalizer{suite.normalizeTmpDir},
		GoldenMode:        reconcilertest.WholeDocument,
	})
}

// normalizeTmpDir replaces the random temp dir in condition messages with a
// stable placeholder for golden comparison.
func (suite *ControllerTestSuite) normalizeTmpDir(obj client.Object) {
	md, ok := obj.(*v1alpha1.ModuleDocumentation)
	if !ok {
		return
	}
	for i := range md.Status.Conditions {
		md.Status.Conditions[i].Message = strings.ReplaceAll(md.Status.Conditions[i].Message, suite.tmpDir, "/testdir")
	}
}

func (suite *ControllerTestSuite) setupController(filename string) {
	suite.Seed(filename)

	dc := dependency.NewDependencyContainer()
	suite.ctr = &reconciler{
		client:               suite.Client(),
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		logger:               log.NewNop(),
		docsBuilder:          docs_builder.NewClient(dc.GetHTTPClient()),
		dc:                   dc,
	}
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("with no builder endpoints", func() {
		suite.setupController("no-builders.yaml")

		md := suite.getModuleDocumentation("testmodule")
		_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		require.NoError(suite.T(), err)
	})

	suite.Run("with only one builder", func() {
		suite.prepareModuleOpenAPI("testmodule")
		dependency.TestDC.HTTPClient.DoMock.Set(docBuildResponder("/api/v1/doc/testmodule/v1.0.0"))

		suite.setupController("one-builder.yaml")

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("with two builders", func() {
		suite.prepareModuleOpenAPI("testmodule")
		dependency.TestDC.HTTPClient.DoMock.Set(docBuildResponder("/api/v1/doc/testmodule/v1.0.0"))

		suite.setupController("two-builders.yaml")

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("one builder cannot render", func() {
		suite.prepareModuleOpenAPI("testmodule")
		dependency.TestDC.HTTPClient.DoMock.Set(func(req *http.Request) (*http.Response, error) {
			if strings.HasPrefix(req.Host, "10-111-111-11") {
				return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader("I'm broken"))}, nil
			}

			switch req.URL.Path {
			case "/api/v1/doc/testmodule/v1.0.0":
				return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(""))}, nil

			case "/api/v1/build":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			}
			return &http.Response{StatusCode: http.StatusBadRequest}, nil
		})

		suite.setupController("two-builders-partially.yaml")

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.Equal(suite.T(), res.RequeueAfter, defaultDocumentationCheckInterval)
		require.NoError(suite.T(), err)
	})

	suite.Run("render new version", func() {
		suite.prepareModuleOpenAPI("testmodule")
		dependency.TestDC.HTTPClient.DoMock.Set(docBuildResponder("/api/v1/doc/testmodule/v1.1.1"))

		suite.setupController("render-new-version.yaml")

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("render new lease", func() {
		suite.prepareModuleOpenAPI("testmodule")
		dependency.TestDC.HTTPClient.DoMock.Set(docBuildResponder("/api/v1/doc/testmodule/v1.1.1"))

		suite.setupController("render-new-lease.yaml")

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("render new checksum", func() {
		suite.prepareModuleOpenAPI("testmodule")
		dependency.TestDC.HTTPClient.DoMock.Set(docBuildResponder("/api/v1/doc/testmodule/mpo-tag"))

		suite.setupController("render-new-checksum.yaml")

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("keep up-to-date rendered documentation", func() {
		suite.prepareModuleOpenAPI("testmodule")
		dependency.TestDC.GetClock().(*clockwork.FakeClock).Advance(1 * time.Hour)

		suite.setupController("keep-actual.yaml")

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("with empty dir", func() {
		suite.setupController("empty-dir.yaml")

		md := suite.getModuleDocumentation("absentmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.Equal(suite.T(), res.RequeueAfter, defaultDocumentationCheckInterval)
		require.NoError(suite.T(), err)
	})
}

// prepareModuleOpenAPI lays down a minimal module openapi config so the builder
// has something to read.
func (suite *ControllerTestSuite) prepareModuleOpenAPI(module string) {
	openapiDir := filepath.Join(suite.tmpDir, "modules", module, "openapi")
	_ = os.MkdirAll(openapiDir, 0o777)
	_ = os.WriteFile(filepath.Join(openapiDir, "config-values.yaml"), []byte("{}"), 0o666)
}

// docBuildResponder answers the doc-upload path with 201 and the build path with
// 200, and anything else with 400.
func docBuildResponder(docPath string) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case docPath:
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(""))}, nil
		case "/api/v1/build":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		return &http.Response{StatusCode: http.StatusBadRequest}, nil
	}
}

// nolint:unparam
func (suite *ControllerTestSuite) getModuleDocumentation(name string) *v1alpha1.ModuleDocumentation {
	var md v1alpha1.ModuleDocumentation
	err := suite.Client().Get(context.TODO(), client.ObjectKey{Name: name}, &md)
	require.NoError(suite.T(), err)

	return &md
}
