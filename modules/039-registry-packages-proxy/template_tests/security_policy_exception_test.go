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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: registry-packages-proxy :: helm template :: security policy exception", func() {
	f := SetupHelmConfig(``)

	Context("With the SecurityPolicyException kind present in the cluster", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesBootstrapped)
			f.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
			f.HelmRender()
		})

		nsName := "d8-cloud-instance-manager"
		chartName := "registry-packages-proxy"

		It("Pod label must point to an existing exception", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, chartName)
			speName := dp.Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).String()
			Expect(speName).NotTo(BeEmpty())
			Expect(f.KubernetesResource("SecurityPolicyException", nsName, speName).Exists()).To(BeTrue())
		})

		// The exception matches a hostPath only when its `readOnly` equals the one on the mount
		// (`input_hostpath_allowed_exact` in lib.check_path), so an entry written with the wrong
		// value reads as no entry at all. The policy only warns in this namespace, so nothing
		// else would tell us.
		It("Every hostPath volume must be listed in the exception, with the same readOnly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, chartName)

			mountReadOnly := map[string]bool{}
			for _, path := range []string{"spec.template.spec.containers", "spec.template.spec.initContainers"} {
				for _, container := range dp.Field(path).Array() {
					for _, mount := range container.Get("volumeMounts").Array() {
						name := mount.Get("name").String()
						// A volume mounted writable anywhere is writable as far as the policy is
						// concerned, so the writable mount is the one that has to be excepted.
						if ro := mount.Get("readOnly").Bool(); !ro || !mountReadOnly[name] {
							mountReadOnly[name] = ro
						}
					}
				}
			}

			spe := f.KubernetesResource("SecurityPolicyException", nsName, chartName)
			allowed := map[string]bool{}
			for _, entry := range spe.Field("spec.volumes.hostPath.allowedValues").Array() {
				allowed[entry.Get("path").String()] = entry.Get("readOnly").Bool()
			}

			hostPaths := 0
			for _, volume := range dp.Field("spec.template.spec.volumes").Array() {
				hostPath := volume.Get("hostPath.path")
				if !hostPath.Exists() {
					continue
				}
				hostPaths++
				name := volume.Get("name").String()
				Expect(allowed).To(HaveKeyWithValue(hostPath.String(), mountReadOnly[name]),
					"hostPath %s (volume %s) is mounted with readOnly=%v and the exception must say the same",
					hostPath.String(), name, mountReadOnly[name])
			}
			// Otherwise the check above passes on a deployment that stopped mounting anything,
			// and the exception would rot unnoticed.
			Expect(hostPaths).To(BeNumerically(">", 0))
		})
	})
})
