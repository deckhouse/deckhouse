// /*
// Copyright 2023 Flant JSC

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

// package hooks

// import (
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"

// 	. "github.com/deckhouse/deckhouse/testing/hooks"
// )

// var _ = Describe("Istio hooks :: ambient_mode_monitor ::", func() {
// 	const (
// 		emptyCluster     = ``
// 		configMapPresent = `
// ---
// apiVersion: v1
// kind: ConfigMap
// metadata:
//   name: istio-ambientmode
//   namespace: d8-istio
// `
// 		configMapInWrongNamespace = `
// ---
// apiVersion: v1
// kind: ConfigMap
// metadata:
//   name: istio-ambientmode
//   namespace: wrong-namespace
// `
// 		otherConfigMap = `
// ---
// apiVersion: v1
// kind: ConfigMap
// metadata:
//   name: other-configmap
//   namespace: d8-istio
// `
// 	)

// 	f := HookExecutionConfigInit(`{"istio":{}}`, `{}`)
// 	f.RegisterCRD("", "v1", "ConfigMap", true)

// 	Context("Empty cluster", func() {
// 		BeforeEach(func() {
// 			f.BindingContexts.Set(f.KubeStateSet(emptyCluster))
// 			f.RunHook()
// 		})

// 		It("Should set enableAmbientMode to false", func() {
// 			Expect(f).To(ExecuteSuccessfully())
// 			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeFalse())
// 		})
// 	})

// 	Context("ConfigMap present in correct namespace", func() {
// 		BeforeEach(func() {
// 			f.BindingContexts.Set(f.KubeStateSet(configMapPresent))
// 			f.RunHook()
// 		})

// 		It("Should set enableAmbientMode to true", func() {
// 			Expect(f).To(ExecuteSuccessfully())
// 			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeTrue())
// 		})
// 	})

// 	Context("ConfigMap present in wrong namespace", func() {
// 		BeforeEach(func() {
// 			f.BindingContexts.Set(f.KubeStateSet(configMapInWrongNamespace))
// 			f.RunHook()
// 		})

// 		It("Should set enableAmbientMode to false", func() {
// 			Expect(f).To(ExecuteSuccessfully())
// 			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeFalse())
// 		})
// 	})

// 	Context("Other ConfigMap present in namespace", func() {
// 		BeforeEach(func() {
// 			f.BindingContexts.Set(f.KubeStateSet(otherConfigMap))
// 			f.RunHook()
// 		})

// 		It("Should set enableAmbientMode to false", func() {
// 			Expect(f).To(ExecuteSuccessfully())
// 			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeFalse())
// 		})
// 	})

// 	Context("Multiple ConfigMaps including target one", func() {
// 		BeforeEach(func() {
// 			state := configMapPresent + otherConfigMap
// 			f.BindingContexts.Set(f.KubeStateSet(state))
// 			f.RunHook()
// 		})

// 		It("Should set enableAmbientMode to true", func() {
// 			Expect(f).To(ExecuteSuccessfully())
// 			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeTrue())
// 		})
// 	})

// 	Context("ConfigMap gets deleted", func() {
// 		BeforeEach(func() {
// 			// First run with ConfigMap present
// 			f.BindingContexts.Set(f.KubeStateSet(configMapPresent))
// 			f.RunHook()
// 			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeTrue())

// 			// Then run with ConfigMap deleted
// 			f.BindingContexts.Set(f.KubeStateSet(emptyCluster))
// 			f.RunHook()
// 		})

// 		It("Should update enableAmbientMode to false", func() {
// 			Expect(f).To(ExecuteSuccessfully())
// 			Expect(f.ValuesGet("istio.internal.enableAmbientMode").Bool()).To(BeFalse())
// 		})
// 	})
// })
