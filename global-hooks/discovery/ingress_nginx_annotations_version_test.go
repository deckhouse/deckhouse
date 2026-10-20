/*
Copyright 2026 Flant JSC

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

/*

User-stories: Hook must save the generation of the additional annotations the ingress-nginx controllers support as
.Values.global.discovery.ingressNginxAnnotationsVersion: the minimum of the version in the
additional-annotations-version ConfigMap of the module and the versions of all controller and validator pods that have
not finished; without pods, the ConfigMap alone decides it. If the ConfigMap has no version or any version is not a
semver, the version is an empty string.

*/

package hooks

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// noVersion leaves the version out of the ConfigMap or the label out of the pod.
const noVersion = "<none>"

func ingressNginxConfigMap(namespace, name, version string) string {
	data := ""
	if version != noVersion {
		data = fmt.Sprintf("data:\n  version: %q\n", version)
	}
	return fmt.Sprintf(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
%s`, name, namespace, data)
}

func ingressNginxAnnotationsVersion(version string) string {
	return ingressNginxConfigMap(ingressNginxNamespace, ingressNginxAnnotationsVersionConfigMap, version)
}

func ingressNginxPodIn(namespace, name, app, version, phase string) string {
	labels := fmt.Sprintf("    app: %s\n", app)
	if version != noVersion {
		labels += fmt.Sprintf("    %s: %q\n", ingressNginxAnnotationsVersionLabel, version)
	}
	return fmt.Sprintf(`
---
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
%sstatus:
  phase: %s
`, name, namespace, labels, phase)
}

func ingressNginxPod(name, app, version, phase string) string {
	return ingressNginxPodIn(ingressNginxNamespace, name, app, version, phase)
}

// ingressNginxPods are the pods of a controller with a failover and a validator, all of version.
func ingressNginxPods(version string) []string {
	return []string{
		ingressNginxPod("controller-main-a", "controller", version, "Running"),
		ingressNginxPod("controller-main-failover-a", "controller", version, "Running"),
		ingressNginxPod("validator-main-a", "validator", version, "Running"),
	}
}

func ingressNginxModule(version string, pods ...string) []string {
	return append([]string{ingressNginxAnnotationsVersion(version)}, pods...)
}

func expectIngressNginxVersion(f *HookExecutionConfig, expected string) {
	value := f.ValuesGet(ingressNginxAnnotationsVersionValuesPath)
	Expect(value.Exists()).To(BeTrue(), "the discovery value must always be set")
	Expect(value.String()).To(Equal(expected))
}

var _ = Describe("Global hooks :: discovery :: ingress_nginx_annotations_version ::", func() {
	f := HookExecutionConfigInit(`{"global":{"discovery":{}}}`, `{}`)

	run := func(manifests ...string) {
		f.BindingContexts.Set(f.KubeStateSet(strings.Join(manifests, "")))
		f.RunHook()
		Expect(f).To(ExecuteSuccessfully())
	}

	DescribeTable("Saves the minimum version of the ConfigMap and the pods",
		func(manifests []string, expected string) {
			run(manifests...)
			expectIngressNginxVersion(f, expected)
		},
		Entry("nothing is deployed", []string{}, ""),
		Entry("every pod has the version", ingressNginxModule("v1.3.0", ingressNginxPods("v1.3.0")...), "v1.3.0"),
		Entry("no controllers, so no pods", ingressNginxModule("v1.3.0"), "v1.3.0"),
		Entry("pods without the ConfigMap", ingressNginxPods("v1.3.0"), ""),
		Entry("a ConfigMap without the version", ingressNginxModule(noVersion, ingressNginxPods(noVersion)...), ""),
		Entry("a ConfigMap with an empty version", ingressNginxModule("", ingressNginxPods("")...), ""),
		Entry("a pod without the label", ingressNginxModule("v1.3.0", append(ingressNginxPods("v1.3.0"),
			ingressNginxPod("controller-main-b", "controller", noVersion, "Running"))...), ""),
		Entry("a pod of another version, during a rollout", ingressNginxModule("v1.3.0", append(ingressNginxPods("v1.3.0"),
			ingressNginxPod("validator-main-b", "validator", "v1.2.0", "Running"))...), "v1.2.0"),
		Entry("a pending pod of another version", ingressNginxModule("v1.3.0", append(ingressNginxPods("v1.3.0"),
			ingressNginxPod("controller-main-b", "controller", "v1.2.0", "Pending"))...), "v1.2.0"),
		Entry("a pod newer than the ConfigMap", ingressNginxModule("v1.3.0", ingressNginxPods("v1.4.0")...), "v1.3.0"),
		Entry("a pod with a version that is not a semver", ingressNginxModule("v1.3.0", append(ingressNginxPods("v1.3.0"),
			ingressNginxPod("controller-main-b", "controller", "broken", "Running"))...), ""),
		Entry("a ConfigMap with a version that is not a semver", ingressNginxModule("broken", ingressNginxPods("v1.3.0")...), ""),
		Entry("terminated pods of another version are ignored", ingressNginxModule("v1.3.0", append(ingressNginxPods("v1.3.0"),
			ingressNginxPod("controller-main-old", "controller", "v1.2.0", "Failed"),
			ingressNginxPod("validator-main-old", "validator", noVersion, "Succeeded"))...), "v1.3.0"),
		Entry("other pods are ignored", ingressNginxModule("v1.3.0", append(ingressNginxPods("v1.3.0"),
			ingressNginxPod("controller-manager-a", "controller-manager", noVersion, "Running"),
			ingressNginxPod("proxy-main-failover-a", "proxy-failover", "v1.2.0", "Running"),
			ingressNginxPodIn("default", "controller-elsewhere", "controller", "v1.2.0", "Running"))...), "v1.3.0"),
		Entry("a ConfigMap in another namespace is ignored", []string{
			ingressNginxConfigMap("default", ingressNginxAnnotationsVersionConfigMap, "v1.3.0"),
		}, ""),
	)

	Context("A new generation of annotations is rolled out", func() {
		It("Saves the previous version until all pods are rolled out", func() {
			run(ingressNginxModule("v1.3.0", ingressNginxPods("v1.3.0")...)...)
			expectIngressNginxVersion(f, "v1.3.0")

			run(ingressNginxModule("v1.4.0", ingressNginxPods("v1.3.0")...)...)
			expectIngressNginxVersion(f, "v1.3.0")

			run(ingressNginxModule("v1.4.0", append(ingressNginxPods("v1.3.0")[:2],
				ingressNginxPod("validator-main-b", "validator", "v1.4.0", "Running"))...)...)
			expectIngressNginxVersion(f, "v1.3.0")

			run(ingressNginxModule("v1.4.0", ingressNginxPods("v1.4.0")...)...)
			expectIngressNginxVersion(f, "v1.4.0")
		})
	})
})
