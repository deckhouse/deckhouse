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
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: external module manager :: hooks :: apply release ::", func() {
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
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ExternalModuleRelease", false)

	Context("Cluster has pending ExternalModuleRelease", func() {
		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp(os.TempDir(), "exrelease-*")
			if err != nil {
				Fail(err.Error())
			}
			_ = os.Mkdir(tmpDir+"/modules", 0777)
			_ = os.Setenv("EXTERNAL_MODULES_DIR", tmpDir)

			st := f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ExternalModuleRelease
metadata:
  name: echoserver-v0.0.1
spec:
  moduleName: echoserver
  version: 0.0.1
status:
  phase: Pending
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		AfterEach(func() {
			_ = os.RemoveAll(tmpDir)
		})

		It("module symlink should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ExternalModuleRelease", "echoserver-v0.0.1").Field("status.phase").String()).To(Equal("Deployed"))
			moduleLinks, err := os.ReadDir(tmpDir + "/modules")
			if err != nil {
				Fail(err.Error())
			}
			Expect(moduleLinks).To(HaveLen(1))
			Expect(moduleLinks[0].Name()).To(Equal("900-echoserver"))
		})

		Context("ExternalModuleRelease was deleted", func() {
			BeforeEach(func() {
				st := f.KubeStateSet(``)
				f.BindingContexts.Set(st)
				fsSynchronized = false
				f.RunHook()
			})

			It("should delete module from FS", func() {
				Expect(f).To(ExecuteSuccessfully())
				moduleLinks, err := os.ReadDir(tmpDir + "/modules")
				if err != nil {
					Fail(err.Error())
				}
				Expect(moduleLinks).To(HaveLen(0))
			})
		})
	})
})
