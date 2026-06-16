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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func genSolverPodManifest(name, namespace, phase string, withDeletionTimestamp bool) string {
	deletionTimestamp := ""
	if withDeletionTimestamp {
		deletionTimestamp = `  deletionTimestamp: "2026-01-01T00:00:00Z"`
	}

	return fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    acme.cert-manager.io/http01-solver: "true"
%s
status:
  phase: %s
`, name, namespace, deletionTimestamp, phase)
}

func setPodsState(f *HookExecutionConfig, manifests ...string) {
	state := strings.Join(manifests, "\n---")
	f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(state, 0))
}

var _ = Describe("Cert Manager hooks :: cleanup stale http01 solver pods ::", func() {
	f := HookExecutionConfigInit(`{"global":{}}`, "")

	const ns = "cm-repro"

	Context("Empty cluster", func() {
		BeforeEach(func() {
			setPodsState(f, ``)
			f.RunHook()
		})

		It("runs successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Terminal solver pod", func() {
		BeforeEach(func() {
			setPodsState(f, genSolverPodManifest("solver-succeeded", ns, "Succeeded", false))
			f.RunHook()
		})

		It("deletes the pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-succeeded")).To(BeEmpty())
		})
	})

	Context("Running solver pod", func() {
		BeforeEach(func() {
			setPodsState(f, genSolverPodManifest("solver-running", ns, "Running", false))
			f.RunHook()
		})

		It("keeps the pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-running")).ToNot(BeEmpty())
		})
	})

	Context("Solver pod marked for deletion", func() {
		BeforeEach(func() {
			setPodsState(f, genSolverPodManifest("solver-terminating", ns, "Succeeded", true))
			f.RunHook()
		})

		It("does not attempt to delete the pod again", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-terminating")).ToNot(BeEmpty())
		})
	})
})
