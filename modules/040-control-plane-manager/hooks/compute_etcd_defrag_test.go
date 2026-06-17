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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: compute_etcd_defrag ::", func() {
	const initConfigValuesString = ``

	newComputeHook := func(mastersCount int, hasArbiter bool, configEnabled *bool, configCronSchedule string) *HookExecutionConfig {
		mastersJSON := `[]`
		if mastersCount > 0 {
			nodes := make([]string, mastersCount)
			for i := range nodes {
				nodes[i] = `"master"`
			}
			mastersJSON = `["master"]`
			if mastersCount == 2 {
				mastersJSON = `["master-0","master-1"]`
			} else if mastersCount == 3 {
				mastersJSON = `["master-0","master-1","master-2"]`
			}
		}

		enabledJSON := "null"
		if configEnabled != nil {
			if *configEnabled {
				enabledJSON = "true"
			} else {
				enabledJSON = "false"
			}
		}

		cronJSON := ""
		if configCronSchedule != "" {
			cronJSON = `"` + configCronSchedule + `"`
		} else {
			cronJSON = `""`
		}

		values := `{
			"controlPlaneManager": {
				"internal": {
					"mastersNode": ` + mastersJSON + `,
					"hasEtcdArbiterNode": ` + boolStr(hasArbiter) + `,
					"etcdDefrag": {}
				},
				"etcd": {
					"defrag": {
						"cronSchedule": ` + cronJSON + `
					}
				},
				"apiserver": {"authn": {}, "authz": {}}
			}
		}`

		configValues := ``
		if configEnabled != nil {
			configValues = `{"controlPlaneManager":{"etcd":{"defrag":{"enabled":` + enabledJSON + `}}}}`
		}

		return HookExecutionConfigInit(values, configValues)
	}

	ptrBool := func(b bool) *bool { return &b }

	Context("3 master nodes, no explicit config", func() {
		f := newComputeHook(3, false, nil, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("sets enabled=true and cronSchedule from default", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragEnabledInternalPath).Bool()).To(BeTrue())
			Expect(f.ValuesGet(etcdDefragScheduleInternalPath).String()).To(Equal(etcdDefragDefaultCronSchedule))
		})
	})

	Context("2 master nodes + etcd arbiter, no explicit config", func() {
		f := newComputeHook(2, true, nil, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("sets enabled=true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragEnabledInternalPath).Bool()).To(BeTrue())
		})
	})

	Context("1 master node, no explicit config", func() {
		f := newComputeHook(1, false, nil, "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("sets enabled=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragEnabledInternalPath).Bool()).To(BeFalse())
		})
	})

	Context("1 master node, explicit enabled=true", func() {
		f := newComputeHook(1, false, ptrBool(true), "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("explicit config takes priority: enabled=true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragEnabledInternalPath).Bool()).To(BeTrue())
		})
	})

	Context("3 master nodes, explicit enabled=false", func() {
		f := newComputeHook(3, false, ptrBool(false), "")
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("explicit config takes priority: enabled=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragEnabledInternalPath).Bool()).To(BeFalse())
		})
	})

	Context("3 master nodes, explicit cronSchedule", func() {
		f := newComputeHook(3, false, nil, "0 3 * * *")
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("uses cronSchedule from config", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragScheduleInternalPath).String()).To(Equal("0 3 * * *"))
		})
	})
})

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
