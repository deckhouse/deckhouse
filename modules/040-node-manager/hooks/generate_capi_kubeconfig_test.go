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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Generate CAPI kubeconfig", func() {
	DescribeTable("resolves controller-reachable API endpoint",
		func(endpoint string, clusterMasterAddresses []string, expected string) {
			Expect(resolveCAPIKubeconfigEndpoint(endpoint, clusterMasterAddresses)).To(Equal(expected))
		},
		Entry("keeps non-loopback endpoint", "https://10.0.0.50:6443", []string{"10.0.0.1:6443"}, "https://10.0.0.50:6443"),
		Entry("replaces localhost endpoint", "https://localhost:6445", []string{"10.0.0.1:6443"}, "https://10.0.0.1:6443"),
		Entry("replaces 127.0.0.1 endpoint", "https://127.0.0.1:6445", []string{"10.0.0.1:6443"}, "https://10.0.0.1:6443"),
		Entry("keeps localhost endpoint without cluster master addresses", "https://127.0.0.1:6445", nil, "https://127.0.0.1:6445"),
	)
})
