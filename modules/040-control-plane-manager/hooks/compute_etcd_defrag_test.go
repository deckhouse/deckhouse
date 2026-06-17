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

const threeMastersValues = `{
	"controlPlaneManager": {
		"internal": {
			"mastersNode": ["master-0","master-1","master-2"],
			"hasEtcdArbiterNode": false,
			"etcdDefrag": {}
		},
		"etcd": {
			"defrag": {
				"cronSchedule": ""
			}
		},
		"apiserver": {"authn": {}, "authz": {}}
	}
}`

const twoMastersArbiterValues = `{
	"controlPlaneManager": {
		"internal": {
			"mastersNode": ["master-0","master-1"],
			"hasEtcdArbiterNode": true,
			"etcdDefrag": {}
		},
		"etcd": {
			"defrag": {
				"cronSchedule": ""
			}
		},
		"apiserver": {"authn": {}, "authz": {}}
	}
}`

const oneMasterValues = `{
	"controlPlaneManager": {
		"internal": {
			"mastersNode": ["master-0"],
			"hasEtcdArbiterNode": false,
			"etcdDefrag": {}
		},
		"etcd": {
			"defrag": {
				"cronSchedule": ""
			}
		},
		"apiserver": {"authn": {}, "authz": {}}
	}
}`

const threeMastersCustomCronValues = `{
	"controlPlaneManager": {
		"internal": {
			"mastersNode": ["master-0","master-1","master-2"],
			"hasEtcdArbiterNode": false,
			"etcdDefrag": {}
		},
		"etcd": {
			"defrag": {
				"cronSchedule": "0 3 * * *"
			}
		},
		"apiserver": {"authn": {}, "authz": {}}
	}
}`

var _ = Describe("Modules :: control-plane-manager :: hooks :: compute_etcd_defrag ::", func() {
	Context("3 master nodes, no explicit config", func() {
		f := HookExecutionConfigInit(threeMastersValues, ``)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("sets enabled=true and default cronSchedule", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragEnabledInternalPath).Bool()).To(BeTrue())
			Expect(f.ValuesGet(etcdDefragScheduleInternalPath).String()).To(Equal(etcdDefragDefaultCronSchedule))
		})
	})

	Context("2 master nodes + etcd arbiter, no explicit config", func() {
		f := HookExecutionConfigInit(twoMastersArbiterValues, ``)
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
		f := HookExecutionConfigInit(oneMasterValues, ``)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("sets enabled=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragEnabledInternalPath).Bool()).To(BeFalse())
		})
	})

	Context("1 master node, explicit enabled=true in config", func() {
		f := HookExecutionConfigInit(oneMasterValues, `{"controlPlaneManager":{"etcd":{"defrag":{"enabled":true}}}}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("explicit config takes priority: enabled=true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragEnabledInternalPath).Bool()).To(BeTrue())
		})
	})

	Context("3 master nodes, explicit enabled=false in config", func() {
		f := HookExecutionConfigInit(threeMastersValues, `{"controlPlaneManager":{"etcd":{"defrag":{"enabled":false}}}}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("explicit config takes priority: enabled=false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragEnabledInternalPath).Bool()).To(BeFalse())
		})
	})

	Context("3 master nodes, custom cronSchedule in values", func() {
		f := HookExecutionConfigInit(threeMastersCustomCronValues, ``)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("uses cronSchedule from values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(etcdDefragScheduleInternalPath).String()).To(Equal("0 3 * * *"))
		})
	})
})
