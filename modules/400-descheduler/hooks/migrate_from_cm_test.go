/*
Copyright 2022 Flant JSC

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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	emptyConfigMap = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: descheduler-config-migration
  namespace: d8-system
data:
  "config": "{}"
`
	configMap = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: descheduler-config-migration
  namespace: d8-system
data:
  "config": '{"removePodsViolatingTopologySpreadConstraint": true}'
`
)

var _ = Describe("Modules :: descheduler :: hooks :: migrate_from_cm ::", func() {
	f := HookExecutionConfigInit(`{"descheduler":{"internal":{}}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Descheduler", false)

	Context("Cluster with enabled, but unconfigured descheduler", func() {
		BeforeEach(func() {
			cm := &corev1.ConfigMap{}
			Expect(yaml.Unmarshal([]byte(emptyConfigMap), &cm)).To(Succeed())

			f.KubeStateSet("")
			_, err := f.KubeClient().CoreV1().ConfigMaps("d8-system").Create(context.TODO(), cm, v1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			f.RunHook()
		})

		It("Should create the default Descheduler CR", func() {
			Expect(f).To(ExecuteSuccessfully())

			legacyCR := f.KubernetesGlobalResource("Descheduler", "legacy")
			Expect(legacyCR.ToYaml()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: legacy
spec:
  deschedulerPolicy:
    strategies:
      removePodsViolatingInterPodAntiAffinity:
        enabled: true
      removePodsViolatingNodeAffinity:
        enabled: true
`))
		})
	})

	Context("Cluster with configured descheduler", func() {
		BeforeEach(func() {
			cm := &corev1.ConfigMap{}
			Expect(yaml.Unmarshal([]byte(configMap), &cm)).To(Succeed())

			f.KubeStateSet("")
			_, err := f.KubeClient().CoreV1().ConfigMaps("d8-system").Create(context.TODO(), cm, v1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			f.RunHook()
		})

		It("Should create the default Descheduler CR", func() {
			Expect(f).To(ExecuteSuccessfully())

			legacyCR := f.KubernetesGlobalResource("Descheduler", "legacy")
			Expect(legacyCR.ToYaml()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: legacy
spec:
  deschedulerPolicy:
    strategies:
      removePodsViolatingInterPodAntiAffinity:
        enabled: true
      removePodsViolatingNodeAffinity:
        enabled: true
      removePodsViolatingTopologySpreadConstraint:
        enabled: true
`))
		})
	})

	Context("Cluster with no descheduler CM", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.RunHook()
		})

		It("Should create the default Descheduler CR", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesGlobalResource("Descheduler", "legacy").Exists()).To(BeFalse())
		})
	})
})
