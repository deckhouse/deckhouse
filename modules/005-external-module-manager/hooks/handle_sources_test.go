/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/fake"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/modules/005-external-module-manager/hooks/internal/apis/v1alpha1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: external module manager :: hooks :: handle sources ::", func() {
	var tmpDir string

	f := HookExecutionConfigInit(`
global:
  deckhouseVersion: "12345"
  modulesImages:
    registry:
      base: registry.deckhouse.io/deckhouse/fe
external-module-manager:
  internal: {}
`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleRelease", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleSource", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleUpdatePolicy", false)

	dependency.TestDC.CRClient = cr.NewClientMock(GinkgoT())
	Context("Cluster with module source and update policy matching 1 module", func() {
		BeforeEach(func() {
			tmpDir, _ = os.MkdirTemp(os.TempDir(), "exrelease-*")
			_ = os.Mkdir(tmpDir+"/modules", 0777)
			_ = os.Setenv("EXTERNAL_MODULES_DIR", tmpDir)
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.3"}`}}, nil
			},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b777")
				}}, nil)
			dependency.TestDC.CRClient.ListTagsMock.Return([]string{
				"echo",
			}, nil)
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: echoserver
spec:
  registry:
    dockerCfg: config
    repo: dev-registry.deckhouse.io/deckhouse/losev/external-modules
    scheme: HTTPS
  releaseChannel: alpha
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleUpdatePolicy
metadata:
  name: echoserver-policy
spec:
  update:
    mode: Manual
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        source: echoserver
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Module source status.modulesCount should be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ModuleSource", "echoserver").Field("status.message").String()).To(Not(Equal("")))
		})
	})

	Context("Cluster with module source and update policy not matching any modules", func() {
		BeforeEach(func() {
			tmpDir, _ = os.MkdirTemp(os.TempDir(), "exrelease-*")
			_ = os.Mkdir(tmpDir+"/modules", 0777)
			_ = os.Setenv("EXTERNAL_MODULES_DIR", tmpDir)
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.3"}`}}, nil
			},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b777")
				}}, nil)
			dependency.TestDC.CRClient.ListTagsMock.Return([]string{
				"echo",
			}, nil)
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: echoserver
spec:
  registry:
    dockerCfg: config
    repo: dev-registry.deckhouse.io/deckhouse/losev/external-modules
    scheme: HTTPS
  releaseChannel: alpha
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleUpdatePolicy
metadata:
  name: echoserver-policy
spec:
  update:
    mode: Manual
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        source: notechoserver
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Module source status.message should have no errors", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ModuleSource", "echoserver").Field("status.message").String()).To(Equal(""))
		})
	})
})

func TestOpenapiInjection(t *testing.T) {
	source := `
x-extend:
  schema: config-values.yaml
type: object
properties:
  internal:
    type: object
    default: {}
    properties:
      pythonVersions:
        type: array
        default: []
        items:
          type: string
  registry:
    type: object
    description: "System field, overwritten by Deckhouse. Don't use"
`

	sourceModule := v1alpha1.ModuleSource{}
	sourceModule.Spec.Registry.Repo = "test.deckhouse.io/foo/bar"
	sourceModule.Spec.Registry.DockerCFG = "dGVzdG1lCg=="

	data, err := mutateOpenapiSchema([]byte(source), sourceModule)
	require.NoError(t, err)

	assert.YAMLEq(t, `
type: object
x-extend:
  schema: config-values.yaml
properties:
  registry:
    type: object
    default: {}
    properties:
      base:
        type: string
        default: test.deckhouse.io/foo/bar
      dockercfg:
        type: string
        default: dGVzdG1lCg==
  internal:
    default: {}
    properties:
      pythonVersions:
        default: []
        items:
          type: string
        type: array
    type: object
`, string(data))
}

type fakeLayer struct {
	v1.Layer
	// Deprecated: use FilesContent with specified name instead
	Body string

	FilesContent map[string]string // pair: filename - file content
}

func (fl fakeLayer) Uncompressed() (io.ReadCloser, error) {
	result := bytes.NewBuffer(nil)
	if fl.FilesContent == nil {
		fl.FilesContent = make(map[string]string)
	}

	if fl.Body != "" && len(fl.FilesContent) == 0 {
		// backward compatibility for tests
		fl.FilesContent["version.json"] = fl.Body
	}

	if len(fl.FilesContent) == 0 {
		return io.NopCloser(result), nil
	}

	wr := tar.NewWriter(result)

	// create files in a single layer
	for filename, content := range fl.FilesContent {
		hdr := &tar.Header{
			Name: filename,
			Mode: 0600,
			Size: int64(len(content)),
		}
		_ = wr.WriteHeader(hdr)
		_, _ = wr.Write([]byte(content))
	}
	_ = wr.Close()

	return io.NopCloser(result), nil
}

func (fl fakeLayer) Size() (int64, error) {
	if len(fl.Body) > 0 {
		return int64(len(fl.Body)), nil
	}

	return int64(len(fl.FilesContent)), nil
}
