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

package hooks

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const legacy = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    cm-migrated: ""
  name: descheduler-config-migration
  namespace: d8-system
data:
  config: |
    {
    "removePodsViolatingInterPodAntiAffinity": true,
    "removePodsViolatingNodeAffinity": true
    }
`

var _ = Describe("Descheduler migration :: delete old descheduler cm", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Legacy CM exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		var cm corev1.ConfigMap
		_ = yaml.Unmarshal([]byte(legacy), &cm)
		_, _ = dependency.TestDC.MustGetK8sClient().CoreV1().ConfigMaps("d8-system").Create(context.TODO(), &cm, metav1.CreateOptions{})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-system", "descheduler-config-migration").Exists()).To(BeFalse())
		})
	})
})
