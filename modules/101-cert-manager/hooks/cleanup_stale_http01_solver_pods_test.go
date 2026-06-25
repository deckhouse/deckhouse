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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func genSolverPodManifest(name, namespace, phase string, withDeletionTimestamp bool, createdAt, finishedAt time.Time) string {
	deletionTimestamp := ""
	if withDeletionTimestamp {
		deletionTimestamp = `  deletionTimestamp: "2026-01-01T00:00:00Z"`
	}

	containerStatuses := ""
	if !finishedAt.IsZero() {
		containerStatuses = fmt.Sprintf(`  containerStatuses:
  - name: acmesolver
    state:
      terminated:
        finishedAt: "%s"`, finishedAt.UTC().Format(time.RFC3339))
	}

	creationTimestamp := createdAt.UTC().Format(time.RFC3339)
	if createdAt.IsZero() {
		creationTimestamp = time.Now().UTC().Format(time.RFC3339)
	}

	return fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  creationTimestamp: "%s"
  labels:
    acme.cert-manager.io/http01-solver: "true"
%s
status:
  phase: %s
%s
`, name, namespace, creationTimestamp, deletionTimestamp, phase, containerStatuses)
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

	Context("Terminal solver pod past grace period", func() {
		BeforeEach(func() {
			terminalAt := time.Now().Add(-2 * time.Minute)
			setPodsState(f, genSolverPodManifest("solver-succeeded", ns, "Succeeded", false, terminalAt, terminalAt))
			f.RunHook()
		})

		It("deletes the pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-succeeded")).To(BeEmpty())
		})
	})

	Context("Failed solver pod past grace period", func() {
		BeforeEach(func() {
			terminalAt := time.Now().Add(-2 * time.Minute)
			setPodsState(f, genSolverPodManifest("solver-failed", ns, "Failed", false, terminalAt, terminalAt))
			f.RunHook()
		})

		It("deletes the pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-failed")).To(BeEmpty())
		})
	})

	Context("Unknown solver pod past grace period", func() {
		BeforeEach(func() {
			terminalAt := time.Now().Add(-2 * time.Minute)
			setPodsState(f, genSolverPodManifest("solver-unknown", ns, "Unknown", false, terminalAt, terminalAt))
			f.RunHook()
		})

		It("deletes the pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-unknown")).To(BeEmpty())
		})
	})

	Context("Terminal solver pod past grace period without finishedAt", func() {
		BeforeEach(func() {
			createdAt := time.Now().Add(-2 * time.Minute)
			setPodsState(f, genSolverPodManifest("solver-fallback", ns, "Succeeded", false, createdAt, time.Time{}))
			f.RunHook()
		})

		It("deletes the pod using creationTimestamp fallback", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-fallback")).To(BeEmpty())
		})
	})

	Context("Terminal solver pod within grace period", func() {
		BeforeEach(func() {
			terminalAt := time.Now().Add(-10 * time.Second)
			setPodsState(f, genSolverPodManifest("solver-fresh", ns, "Succeeded", false, terminalAt, terminalAt))
			f.RunHook()
		})

		It("keeps the pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-fresh")).ToNot(BeEmpty())
		})
	})

	Context("Running solver pod", func() {
		BeforeEach(func() {
			setPodsState(f, genSolverPodManifest("solver-running", ns, "Running", false, time.Time{}, time.Time{}))
			f.RunHook()
		})

		It("keeps the pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-running")).ToNot(BeEmpty())
		})
	})

	Context("Solver pod marked for deletion", func() {
		BeforeEach(func() {
			terminalAt := time.Now().Add(-2 * time.Minute)
			setPodsState(f, genSolverPodManifest("solver-terminating", ns, "Succeeded", true, terminalAt, terminalAt))
			f.RunHook()
		})

		It("does not attempt to delete the pod again", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", ns, "solver-terminating")).ToNot(BeEmpty())
		})
	})
})
