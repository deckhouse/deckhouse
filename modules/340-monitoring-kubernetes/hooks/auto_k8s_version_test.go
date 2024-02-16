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
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("helm :: hooks :: cluster_configuration ::", func() {
	var (
		stateAClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "Automatic"
`
		stateAutomatic = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration))

		stateBClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "1.25"
`
		stateConcreteVersion = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration))
	)

	f := HookExecutionConfigInit("{\"global\": {\"discovery\": {}}}", "{}")
	Context("helm3 release with deprecated versions", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			var sec corev1.Secret
			_ = yaml.Unmarshal([]byte(helm3ReleaseWithDeprecated), &sec)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Secrets("appns").
				Create(context.TODO(), &sec, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))

		})

		Context("check for kubernetesVersion: \"Automatic\"", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateAutomatic))
				f.RunGoHook()

			})

			It("must have autoK8sVersion", func() {
				Expect(f).To(ExecuteSuccessfully())

				var k8sVersion string
				if val, exists := requirements.GetValue(AutoK8sVersion); exists {
					k8sVersion = fmt.Sprintf("%v", val)
				}
				var reasons []string
				if val, exists := requirements.GetValue(AutoK8sReason); exists {
					reasons = strings.Split(fmt.Sprintf("%v", val), ", ")
				}
				Expect(k8sVersion).To(Equal("1.22.0"))
				Expect(reasons).To(HaveLen(2))
				for _, reason := range reasons {
					Expect(`
						networking.k8s.io/v1beta1: Ingress,
						apiextensions.k8s.io/v1beta1: CustomResourceDefinition
					`).To(ContainSubstring(reason))
				}
			})
		})

		Context("check for kubernetesVersion: \"1.25\"", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateConcreteVersion))
				f.RunGoHook()
			})

			It("autoK8sVersion must be empty", func() {
				Expect(f).To(ExecuteSuccessfully())

				var k8sVersion string
				if val, exists := requirements.GetValue(AutoK8sVersion); exists {
					k8sVersion = fmt.Sprintf("%v", val)
				}
				var reasons []string
				if val, exists := requirements.GetValue(AutoK8sReason); exists {
					reasons = strings.Split(fmt.Sprintf("%v", val), ", ")
				}
				Expect(k8sVersion).To(BeEmpty())
				Expect(reasons).To(BeEmpty())
			})
		})

		Context("check for empty \"ClusterConfiguration\"", func() {
			BeforeEach(func() {
				f.RunGoHook()
			})

			It("must return error", func() {
				Expect(f.GoHookError).To(MatchError("cluster configuration kubernetesVersion is empty or invalid"))

				var k8sVersion string
				if val, exists := requirements.GetValue(AutoK8sVersion); exists {
					k8sVersion = fmt.Sprintf("%v", val)
				}
				var reasons []string
				if val, exists := requirements.GetValue(AutoK8sReason); exists {
					reasons = strings.Split(fmt.Sprintf("%v", val), ", ")
				}
				Expect(k8sVersion).To(BeEmpty())
				Expect(reasons).To(BeEmpty())
			})
		})
	})

	Context("helm3 release without deprecated apis", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			var sec corev1.Secret
			_ = yaml.Unmarshal([]byte(helm3ReleaseWithoutDeprecated), &sec)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Secrets("default").
				Create(context.TODO(), &sec, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()
		})

		Context("check for kubernetesVersion: \"Automatic\"", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateAutomatic))
				f.RunGoHook()

			})

			It("autoK8sVersion must be empty", func() {
				Expect(f).To(ExecuteSuccessfully())

				var k8sVersion string
				if val, exists := requirements.GetValue(AutoK8sVersion); exists {
					k8sVersion = fmt.Sprintf("%v", val)
				}
				var reasons []string
				if val, exists := requirements.GetValue(AutoK8sReason); exists {
					reasons = strings.Split(fmt.Sprintf("%v", val), ", ")
				}
				Expect(k8sVersion).To(BeEmpty())
				Expect(reasons).To(BeEmpty())
			})
		})

	})

	Context("helm2 release with deprecated versions", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			var cm corev1.ConfigMap
			_ = yaml.Unmarshal([]byte(helm2ReleaseWithDeprecated), &cm)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				ConfigMaps("default").
				Create(context.TODO(), &cm, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.RunGoHook()

		})

		Context("check for kubernetesVersion: \"Automatic\"", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateAutomatic))
				f.RunGoHook()

			})

			It("must have autoK8sVersion", func() {
				Expect(f).To(ExecuteSuccessfully())

				var k8sVersion string
				if val, exists := requirements.GetValue(AutoK8sVersion); exists {
					k8sVersion = fmt.Sprintf("%v", val)
				}
				var reasons []string
				if val, exists := requirements.GetValue(AutoK8sReason); exists {
					reasons = strings.Split(fmt.Sprintf("%v", val), ", ")
				}
				Expect(k8sVersion).To(Equal("1.22.0"))
				Expect(reasons).To(HaveLen(1))
				for _, reason := range reasons {
					Expect("networking.k8s.io/v1beta1: Ingress").To(ContainSubstring(reason))
				}
			})
		})
	})

	Context("release with doubled fields", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			var sec corev1.Secret
			_ = yaml.Unmarshal([]byte(releaseWithDoubleFields), &sec)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Secrets("default").
				Create(context.TODO(), &sec, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))
			f.BindingContexts.Set(f.KubeStateSet(stateAutomatic))
			f.RunGoHook()

		})

		It("must be valid and have no deprecated resources", func() {
			Expect(f).To(ExecuteSuccessfully())

			var k8sVersion string
			if val, exists := requirements.GetValue(AutoK8sVersion); exists {
				k8sVersion = fmt.Sprintf("%v", val)
			}
			var reasons []string
			if val, exists := requirements.GetValue(AutoK8sReason); exists {
				reasons = strings.Split(fmt.Sprintf("%v", val), ", ")
			}
			Expect(k8sVersion).To(Equal("1.22.0"))
			Expect(reasons).To(HaveLen(1))
			for _, reason := range reasons {
				Expect("networking.k8s.io/v1beta1: Ingress").To(ContainSubstring(reason))
			}
		})
	})
})
