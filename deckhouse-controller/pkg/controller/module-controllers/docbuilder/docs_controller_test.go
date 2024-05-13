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
	"bytes"
	"context"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/releaseutil"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	docs_builder "github.com/deckhouse/deckhouse/go_lib/module/docs-builder"
)

var golden bool

func init() {
	flag.BoolVar(&golden, "false", false, "generate golden files")
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}

type ControllerTestSuite struct {
	suite.Suite

	kubeClient client.Client
	ctr        *moduleDocumentationReconciler

	tmpDir           string
	testDataFileName string
}

func (suite *ControllerTestSuite) TearDownSubTest() {
	goldenFile := filepath.Join("./testdata", "golden", suite.testDataFileName)
	got := suite.fetchResults()

	if golden {
		err := os.WriteFile(goldenFile, got, 0666)
		require.NoError(suite.T(), err)
	} else {
		exp, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		assert.YAMLEq(suite.T(), string(exp), string(got))
	}
}

func (suite *ControllerTestSuite) SetupSuite() {
	flag.Parse()
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	suite.tmpDir = suite.T().TempDir()
	suite.T().Setenv("EXTERNAL_MODULES_DIR", suite.tmpDir)
	_ = os.MkdirAll(filepath.Join(suite.tmpDir, "modules"), 0777)
}

