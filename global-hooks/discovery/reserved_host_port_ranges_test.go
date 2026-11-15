// Copyright 2026 Flant JSC
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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: reserved_host_port_ranges ::", func() {
	pod := func(namespace, name, label, container, command, args string) string {
		return fmt.Sprintf(`
---
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    %s: kube-apiserver
spec:
  containers:
  - name: %s
    command: %s
    args: %s
`, name, namespace, label, container, command, args)
	}
	apiserver := func(name, label, command, args string) string {
		return pod("kube-system", name, label, "kube-apiserver", command, args)
	}
	apiserverWithRange := func(name, value string) string {
		return apiserver(name, "component", `["kube-apiserver"]`, `["--service-node-port-range=`+value+`"]`)
	}
	ciliumConfig := func(namespace, data string) string {
		return fmt.Sprintf(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: %s
data:
`, namespace) + data
	}
	cilium := func(data string) string {
		return ciliumConfig("d8-cni-cilium", data)
	}
	defaultAPI := apiserver("api", "component", `["kube-apiserver"]`, `[]`)

	for _, tc := range []struct {
		name, state, ranges string
	}{
		{"default without a visible apiserver", "", `[{"min":30000,"max":32767}]`},
		{"default for an apiserver without the flag", defaultAPI, `[{"min":30000,"max":32767}]`},
		{"range in command", apiserver("api", "component", `["kube-apiserver","--service-node-port-range=20000-21000"]`, `[]`), `[{"min":20000,"max":21000}]`},
		{"split argument", apiserver("api", "component", `["kube-apiserver"]`, `["--service-node-port-range","20000-21000"]`), `[{"min":20000,"max":21000}]`},
		{"single API port", apiserver("api", "component", `["kube-apiserver"]`, `["--service-node-port-range=31000"]`), `[{"min":31000,"max":31000}]`},
		{"API range with offset", apiserver("api", "component", `["kube-apiserver"]`, `["--service-node-port-range=31000+99"]`), `[{"min":31000,"max":31099}]`},
		{"overlap across apiservers", defaultAPI + apiserver("api-2", "component", `["kube-apiserver"]`, `["--service-node-port-range=32000-34000"]`), `[{"min":30000,"max":34000}]`},
		{"disjoint ranges across apiservers", apiserverWithRange("api", "20000-21000") + apiserverWithRange("api-2", "40000-41000"), `[{"min":20000,"max":21000},{"min":40000,"max":41000}]`},
		{"apiserver outside kube-system", pod("default", "api", "component", "kube-apiserver", `["kube-apiserver"]`, `["--service-node-port-range=20000-21000"]`), `[{"min":30000,"max":32767}]`},
		{"apiserver without the component label", apiserver("api", "tier", `["kube-apiserver"]`, `["--service-node-port-range=20000-21000"]`), `[{"min":30000,"max":32767}]`},
		{"Pod without a kube-apiserver container", pod("kube-system", "api", "component", "proxy", `["proxy"]`, `["--service-node-port-range=20000-21000"]`), `[{"min":30000,"max":32767}]`},
		{"Cilium default", defaultAPI + cilium(`  kube-proxy-replacement: "true"`), `[{"min":30000,"max":32767}]`},
		{"disjoint Cilium range", defaultAPI + cilium("  kube-proxy-replacement: \"true\"\n  node-port-range: \"20000,21000\""), `[{"min":20000,"max":21000},{"min":30000,"max":32767}]`},
		{"adjacent Cilium range", defaultAPI + cilium("  kube-proxy-replacement: \"true\"\n  node-port-range: \"32768,33000\""), `[{"min":30000,"max":33000}]`},
		{"Cilium without visible apiserver", cilium("  kube-proxy-replacement: \"true\"\n  node-port-range: \"20000,21000\""), `[{"min":20000,"max":21000},{"min":30000,"max":32767}]`},
		{"disabled native Cilium", defaultAPI + cilium("  kube-proxy-replacement: \"false\"\n  node-port-range: \"20000,21000\""), `[{"min":30000,"max":32767}]`},
		{"Cilium without kube-proxy-replacement", defaultAPI + cilium(`  node-port-range: "20000,21000"`), `[{"min":30000,"max":32767}]`},
		{"Cilium ConfigMap without data", defaultAPI + cilium(""), `[{"min":30000,"max":32767}]`},
		{"legacy strict native Cilium", defaultAPI + cilium("  kube-proxy-replacement: \"strict\"\n  node-port-range: \"20000,21000\""), `[{"min":20000,"max":21000},{"min":30000,"max":32767}]`},
		{"legacy disabled native Cilium", defaultAPI + cilium("  kube-proxy-replacement: \"disabled\"\n  node-port-range: \"20000,21000\""), `[{"min":30000,"max":32767}]`},
		{"Cilium in kube-system is not Deckhouse's", defaultAPI + ciliumConfig("kube-system", "  kube-proxy-replacement: \"true\"\n  node-port-range: \"20000,21000\""), `[{"min":30000,"max":32767}]`},
		{"Deckhouse Cilium next to one in kube-system", defaultAPI + cilium("  kube-proxy-replacement: \"true\"\n  node-port-range: \"20000,21000\"") + ciliumConfig("kube-system", "  kube-proxy-replacement: \"true\"\n  node-port-range: \"40000,41000\""), `[{"min":20000,"max":21000},{"min":30000,"max":32767}]`},
		{"unknown settings of a Cilium in kube-system", defaultAPI + ciliumConfig("kube-system", "  kube-proxy-replacement: \"partial\""), `[{"min":30000,"max":32767}]`},
		{"empty flag is default", apiserver("api", "component", `["kube-apiserver"]`, `["--service-node-port-range="]`), `[{"min":30000,"max":32767}]`},
		{"empty split argument is default", apiserver("api", "component", `["kube-apiserver"]`, `["--service-node-port-range",""]`), `[{"min":30000,"max":32767}]`},
	} {
		Context(tc.name, func() {
			f := HookExecutionConfigInit(`{"global":{"discovery":{}}}`, `{}`)
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(tc.state))
				f.RunHook()
			})
			It("publishes the reserved ranges", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.reservedHostPortRanges").Raw).To(MatchJSON(tc.ranges))
			})
		})
	}

	Context("sources disappear", func() {
		f := HookExecutionConfigInit(`{"global":{"discovery":{"reservedHostPortRanges":[{"min":20000,"max":21000}]}}}`, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("replaces stale reservations", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.reservedHostPortRanges").Raw).To(MatchJSON(`[{"min":30000,"max":32767}]`))
		})
	})

	// Every step is a hook run on the same cluster, as while the apiservers are reconfigured one by one.
	type migrationStep struct{ state, ranges string }
	for _, tc := range []struct {
		name  string
		steps []migrationStep
	}{
		{"apiservers move to another range", []migrationStep{
			{apiserverWithRange("api-0", "20000-21000") + apiserverWithRange("api-1", "20000-21000"), `[{"min":20000,"max":21000}]`},
			{apiserverWithRange("api-0", "40000-41000") + apiserverWithRange("api-1", "20000-21000"), `[{"min":20000,"max":21000},{"min":40000,"max":41000}]`},
			{apiserverWithRange("api-0", "40000-41000") + apiserverWithRange("api-1", "40000-41000"), `[{"min":40000,"max":41000}]`},
		}},
		{"apiservers drop the flag", []migrationStep{
			{apiserverWithRange("api-0", "20000-21000") + apiserverWithRange("api-1", "20000-21000"), `[{"min":20000,"max":21000}]`},
			{apiserver("api-0", "component", `["kube-apiserver"]`, `[]`) + apiserverWithRange("api-1", "20000-21000"), `[{"min":20000,"max":21000},{"min":30000,"max":32767}]`},
			{apiserver("api-0", "component", `["kube-apiserver"]`, `[]`) + apiserver("api-1", "component", `["kube-apiserver"]`, `[]`), `[{"min":30000,"max":32767}]`},
		}},
		{"the only apiserver Pod is recreated", []migrationStep{
			{apiserverWithRange("api-0", "20000-21000"), `[{"min":20000,"max":21000}]`},
			{"", `[{"min":30000,"max":32767}]`},
			{apiserverWithRange("api-0", "40000-41000"), `[{"min":40000,"max":41000}]`},
		}},
		{"Cilium moves to another range", []migrationStep{
			{defaultAPI + cilium("  kube-proxy-replacement: \"true\"\n  node-port-range: \"20000,21000\""), `[{"min":20000,"max":21000},{"min":30000,"max":32767}]`},
			{defaultAPI + cilium("  kube-proxy-replacement: \"true\"\n  node-port-range: \"40000,41000\""), `[{"min":30000,"max":32767},{"min":40000,"max":41000}]`},
		}},
	} {
		Context(tc.name, func() {
			f := HookExecutionConfigInit(`{"global":{"discovery":{}}}`, `{}`)
			It("publishes the ranges of every step", func() {
				for _, step := range tc.steps {
					f.BindingContexts.Set(f.KubeStateSet(step.state))
					f.RunHook()
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.discovery.reservedHostPortRanges").Raw).To(MatchJSON(step.ranges))
				}
			})
		})
	}

	// The test cluster cannot list objects a filter fails on, so the filters are called directly.
	Context("invalid sources", func() {
		toUnstructured := func(manifest string) *unstructured.Unstructured {
			obj := &unstructured.Unstructured{}
			Expect(yaml.Unmarshal([]byte(manifest), &obj.Object)).To(Succeed())
			return obj
		}

		for _, tc := range []struct {
			name, manifest, err string
			filter              go_hook.FilterFunc
		}{
			{"zero API port", apiserver("api", "component", `["kube-apiserver"]`, `["--service-node-port-range=0"]`), `invalid NodePort range "0"`, filterAPIServerNodePortRange},
			{"API offset exceeds port limit", apiserver("api", "component", `["kube-apiserver"]`, `["--service-node-port-range=65535+1"]`), `invalid NodePort range "65535+1"`, filterAPIServerNodePortRange},
			{"reversed API range", apiserver("api", "component", `["kube-apiserver"]`, `["--service-node-port-range=40000-30000"]`), `invalid NodePort range "40000-30000"`, filterAPIServerNodePortRange},
			{"invalid Cilium range", cilium("  kube-proxy-replacement: \"true\"\n  node-port-range: \"0,65536\""), `invalid NodePort range "0,65536"`, filterCiliumNodePortRange},
			{"unknown kube-proxy-replacement", cilium(`  kube-proxy-replacement: "partial"`), `unknown kube-proxy-replacement "partial"`, filterCiliumNodePortRange},
		} {
			It("fails on "+tc.name, func() {
				_, err := tc.filter(toUnstructured(tc.manifest))
				Expect(err).To(MatchError(tc.err))
			})
		}
	})
})
