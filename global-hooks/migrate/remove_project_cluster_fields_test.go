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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: migrate/remove_project_cluster_fields ::", func() {
	const (
		initValuesString = `{}`
	)

	Context("Cluster with old configmap", func() {
		const initConfigValuesString = `{"global":{"clusterName":"test-cluster", "project":"test-project"}}`
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook migrates old values into new ones", func() {
			Expect(f.ConfigValuesGet("global.clusterName").Exists()).To(BeFalse())
			Expect(f.ConfigValuesGet("global.project").Exists()).To(BeFalse())
		})
	})

	Context("With additional fields in global section", func() {
		const initConfigValuesString = `{"global":{"clusterName":"test-cluster", "project":"test-project", "anotherValue": "test"}}`
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook migrates old values into new ones", func() {
			Expect(f.ConfigValuesGet("global.clusterName").Exists()).To(BeFalse())
			Expect(f.ConfigValuesGet("global.project").Exists()).To(BeFalse())
			Expect(f.ConfigValuesGet("global.anotherValue").String()).To(Equal("test"))
		})
	})

	Context("With migrated config", func() {
		const initConfigValuesString = `{"global":{"anotherValue": "test"}}`
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook migrates old values into new ones", func() {
			Expect(f.ConfigValuesGet("global.clusterName").Exists()).To(BeFalse())
			Expect(f.ConfigValuesGet("global.project").Exists()).To(BeFalse())
			Expect(f.ConfigValuesGet("global.anotherValue").String()).To(Equal("test"))
		})
	})
})
