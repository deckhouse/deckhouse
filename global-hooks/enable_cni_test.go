// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: enable_cni ::", func() {
	cniConfig := func(name string) string {
		s := &v1core.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},

			ObjectMeta: metav1.ObjectMeta{
				Name:      "d8-cluster-configuration",
				Namespace: "kube-system",
			},

			Data: map[string][]byte{
				"cni": []byte(name),
				name:  []byte(`{"data": "some"}`),
			},
		}

		j, err := json.Marshal(s)
		if err != nil {
			panic(err)
		}

		c, err := yaml.JSONToYAML(j)
		if err != nil {
			panic(err)
		}
		return string(c)
	}

	const invalidCni = "invalid"

	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	Context("Cluster has not d8-cni-configuration secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has d8-cni-configuration secret", func() {
		Context("With valid cni name", func() {
			for cniName, module := range cniNameToModule {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(cniConfig(cniName)))
					f.RunHook()
				})

				It("Enables cni module "+module, func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet(module).Exists()).To(BeFalse())
				})

				It("Disables another cni modules", func() {
					Expect(f).To(ExecuteSuccessfully())
					for cniNameTOCompare, module := range cniNameToModule {
						if cniNameTOCompare == cniName {
							continue
						}
						Expect(f.ValuesGet(module).Exists()).To(BeFalse())
					}
				})

				Context("Edit to invalid cni name", func() {
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSet(cniConfig(invalidCni)))
						f.RunHook()
					})

					It("Does not change cni module", func() {
						Expect(f).To(ExecuteSuccessfully())
						Expect(f.ValuesGet(module).Exists()).To(BeFalse())
					})
				})
			}
		})

		Context("With invalid cni name", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(cniConfig(invalidCni)))
				f.RunHook()
			})

			It("Does not enable any known modules", func() {
				Expect(f).To(ExecuteSuccessfully())
				for _, module := range cniNameToModule {
					Expect(f.ValuesGet(module).Exists()).To(BeFalse())
				}
			})

			It("Does not enable invalid module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(fmt.Sprintf("%sEnabled", invalidCni)).Exists()).To(BeFalse())
			})
		})
	})
})
