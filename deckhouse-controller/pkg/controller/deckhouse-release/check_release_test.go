/*
Copyright 2024 Flant JSC

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

package deckhouse_release

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"testing"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/updater"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/iancoleman/strcase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func (suite *ControllerTestSuite) TestCheckDeckhouseRelease() {
	ctx := context.Background()

	var initValues = `{
  "global": {
    "modulesImages": {
      "registry": {
        "base": "my.registry.com/deckhouse"
      }
    },
    "discovery": {
      "clusterUUID": "21da7734-77a7-45ad-a795-ea0b629ee930"
    }
  },
  "deckhouse":{
    "bundle": "Default",
    "releaseChannel": "Stable",
    "internal":{
      "releaseVersionImageHash":"zxczxczxc"
    }
  }
}`

	suite.Run("Have new deckhouse image", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.3"}`}}, nil
		},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b777")
			}}, nil)

		suite.setupController("have-new-deckhouse-image.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Have canary release wave 0", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0", "canary": {"stable": {"enabled": true, "waves": 5, "interval": "6m"}}}`}}, nil
		},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
			}}, nil)

		suite.setupController("have-canary-release-wave-0.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Have canary release wave 4", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
			return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.5", "canary": {"stable": {"enabled": true, "waves": 5, "interval": "15m"}}}`}}, nil
		},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b666")
			}}, nil)

		suite.setupController("have-canary-release-wave-4.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Existed release suspended", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0", "suspend": true}`}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
			},
		}, nil)

		suite.setupController("existed-release-suspended.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Deployed release suspended", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0", "suspend": true}`}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
			},
		}, nil)

		suite.setupController("deployed-release-suspended.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("New release suspended", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0", "suspend": true}`}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
			},
		}, nil)

		suite.setupController("new-release-suspended.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Resume suspended release", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0"}`}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
			},
		}, nil)

		suite.setupController("resume-suspended-release.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Image hash not changed", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0"}`}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66857")
			},
		}, nil)

		suite.setupController("image-hash-not-changed.yaml", initValues, embeddedMUP)
		suite.ctr.releaseVersionImageHash.Set("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66857")
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release has requirements", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.30.0", "requirements": {"k8s": "1.19", "req1": "dep1"}}`}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66858")
			},
		}, nil)

		suite.setupController("release-has-requirements.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release has canary", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"canary":{"alpha":{"enabled":true,"interval":"5m","waves":2},"beta":{"enabled":false,"interval":"1m","waves":1},"early-access":{"enabled":true,"interval":"30m","waves":6},"rock-solid":{"enabled":false,"interval":"5m","waves":5},"stable":{"enabled":true,"interval":"30m","waves":6}},"version":"v1.31.0"}`}}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f76859")
			},
		}, nil)

		suite.setupController("release-has-canary.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release has cooldown", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{
			"v1.31.0",
			"v1.31.1",
			"v1.32.0",
			"v1.32.1",
			"v1.32.2",
			"v1.32.3",
			"v1.33.0",
			"v1.33.1",
		}, nil)
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version":"v1.31.0"}`}}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f76859")
			},
			ConfigFileStub: func() (*v1.ConfigFile, error) {
				return &v1.ConfigFile{
					Config: v1.Config{
						Labels: map[string]string{"cooldown": "2026-06-06 16:16:16"},
					},
				}, nil
			},
		}, nil)

		suite.setupController("release-has-cooldown.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Inherit release cooldown", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version":"v1.31.1"}`}}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f76869")
			},
			ConfigFileStub: func() (*v1.ConfigFile, error) {
				return &v1.ConfigFile{
					Config: v1.Config{
						Labels: map[string]string{"cooldown": "2026-06-06 16:16:16"},
					},
				}, nil
			},
		}, nil)

		suite.setupController("inherit-release-cooldown.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Patch release has own cooldown", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version":"v1.31.2"}`}}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f76879")
			},
			ConfigFileStub: func() (*v1.ConfigFile, error) {
				return &v1.ConfigFile{
					Config: v1.Config{
						Labels: map[string]string{"cooldown": "2030-05-05T15:15:15Z"},
					},
				}, nil
			},
		}, nil)

		suite.setupController("patch-release-has-own-cooldown.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release has disruptions", func() {
		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version": "v1.32.0", "disruptions":{"1.32":["ingressNginx"]}}`}}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66859")
			},
		}, nil)

		suite.setupController("release-has-disruptions.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("Release with changelog", func() {
		changelogTemplate := `
cert-manager:
  fixes:
    - summary: Remove D8CertmanagerOrphanSecretsWithoutCorrespondingCertificateResources
      pull_request: https://github.com/deckhouse/deckhouse/pull/999
ci:
  fixes:
    - summary: Fix GitLab CI (.gitlab-ci-simple.yml)
      pull_request: https://github.com/deckhouse/deckhouse/pull/911
global:
  features:
    - description: All master nodes will have %s role in new exist clusters.
      note: Add migration for adding role. Bashible steps will be rerunned on master nodes.
      pull_request: https://github.com/deckhouse/deckhouse/pull/562
    - description: Update Kubernetes patch versions.
      pull_request: https://github.com/deckhouse/deckhouse/pull/558
  fixes:
    - description: Fix parsing deckhouse images repo if there is the sha256 sum in the image name
      pull_request: https://github.com/deckhouse/deckhouse/pull/527
    - description: Fix serialization of empty strings in secrets
      pull_request: https://github.com/deckhouse/deckhouse/pull/523
`

		changelog := fmt.Sprintf(changelogTemplate, "`control-plane`") // global.features[0].description

		dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{
					&fakeLayer{},
					&fakeLayer{FilesContent: map[string]string{
						"version.json":   `{"version": "v1.31.0"}`,
						"changelog.yaml": changelog,
					}},
				}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66858")
			},
		}, nil)

		suite.setupController("release-with-changelog.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})

	suite.Run("StepByStepUpdateFailed", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{
			"v1.31.0",
			"v1.31.1",
			"v1.33.0",
			"v1.33.1",
			"v1.34.0",
		}, nil)
		dependency.TestDC.CRClient.ImageMock.When("stable").Then(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version":"v1.34.0"}`}}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f76879")
			},
		}, nil)
		dependency.TestDC.CRClient.ImageMock.When("v1.32.3").Then(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version":"v1.32.3"}`}}}, nil
			},
		}, nil)

		suite.setupController("step-by-step-update-failed.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.Error(suite.T(), err)
	})

	suite.Run("StepByStepUpdateSuccessfully", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{
			"v1.31.0",
			"v1.31.1",
			"v1.32.0",
			"v1.32.1",
			"v1.32.2",
			"v1.32.3",
			"v1.33.0",
			"v1.33.1",
		}, nil)
		dependency.TestDC.CRClient.ImageMock.When("stable").Then(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version":"v1.33.1"}`}}}, nil
			},
			DigestStub: func() (v1.Hash, error) {
				return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f76879")
			},
		}, nil)
		dependency.TestDC.CRClient.ImageMock.When("v1.32.3").Then(&fake.FakeImage{
			LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version":"v1.32.3"}`}}}, nil
			},
		}, nil)

		suite.setupController("step-by-step-update-successfully.yaml", initValues, embeddedMUP)
		err := suite.ctr.checkDeckhouseRelease(ctx)
		require.NoError(suite.T(), err)
	})
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

func TestSort(t *testing.T) {
	s1 := &v1alpha1.DeckhouseRelease{
		Spec: v1alpha1.DeckhouseReleaseSpec{Version: "v1.29.0"},
	}
	s2 := &v1alpha1.DeckhouseRelease{
		Spec: v1alpha1.DeckhouseReleaseSpec{Version: "v1.29.1"},
	}
	s3 := &v1alpha1.DeckhouseRelease{
		Spec: v1alpha1.DeckhouseReleaseSpec{Version: "v1.29.2"},
	}
	s4 := &v1alpha1.DeckhouseRelease{
		Spec: v1alpha1.DeckhouseReleaseSpec{Version: "v1.29.3"},
	}
	s5 := &v1alpha1.DeckhouseRelease{
		Spec: v1alpha1.DeckhouseReleaseSpec{Version: "v1.29.4"},
	}

	releases := []*v1alpha1.DeckhouseRelease{s3, s4, s1, s5, s2}
	sort.Sort(sort.Reverse(updater.ByVersion[*v1alpha1.DeckhouseRelease](releases)))

	for i, rl := range releases {
		if rl.GetVersion().String() != "1.29."+strconv.FormatInt(int64(4-i), 10) {
			t.Fail()
		}
	}
}

func TestKebabCase(t *testing.T) {
	cases := map[string]string{
		"Alpha":       "alpha",
		"Beta":        "beta",
		"EarlyAccess": "early-access",
		"Stable":      "stable",
		"RockSolid":   "rock-solid",
	}

	for original, kebabed := range cases {
		result := strcase.ToKebab(original)

		assert.Equal(t, result, kebabed)
	}
}