func (suite *ControllerTestSuite) TestCreateReconcile() {
	suite.Run("with no builder endpoints", func() {
		suite.setupController(string(suite.fetchTestFileData("no-builders.yaml")))

		md := suite.getModuleDocumentation("testmodule")
		_, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		require.NoError(suite.T(), err)
	})

	suite.Run("with only one builder", func() {
		_ = os.MkdirAll(filepath.Join(suite.tmpDir, "testmodule", "v1.0.0", "openapi"), 0777)
		_ = os.WriteFile(filepath.Join(suite.tmpDir, "testmodule", "v1.0.0", "openapi", "config-values.yaml"), []byte("{}"), 0666)

		dependency.TestDC.HTTPClient.DoMock.Set(func(req *http.Request) (rp1 *http.Response, err error) {
			switch req.URL.Path {
			case "/loadDocArchive/testmodule/v1.0.0":
				return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(""))}, nil

			case "/build":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			}
			return &http.Response{StatusCode: http.StatusBadRequest}, nil
		})

		suite.setupController(string(suite.fetchTestFileData("one-builder.yaml")))

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("with two builders", func() {
		_ = os.MkdirAll(filepath.Join(suite.tmpDir, "testmodule", "v1.0.0", "openapi"), 0777)
		_ = os.WriteFile(filepath.Join(suite.tmpDir, "testmodule", "v1.0.0", "openapi", "config-values.yaml"), []byte("{}"), 0666)

		dependency.TestDC.HTTPClient.DoMock.Set(func(req *http.Request) (rp1 *http.Response, err error) {
			switch req.URL.Path {
			case "/loadDocArchive/testmodule/v1.0.0":
				return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(""))}, nil

			case "/build":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			}
			return &http.Response{StatusCode: http.StatusBadRequest}, nil
		})

		suite.setupController(string(suite.fetchTestFileData("two-builders.yaml")))

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("one builder cannot render", func() {
		_ = os.MkdirAll(filepath.Join(suite.tmpDir, "testmodule", "v1.0.0", "openapi"), 0777)
		_ = os.WriteFile(filepath.Join(suite.tmpDir, "testmodule", "v1.0.0", "openapi", "config-values.yaml"), []byte("{}"), 0666)

		dependency.TestDC.HTTPClient.DoMock.Set(func(req *http.Request) (rp1 *http.Response, err error) {
			if strings.HasPrefix(req.Host, "10-111-111-11") {
				return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader("I'm broken"))}, nil
			}

			switch req.URL.Path {
			case "/loadDocArchive/testmodule/v1.0.0":
				return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(""))}, nil

			case "/build":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			}
			return &http.Response{StatusCode: http.StatusBadRequest}, nil
		})

		suite.setupController(string(suite.fetchTestFileData("two-builders-partially.yaml")))

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.True(suite.T(), res.Requeue)
		assert.Equal(suite.T(), res.RequeueAfter, 10*time.Second)
		require.NoError(suite.T(), err)
	})

	suite.Run("render new version", func() {
		_ = os.MkdirAll(filepath.Join(suite.tmpDir, "testmodule", "v1.1.1", "openapi"), 0777)
		_ = os.WriteFile(filepath.Join(suite.tmpDir, "testmodule", "v1.1.1", "openapi", "config-values.yaml"), []byte("{}"), 0666)

		dependency.TestDC.HTTPClient.DoMock.Set(func(req *http.Request) (rp1 *http.Response, err error) {
			switch req.URL.Path {
			case "/loadDocArchive/testmodule/v1.1.1":
				return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(""))}, nil

			case "/build":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			}
			return &http.Response{StatusCode: http.StatusBadRequest}, nil
		})

		suite.setupController(string(suite.fetchTestFileData("render-new-version.yaml")))

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("render new lease", func() {
		_ = os.MkdirAll(filepath.Join(suite.tmpDir, "testmodule", "v1.1.1", "openapi"), 0777)
		_ = os.WriteFile(filepath.Join(suite.tmpDir, "testmodule", "v1.1.1", "openapi", "config-values.yaml"), []byte("{}"), 0666)

		dependency.TestDC.HTTPClient.DoMock.Set(func(req *http.Request) (rp1 *http.Response, err error) {
			switch req.URL.Path {
			case "/loadDocArchive/testmodule/v1.1.1":
				return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(""))}, nil

			case "/build":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			}
			return &http.Response{StatusCode: http.StatusBadRequest}, nil
		})

		suite.setupController(string(suite.fetchTestFileData("render-new-lease.yaml")))

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("render new checksum", func() {
		_ = os.MkdirAll(filepath.Join(suite.tmpDir, "testmodule", "dev", "openapi"), 0777)
		_ = os.WriteFile(filepath.Join(suite.tmpDir, "testmodule", "dev", "openapi", "config-values.yaml"), []byte("{}"), 0666)

		dependency.TestDC.HTTPClient.DoMock.Set(func(req *http.Request) (rp1 *http.Response, err error) {
			switch req.URL.Path {
			case "/loadDocArchive/testmodule/mpo-tag":
				return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(""))}, nil

			case "/build":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			}
			return &http.Response{StatusCode: http.StatusBadRequest}, nil
		})

		suite.setupController(string(suite.fetchTestFileData("render-new-checksum.yaml")))

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("keep up-to-date rendered documentation", func() {
		_ = os.MkdirAll(filepath.Join(suite.tmpDir, "testmodule", "v1.1.1", "openapi"), 0777)
		_ = os.WriteFile(filepath.Join(suite.tmpDir, "testmodule", "v1.1.1", "openapi", "config-values.yaml"), []byte("{}"), 0666)
		dependency.TestDC.GetClock().(clockwork.FakeClock).Advance(1 * time.Hour)

		suite.setupController(string(suite.fetchTestFileData("keep-actual.yaml")))

		md := suite.getModuleDocumentation("testmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.False(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})

	suite.Run("with empty dir", func() {
		suite.setupController(string(suite.fetchTestFileData("empty-dir.yaml")))

		md := suite.getModuleDocumentation("absentmodule")
		res, err := suite.ctr.createOrUpdateReconcile(context.TODO(), md)
		assert.True(suite.T(), res.Requeue)
		require.NoError(suite.T(), err)
	})
}

func (suite *ControllerTestSuite) setupController(yamlDoc string) {
	manifests := releaseutil.SplitManifests(yamlDoc)

	var initObjects = make([]client.Object, 0, len(manifests))

	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}

	sc := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(sc)
	_ = corev1.AddToScheme(sc)
	_ = coordv1.AddToScheme(sc)
	cl := fake.NewClientBuilder().WithScheme(sc).WithObjects(initObjects...).WithStatusSubresource(&v1alpha1.ModuleDocumentation{}).Build()
	dc := dependency.NewDependencyContainer()
	rec := &moduleDocumentationReconciler{
		client:             cl,
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
		logger:             log.New(),
		docsBuilder:        docs_builder.NewClient(dc.GetHTTPClient()),
		dc:                 dc,
	}

	suite.ctr = rec
	suite.kubeClient = cl
}

func (suite *ControllerTestSuite) assembleInitObject(obj string) client.Object {
	var res client.Object

	var typ runtime.TypeMeta

	err := yaml.Unmarshal([]byte(obj), &typ)
	require.NoError(suite.T(), err)

	switch typ.Kind {
	case "ModuleDocumentation":
		var ms v1alpha1.ModuleDocumentation
		err = yaml.Unmarshal([]byte(obj), &ms)
		require.NoError(suite.T(), err)
		res = &ms

	case "Lease":
		var ms coordv1.Lease
		err = yaml.Unmarshal([]byte(obj), &ms)
		require.NoError(suite.T(), err)
		res = &ms

	default:
		require.Fail(suite.T(), "unknown Kind:"+typ.Kind)
	}

	return res
}

// nolint:unparam
func (suite *ControllerTestSuite) getModuleDocumentation(name string) *v1alpha1.ModuleDocumentation {
	var md v1alpha1.ModuleDocumentation
	err := suite.kubeClient.Get(context.TODO(), types.NamespacedName{Name: name}, &md)
	require.NoError(suite.T(), err)

	return &md
}

func (suite *ControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	var mdlist v1alpha1.ModuleDocumentationList
	err := suite.kubeClient.List(context.TODO(), &mdlist)
	require.NoError(suite.T(), err)

	for _, item := range mdlist.Items {
		for i, cond := range item.Status.Conditions {
			cond.Message = strings.ReplaceAll(cond.Message, suite.tmpDir, "/testdir")
			item.Status.Conditions[i] = cond
		}
		got, _ := yaml.Marshal(item)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

func (suite *ControllerTestSuite) fetchTestFileData(filename string) []byte {
	dir := "./testdata"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return data
}
