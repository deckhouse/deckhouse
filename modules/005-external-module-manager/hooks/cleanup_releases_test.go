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
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: external module manager :: hooks :: cleanup::", func() {

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
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Module", false)

	Context("Cluster has releases which should be cleaned up", func() {
		BeforeEach(func() {
			var echoserverState string
			for i := 1; i < 6; i++ {
				echoserverState += "\n" + generateOutdated("echoserver", "v0.0."+strconv.Itoa(i))
			}

			var helloState string
			for i := 1; i < 3; i++ {
				helloState += "\n" + generateOutdated("hellow", "v0.0."+strconv.Itoa(i))
			}

			f.KubeStateSet(echoserverState + `
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: echoserver
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: hellow
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  name: echoserver-v0.0.6
spec:
  moduleName: echoserver
  version: 0.0.6
status:
  phase: Deployed
---
` + helloState + `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  name: hellow-v0.0.3
spec:
  moduleName: hellow
  version: 0.0.3
status:
  phase: Deployed
`)

			f.BindingContexts.Set(f.GenerateScheduleContext("13 3 * * *"))
			f.RunHook()
		})

		It("Should delete outdated releases", func() {
			Expect(f).To(ExecuteSuccessfully())
			rele1 := f.KubernetesGlobalResource("ModuleRelease", "echoserver-v0.0.1")
			Expect(rele1.Exists()).To(BeFalse())
			rele2 := f.KubernetesGlobalResource("ModuleRelease", "echoserver-v0.0.2")
			Expect(rele2.Exists()).To(BeFalse())
			rele3 := f.KubernetesGlobalResource("ModuleRelease", "echoserver-v0.0.3")
			Expect(rele3.Exists()).To(BeTrue())

			hel1 := f.KubernetesGlobalResource("ModuleRelease", "hellow-v0.0.1")
			Expect(hel1.Exists()).To(BeTrue())
		})
	})

	Context("Cluster has releases from absent module", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: testmodule
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  name: testmodule-v0.0.1
spec:
  moduleName: testmodule
  version: 0.0.1
status:
  phase: Deployed
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  name: echoserver-v0.0.6
spec:
  moduleName: echoserver
  version: 0.0.6
status:
  phase: Deployed
`)

			f.BindingContexts.Set(f.GenerateScheduleContext("13 3 * * *"))
			f.RunHook()
		})

		It("Should delete echoserver and testmodule releases, should keep hellow releases", func() {
			Expect(f).To(ExecuteSuccessfully())
			rele1 := f.KubernetesGlobalResource("ModuleRelease", "echoserver-v0.0.6")
			Expect(rele1.Exists()).To(BeFalse())

			test1 := f.KubernetesGlobalResource("ModuleRelease", "testmodule-v0.0.1")
			Expect(test1.Exists()).To(BeTrue())
		})
	})
})

const outdatedTemplate = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  name: %[1]s-%[2]s
spec:
  moduleName: %[1]s
  version: %[2]s
status:
  phase: Superseded
`

func generateOutdated(moduleName, moduleVersion string) string {
	return fmt.Sprintf(outdatedTemplate, moduleName, moduleVersion)
}
