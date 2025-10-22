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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: openvpn :: hooks :: set_push_routes ", func() {
	const globalPodSubnet = "10.0.0.0/24"
	const globalServiceSubnet = "10.0.2.0/24"

	f := HookExecutionConfigInit(
		`{ "global": {"discovery":{}}, "openvpn":{"internal":{"auth": {}}} }`,
		`{"openvpn":{}}`)

	Context("openvpn.pushToClientRoutes is not set", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet(globalPodSubnetPath, globalPodSubnet)
			f.ValuesSet(globalServiceSubnetPath, globalServiceSubnet)
			f.RunHook()
		})

		It("should push subnets from global discovery", func() {
			Expect(f).To(ExecuteSuccessfully())
			routeArray := f.ValuesGet(clientRoutesInternalValuesPath).Array()
			Expect(routeArray).ShouldNot(BeEmpty())
			Expect(routesFromArray(routeArray)).Should(SatisfyAll(
				ContainElement(globalPodSubnet),
				ContainElement(globalServiceSubnet),
			))
		})
	})

	Context("openvpn.pushToClientRoutes is set", func() {
		const userDefinedRoute = "example.example"

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet(globalPodSubnetPath, globalPodSubnet)
			f.ValuesSet(globalServiceSubnetPath, globalServiceSubnet)
			f.ConfigValuesSet("openvpn.pushToClientRoutes", []string{userDefinedRoute})
			f.RunHook()
		})

		It("should push subnets from global discovery and from configuration", func() {
			Expect(f).To(ExecuteSuccessfully())

			routeArray := f.ValuesGet(clientRoutesInternalValuesPath).Array()
			Expect(routeArray).ShouldNot(BeEmpty())
			Expect(routesFromArray(routeArray)).Should(SatisfyAll(
				ContainElement(userDefinedRoute),
				ContainElement(globalPodSubnet),
				ContainElement(globalServiceSubnet),
			))
		})
	})
})

func routesFromArray(arr []gjson.Result) []string {
	routes := make([]string, 0)
	for _, route := range arr {
		routes = append(routes, route.String())
	}
	return routes
}
