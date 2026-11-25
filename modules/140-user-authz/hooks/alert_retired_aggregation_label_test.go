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
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authz hooks :: alert on a retired aggregation label ::", func() {
	const roles = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:manage:permission:module:ingress-nginx:view"
  labels:
    heritage: deckhouse
    module: ingress-nginx
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-networking-as: viewer
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:system-capability:storage-foundation:edit"
  labels:
    heritage: deckhouse
    module: storage-foundation
    rbac.deckhouse.io/kind: capability
    rbac.deckhouse.io/scope: system
    rbac.deckhouse.io/aggregate-to-infrastructure-as: manager
    rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    rbac.deckhouse.io/aggregate-to-storage-as: manager
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:custom:subsystem-capability:deckhouse:superresource:view"
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: deckhouse
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:use:capability:module:cert-manager:edit"
  labels:
    heritage: deckhouse
    module: cert-manager
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:system-capability:node-manager:edit"
  labels:
    heritage: deckhouse
    module: node-manager
    rbac.deckhouse.io/kind: capability
    rbac.deckhouse.io/scope: system
    rbac.deckhouse.io/aggregate-to-cluster-as: manager
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:manage:networking:manager"
  labels:
    heritage: deckhouse
    module: user-authz
    rbac.deckhouse.io/deprecated: "true"
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      rbac.deckhouse.io/aggregate-to-networking-as: manager
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:use:capability:module:cni-cilium:view"
  labels:
    heritage: deckhouse
    module: cni-cilium
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: viewer
    rbac.deckhouse.io/aggregate-to-networking-as: viewer
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "custom:use:logs"
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: user
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:use:role:user:virtualization"
  labels:
    heritage: deckhouse
    module: virtualization
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-all-as: user
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      rbac.deckhouse.io/kind: use
      rbac.deckhouse.io/aggregate-to-virtualization-as: user
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:use:capability:module:example:view"
  labels:
    heritage: deckhouse
    module: example
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-network-as: viewer
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "custom:use:audit"
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-iam-as: viewer
    rbac.deckhouse.io/aggregate-to-security-as: user
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "custom:use:releases"
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:use:capability:module:virtualization:view"
  labels:
    heritage: deckhouse
    module: virtualization
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-virtualization-as: user
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "custom:use:namespace"
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-namespace-as: user
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:use:capability:module:no-helm-lib:view"
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: viewer
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "module-role-without-prefix"
  labels:
    module: no-prefix
    rbac.deckhouse.io/aggregate-to-networking-as: viewer
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: plain
rules: []
`
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, "")

	// series maps "<name>/<lineage>" of every exported series to "<module>/<replaced_by>".
	series := func() map[string]string {
		out := map[string]string{}
		for _, m := range f.MetricsCollector.CollectedMetrics() {
			if m.Name == "d8_user_authz_retired_aggregation_label" && m.Value != nil {
				out[m.Labels["name"]+"/"+m.Labels["lineage"]] = m.Labels["module"] + "/" + m.Labels["replaced_by"]
			}
		}
		return out
	}

	// useSeries maps "<name>/<lineage>" of every exported use capability or use role of the previous
	// scheme to "<module>/<level>".
	useSeries := func() map[string]string {
		out := map[string]string{}
		for _, m := range f.MetricsCollector.CollectedMetrics() {
			if m.Name == "d8_user_authz_legacy_use_capability" && m.Value != nil {
				out[m.Labels["name"]+"/"+m.Labels["lineage"]] = m.Labels["module"] + "/" + m.Labels["level"]
			}
		}
		return out
	}

	Context("roles of every shape are present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(roles))
			f.RunHook()
		})
		It("flags every role of a user that carries a retired label and leaves the roles of the modules alone", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(series()).To(Equal(map[string]string{
				"d8:custom:subsystem-capability:deckhouse:superresource:view/deckhouse": "/cluster",
			}))
		})
		It("flags every use capability of a user of the previous scheme as one of its own and leaves the roles of the modules alone", func() {
			Expect(f).To(ExecuteSuccessfully())
			// A use capability under a lineage no role collects, a subsystem or a retired lineage other
			// than kubernetes, is flagged once per such lineage. The namespace lineage is collected and
			// left alone. The ClusterRoles with heritage: deckhouse belong to modules and are left out.
			Expect(useSeries()).To(Equal(map[string]string{
				"custom:use:logs/kubernetes":    "/user",
				"custom:use:audit/iam":          "/viewer",
				"custom:use:audit/security":     "/user",
				"custom:use:releases/deckhouse": "/manager",
			}))
		})
	})

	Context("no ClusterRole carries a retired label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("exports nothing and only expires the groups", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(series()).To(BeEmpty())
			Expect(useSeries()).To(BeEmpty())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(2))
		})
	})
})

// The lineages no role collects are written out twice, in uncollectedUseLineages and in the listing
// command of D8UserAuthzLegacyUseCapability. A subsystem added to the role model or renamed in it
// would otherwise drop a use capability of its lineage out of the alert in silence.
var _ = Describe("User Authz hooks :: lineages under which no role collects a use capability ::", func() {
	It("are system, the subsystems of the role model and the retired lineages, in the hook and in the alert", func() {
		entries, err := os.ReadDir(filepath.Join("..", "templates", "rbacv2", "global", "subsystem", "roles"))
		Expect(err).NotTo(HaveOccurred())
		expected := []string{"system"}
		for _, entry := range entries {
			if entry.IsDir() {
				expected = append(expected, entry.Name())
			}
		}
		Expect(expected).To(ContainElement("managed-services"))
		for lineage := range retiredLineageReplacements {
			// The use role aliases still collect a use capability under kubernetes.
			if lineage != "kubernetes" {
				expected = append(expected, lineage)
			}
		}
		Expect(slices.Collect(maps.Keys(uncollectedUseLineages))).To(ConsistOf(expected))

		rules, err := os.ReadFile(filepath.Join("..", "monitoring", "prometheus-rules", "retired-aggregation-label.yaml"))
		Expect(err).NotTo(HaveOccurred())
		inRe := regexp.MustCompile(`any\(IN\(([^)]*)\)\)`)
		quotedRe := regexp.MustCompile(`"([^"]+)"`)
		var listed []string
		for _, line := range strings.Split(string(rules), "\n") {
			if !strings.Contains(line, "-l rbac.deckhouse.io/kind=use") {
				continue
			}
			in := inRe.FindStringSubmatch(line)
			Expect(in).NotTo(BeNil(), line)
			for _, quoted := range quotedRe.FindAllStringSubmatch(in[1], -1) {
				listed = append(listed, quoted[1])
			}
		}
		// The listing also names the use capabilities and use roles that only the aliases still collect.
		Expect(listed).To(ConsistOf(slices.Concat(expected, []string{"kubernetes", "all"})))
	})
})
