/*
Copyright 2023 Flant JSC

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
	"fmt"
	"maps"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type labels = map[string]string

func objectYAML(kind, name, namespace string, l labels) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: v1\nkind: %s\nmetadata:\n  name: %s\n", kind, name)
	if namespace != "" {
		fmt.Fprintf(&b, "  namespace: %s\n", namespace)
	}
	if len(l) > 0 {
		b.WriteString("  labels:\n")
		for _, k := range slices.Sorted(maps.Keys(l)) {
			fmt.Fprintf(&b, "    %s: %q\n", k, l[k])
		}
	}
	return b.String()
}

func namespaceYAML(name string, l labels) string {
	return objectYAML("Namespace", name, "", l)
}

func terminatingNamespaceYAML(name string, l labels) string {
	return namespaceYAML(name, l) + "  deletionTimestamp: \"2020-10-22T21:30:34Z\"\n  finalizers:\n  - kubernetes\n"
}

func podYAML(name, namespace string, l labels) string {
	return objectYAML("Pod", name, namespace, l)
}

type discoveryExpectation int

const (
	notApplication discoveryExpectation = iota
	monitoredApplication
	unmonitoredApplication
)

var _ = Describe("Istio hooks :: discovery_application_namespaces ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{}}}`, "")

	run := func(objects ...string) {
		f.BindingContexts.Set(f.KubeStateSet(strings.Join(objects, "---\n")))
		f.RunHook()
		Expect(f).To(ExecuteSuccessfully())
	}
	applicationNamespaces := func() []string {
		return f.ValuesGet("istio.internal.applicationNamespaces").AsStringSlice()
	}
	applicationNamespacesToMonitor := func() []string {
		return f.ValuesGet("istio.internal.applicationNamespacesToMonitor").AsStringSlice()
	}

	It("Empty cluster", func() {
		run()
		Expect(f.LoggerOutput.Contents()).To(HaveLen(0))
		Expect(applicationNamespaces()).To(BeEmpty())
		Expect(applicationNamespacesToMonitor()).To(BeEmpty())
	})

	DescribeTable("Single namespace", func(want discoveryExpectation, objects ...string) {
		run(objects...)

		switch want {
		case notApplication:
			Expect(applicationNamespaces()).To(BeEmpty())
			Expect(applicationNamespacesToMonitor()).To(BeEmpty())
		case monitoredApplication:
			Expect(applicationNamespaces()).To(Equal([]string{"ns"}))
			Expect(applicationNamespacesToMonitor()).To(Equal([]string{"ns"}))
		case unmonitoredApplication:
			Expect(applicationNamespaces()).To(Equal([]string{"ns"}))
			Expect(applicationNamespacesToMonitor()).To(BeEmpty())
		}
	},
		// namespace labels
		Entry("NS without labels", notApplication,
			namespaceYAML("ns", nil)),
		Entry("NS with istio-injection=enabled", monitoredApplication,
			namespaceYAML("ns", labels{"istio-injection": "enabled"})),
		Entry("NS with istio-injection!=enabled", notApplication,
			namespaceYAML("ns", labels{"istio-injection": "disabled"})),
		Entry("NS with definite istio.io/rev", monitoredApplication,
			namespaceYAML("ns", labels{"istio.io/rev": "v1x13"})),
		Entry("NS with istio.io/rev=default", monitoredApplication,
			namespaceYAML("ns", labels{"istio.io/rev": "default"})),
		Entry("NS with empty istio.io/rev", notApplication,
			namespaceYAML("ns", labels{"istio.io/rev": ""})),
		Entry("NS with both istio-injection and istio.io/rev", notApplication,
			namespaceYAML("ns", labels{"istio-injection": "enabled", "istio.io/rev": "v1x14"})),

		// pod labels in a namespace without injection labels
		Entry("Pod without istio labels", notApplication,
			namespaceYAML("ns", nil),
			podYAML("pod", "ns", nil)),
		Entry("Pod with inject=true", monitoredApplication,
			namespaceYAML("ns", nil),
			podYAML("pod", "ns", labels{"sidecar.istio.io/inject": "true"})),
		Entry("Pod with definite istio.io/rev", monitoredApplication,
			namespaceYAML("ns", nil),
			podYAML("pod", "ns", labels{"istio.io/rev": "v1x11"})),
		Entry("Pod with inject=true and empty istio.io/rev", notApplication,
			namespaceYAML("ns", nil),
			podYAML("pod", "ns", labels{"sidecar.istio.io/inject": "true", "istio.io/rev": ""})),
		Entry("Pod with inject=false and definite istio.io/rev", notApplication,
			namespaceYAML("ns", nil),
			podYAML("pod", "ns", labels{"sidecar.istio.io/inject": "false", "istio.io/rev": "v1x11"})),

		// pod labels are ignored in a namespace with injection labels
		Entry("NS with istio-injection!=enabled, pod with inject=true", notApplication,
			namespaceYAML("ns", labels{"istio-injection": "disabled"}),
			podYAML("pod", "ns", labels{"sidecar.istio.io/inject": "true"})),
		Entry("NS with istio-injection!=enabled, pod with definite istio.io/rev", notApplication,
			namespaceYAML("ns", labels{"istio-injection": "disabled"}),
			podYAML("pod", "ns", labels{"istio.io/rev": "v1x11"})),
		Entry("NS with both istio-injection and istio.io/rev, pod with definite istio.io/rev", notApplication,
			namespaceYAML("ns", labels{"istio-injection": "enabled", "istio.io/rev": "v1x14"}),
			podYAML("pod", "ns", labels{"istio.io/rev": "v1x12"})),

		// terminating namespaces
		Entry("Terminating NS with istio-injection=enabled", notApplication,
			terminatingNamespaceYAML("ns", labels{"istio-injection": "enabled"})),
		Entry("Terminating NS without labels, pod with inject=true", notApplication,
			terminatingNamespaceYAML("ns", nil),
			podYAML("pod", "ns", labels{"sidecar.istio.io/inject": "true"})),

		// discard-metrics label
		Entry("NS with istio-injection=enabled and discard-metrics", unmonitoredApplication,
			namespaceYAML("ns", labels{"istio-injection": "enabled", "istio.deckhouse.io/discard-metrics": "true"})),
		Entry("NS with discard-metrics, pod with inject=true", unmonitoredApplication,
			namespaceYAML("ns", labels{"istio.deckhouse.io/discard-metrics": "true"}),
			podYAML("pod", "ns", labels{"sidecar.istio.io/inject": "true"})),
	)

	It("Several namespaces are sorted and deduplicated", func() {
		run(
			namespaceYAML("ns-b", labels{"istio-injection": "enabled"}),
			podYAML("pod-0", "ns-b", labels{"sidecar.istio.io/inject": "true"}),
			namespaceYAML("ns-a", nil),
			podYAML("pod-1", "ns-a", labels{"sidecar.istio.io/inject": "true"}),
			podYAML("pod-2", "ns-a", labels{"istio.io/rev": "v1x13"}),
			namespaceYAML("kube-ns", labels{"istio.io/rev": "v1x13"}),
			namespaceYAML("d8-ns", labels{"istio-injection": "enabled", "istio.deckhouse.io/discard-metrics": "true"}),
			namespaceYAML("ns-c", nil),
		)
		Expect(applicationNamespaces()).To(Equal([]string{"d8-ns", "kube-ns", "ns-a", "ns-b"}))
		Expect(applicationNamespacesToMonitor()).To(Equal([]string{"kube-ns", "ns-a", "ns-b"}))
	})
})
