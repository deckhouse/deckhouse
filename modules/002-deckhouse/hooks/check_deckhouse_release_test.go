/*
Copyright 2021 Flant JSC

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
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/iancoleman/strcase"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: check deckhouse release ::", func() {
	f := HookExecutionConfigInit(`{
"global": {
  "discovery": {
    "clusterUUID": "21da7734-77a7-45ad-a795-ea0b629ee930"
  }
},
"deckhouse":{
  "releaseChannel": "Stable",
  "internal":{
	"releaseVersionImageHash":"zxczxczxc"}
  }
}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "DeckhouseRelease", false)

	dependency.TestDC.CRClient = cr.NewClientMock(GinkgoT())
	Context("Have new deckhouse image", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.3"}`}}, nil
			},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b777")
				}}, nil)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-3").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-3").Field("spec.version").String()).To(BeEquivalentTo("v1.25.3"))
		})
	})

	Context("Have canary release", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0", "canary": {"stable": {"enabled": true, "waves": 5, "interval": "6m"}}}`}}, nil
			},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
				}}, nil)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be created without ApplyAfter (wave 0)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Field("spec.version").String()).To(BeEquivalentTo("v1.25.0"))
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Field("spec.applyAfter").Exists()).To(BeFalse())
		})
	})

	Context("Have canary release", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{LayersStub: func() ([]v1.Layer, error) {
				return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.5", "canary": {"stable": {"enabled": true, "waves": 5, "interval": "15m"}}}`}}, nil
			},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b666")
				}}, nil)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be created with ApplyAfter (wave 4)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-5").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-5").Field("spec.applyAfter").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-5").Field("spec.applyAfter").Time()).To(BeTemporally("~", time.Now().UTC().Add(60*time.Minute), time.Minute))
		})
	})

	Context("Existed release suspended", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0", "suspend": true}`}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
				},
			}, nil)
			f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1-25-0
spec:
  version: "v1.25.0"
status:
  phase: Pending
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be marked with annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Field("metadata.annotations.release\\.deckhouse\\.io/suspended").String()).To(Equal("true"))
		})
	})

	Context("Deployed release suspended", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0", "suspend": true}`}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
				},
			}, nil)
			f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1-25-0
spec:
  version: "v1.25.0"
status:
  phase: Deployed
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should not be marked with annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Field("metadata.annotations.release\\.deckhouse\\.io/suspended").Exists()).To(BeFalse())
		})
	})

	Context("New release suspended", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0", "suspend": true}`}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
				},
			}, nil)
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be marked with annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Field("metadata.annotations.release\\.deckhouse\\.io/suspended").String()).To(Equal("true"))
		})
	})

	Context("Resume suspended release", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0"}`}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
				},
			}, nil)
			f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  name: v1-25-0
  annotations:
    release.deckhouse.io/suspended: "true"
spec:
  version: "v1.25.0"
status:
  phase: Suspended
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be marked with annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Field("metadata.annotations.release\\.deckhouse\\.io/suspended").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Field("metadata.annotations.release\\.deckhouse\\.io/suspended").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Field("status.phase").String()).To(Equal("Pending"))
		})
	})

	Context("Image hash not changed", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.25.0"}`}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66857")
				},
			}, nil)
			f.ValuesSet("deckhouse.internal.releaseVersionImageHash", "sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66857")
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should not be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-25-0").Exists()).To(BeFalse())
		})
	})

	Context("Release has requirements", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&fakeLayer{}, &fakeLayer{Body: `{"version": "v1.30.0", "requirements": {"k8s": "1.19", "req1": "dep1"}}`}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66858")
				},
			}, nil)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be created with requirements", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-30-0").Exists()).To(BeTrue())
			rl := f.KubernetesGlobalResource("DeckhouseRelease", "v1-30-0")
			Expect(rl.Field("spec.requirements").String()).To(Equal(`{"k8s":"1.19","req1":"dep1"}`))
		})
	})

	Context("Release has canary", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"canary":{"alpha":{"enabled":true,"interval":"5m","waves":2},"beta":{"enabled":false,"interval":"1m","waves":1},"early-access":{"enabled":true,"interval":"30m","waves":6},"rock-solid":{"enabled":false,"interval":"5m","waves":5},"stable":{"enabled":true,"interval":"30m","waves":6}},"version":"v1.31.0"}`}}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f76859")
				},
			}, nil)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be created with requirements", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-0").Exists()).To(BeTrue())
			rl := f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-0")
			Expect(rl.Field("spec.applyAfter").Exists()).To(BeTrue())
		})
	})

	Context("Release has cooldown", func() {
		BeforeEach(func() {
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
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be created with cooldown", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-0").Exists()).To(BeTrue())
			rl := f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-0")
			Expect(rl.Field(`metadata.annotations.release\.deckhouse\.io/cooldown`).Exists()).To(BeTrue())
			Expect(rl.Field(`metadata.annotations.release\.deckhouse\.io/cooldown`).String()).To(Equal("2026-06-06T16:16:16Z"))
		})

		Context("Inherit release cooldown", func() {
			BeforeEach(func() {
				dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
					LayersStub: func() ([]v1.Layer, error) {
						return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version":"v1.31.1"}`}}}, nil
					},
					DigestStub: func() (v1.Hash, error) {
						return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f76869")
					},
				}, nil)
				f.KubeStateSet("")
				f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
				f.RunHook()
			})
			It("Release should inherit cooldown from previous one", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-1").Exists()).To(BeTrue())
				rl := f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-1")
				Expect(rl.Field(`metadata.annotations.release\.deckhouse\.io/cooldown`).Exists()).To(BeTrue())
				Expect(rl.Field(`metadata.annotations.release\.deckhouse\.io/cooldown`).String()).To(Equal("2026-06-06T16:16:16Z"))
			})
		})

		Context("Patch release has own cooldown", func() {
			BeforeEach(func() {
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
				f.KubeStateSet("")
				f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
				f.RunHook()
			})
			It("Release should not inherit cooldown from previous one", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-2").Exists()).To(BeTrue())
				rl := f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-2")
				Expect(rl.Field(`metadata.annotations.release\.deckhouse\.io/cooldown`).Exists()).To(BeTrue())
				Expect(rl.Field(`metadata.annotations.release\.deckhouse\.io/cooldown`).String()).To(Equal("2030-05-05T15:15:15Z"))
			})
		})
	})

	Context("Release has disruptions", func() {
		BeforeEach(func() {
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": `{"version": "v1.32.0", "disruptions":{"1.32":["ingressNginx"]}}`}}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66859")
				},
			}, nil)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be created with requirements", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-32-0").Exists()).To(BeTrue())
			rl := f.KubernetesGlobalResource("DeckhouseRelease", "v1-32-0")
			Expect(rl.Field("spec.disruptions").Array()).To(HaveLen(1))
			Expect(rl.Field("spec.disruptions").Array()[0].String()).To(Equal("ingressNginx"))
		})
	})

	Context("Release with changelog", func() {
		BeforeEach(func() {
			changelog := `
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
    - description: All master nodes will have  role in new exist clusters.
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
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{
						&fakeLayer{},
						&fakeLayer{FilesContent: map[string]string{
							"version.json":   `{"version": "v1.31.0"}`,
							"changelog.yaml": changelog,
						},
						},
					}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f66858")
				},
			}, nil)
			f.ValuesSet("global.enabledModules", []string{"cert-manager", "prometheus"})
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Release should be created with requirements", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-0").Exists()).To(BeTrue())
			rl := f.KubernetesGlobalResource("DeckhouseRelease", "v1-31-0")
			// global changelog is added
			globalChangelog := rl.Field("spec.changelog.global")
			Expect(globalChangelog.Exists()).To(BeTrue())
			// cert-manager module is enabled and has changes
			certManagerChangelog := rl.Field("spec.changelog.cert-manager")
			Expect(certManagerChangelog.Exists()).To(BeTrue())
			// prometheus is enabled but doesn't have changes
			prometheusChangelog := rl.Field("spec.changelog.prometheus")
			Expect(prometheusChangelog.Exists()).To(BeFalse())
			// ci module has changes but not enabled
			ciChangelog := rl.Field("spec.changelog.ci")
			Expect(ciChangelog.Exists()).To(BeFalse())

			link := rl.Field("spec.changelogLink")
			Expect(link.String()).To(BeEquivalentTo("https://github.com/deckhouse/deckhouse/releases/tag/v1.31.0"))
		})
	})

	// manual release creation, for testing in a cluster
	XContext("Generate release", func() {
		const releaseJson = `
			{"canary":{"alpha":{"enabled":true,"interval":"5m","waves":2},"beta":{"enabled":false,"interval":"1m","waves":1},"early-access":{"enabled":true,"interval":"30m","waves":6},"rock-solid":{"enabled":false,"interval":"5m","waves":5},"stable":{"enabled":true,"interval":"30m","waves":6}},"disruptions":{"1.36":["ingressNginx"]},"requirements":{"ingressNginx":"0.33","k8s":"1.19.0"},"version":"v1.666.0"}
	`
		BeforeEach(func() {
			f.ValuesSet("deckhouse.update.notification.webhook", "https://webhook.site/8e3039b8-216e-4b5b-bb9c-68bb352f1be3")
			dependency.TestDC.CRClient.ImageMock.Return(&fake.FakeImage{
				LayersStub: func() ([]v1.Layer, error) {
					return []v1.Layer{&fakeLayer{}, &fakeLayer{FilesContent: map[string]string{"version.json": releaseJson}}}, nil
				},
				DigestStub: func() (v1.Hash, error) {
					return v1.NewHash("sha256:e1752280e1115ac71ca734ed769f9a1af979aaee4013cdafb62d0f9090f63859")
				},
			}, nil)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
			f.RunHook()
		})
		It("Manual release creation", func() {
			Expect(f).To(ExecuteSuccessfully())
			rl := f.KubernetesGlobalResource("DeckhouseRelease", "v1-666-0")
			fmt.Println("\n" + rl.ToYaml())
			time.Sleep(300 * time.Millisecond)
		})
	})

})

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
		return ioutil.NopCloser(result), nil
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

	return ioutil.NopCloser(result), nil
}

func (fl fakeLayer) Size() (int64, error) {
	if len(fl.Body) > 0 {
		return int64(len(fl.Body)), nil
	}

	return int64(len(fl.FilesContent)), nil
}

func TestSort(t *testing.T) {
	s1 := deckhouseRelease{
		Version: semver.MustParse("v1.24.0"),
	}
	s2 := deckhouseRelease{
		Version: semver.MustParse("v1.24.1"),
	}
	s3 := deckhouseRelease{
		Version: semver.MustParse("v1.24.2"),
	}
	s4 := deckhouseRelease{
		Version: semver.MustParse("v1.24.3"),
	}
	s5 := deckhouseRelease{
		Version: semver.MustParse("v1.24.4"),
	}

	releases := []deckhouseRelease{s3, s4, s1, s5, s2}
	sort.Sort(sort.Reverse(byVersion(releases)))

	for i, rl := range releases {
		if rl.Version.String() != "1.24."+strconv.FormatInt(int64(4-i), 10) {
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
