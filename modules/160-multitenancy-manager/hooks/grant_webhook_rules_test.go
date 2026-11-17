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
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	helmengine "helm.sh/helm/v3/pkg/engine"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: grant_webhook_rules ::", func() {
	const initValues = `
global:
  discovery: {}
multitenancyManager:
  internal:
    admissionWebhookCert: {}
`

	const managedDefinitions = `
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: clusterroles
spec:
  enforcement: Managed
  defaultAvailability: All
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: storageclasses
spec:
  enforcement: Managed
  defaultAvailability: All
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: external
spec:
  enforcement: External
  defaultAvailability: All
`

	const rolebindingReference = `
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterroles-rolebinding
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups: ["rbac.authorization.k8s.io"]
    apiVersions: ["v1"]
    resources: ["rolebindings"]
`

	const otherReferences = `
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-pvc
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["persistentvolumeclaims"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterroles-rolebinding-duplicate
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups: ["rbac.authorization.k8s.io"]
    apiVersions: ["v1"]
    resources: ["rolebindings"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: dangling
spec:
  grantableClusterResourceName: missing
  rule:
    apiGroups: ["apps"]
    apiVersions: ["v1"]
    resources: ["deployments"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: external
spec:
  grantableClusterResourceName: external
  rule:
    apiGroups: ["batch"]
    apiVersions: ["v1"]
    resources: ["jobs"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: wildcard-group
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups: ["*"]
    apiVersions: ["v1"]
    resources: ["configmaps"]
`

	f := HookExecutionConfigInit(initValues, `{}`)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceDefinition", false)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceReference", false)

	Context("no grantable references", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(managedDefinitions))
			f.RunHook()
		})

		It("writes an empty list", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[]`))
		})
	})

	Context("references of every kind", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(managedDefinitions + rolebindingReference + otherReferences))
			f.RunHook()
		})

		It("keeps managed targets only, once each, sorted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
				{"apiGroup": "", "resource": "persistentvolumeclaims"},
				{"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
			]`))
		})

		Context("a reference is deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(managedDefinitions + rolebindingReference))
				f.RunHook()
			})

			It("drops its rule", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
					{"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
				]`))
			})
		})
	})

	Context("the registrations the module ships", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(renderShippedGrants()))
			f.RunHook()
		})

		// The rules are the cross product of rule.apiGroups and rule.resources, so a shipped reference
		// that mixed API groups would register pairs that do not exist, such as apps/pods. The exact
		// list catches that, and a path the module forgot to intercept as well.
		It("intercepts exactly the resources the shipped references name", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
				{"apiGroup": "", "resource": "persistentvolumeclaims"},
				{"apiGroup": "", "resource": "pods"},
				{"apiGroup": "", "resource": "podtemplates"},
				{"apiGroup": "", "resource": "replicationcontrollers"},
				{"apiGroup": "", "resource": "services"},
				{"apiGroup": "apps", "resource": "daemonsets"},
				{"apiGroup": "apps", "resource": "deployments"},
				{"apiGroup": "apps", "resource": "replicasets"},
				{"apiGroup": "apps", "resource": "statefulsets"},
				{"apiGroup": "batch", "resource": "cronjobs"},
				{"apiGroup": "batch", "resource": "jobs"},
				{"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
			]`))
		})
	})

	const templateReferences = `
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-statefulset-claims
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups: ["apps"]
    apiVersions: ["v1"]
    resources: ["statefulsets"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-pod-ephemeral
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods", "replicationcontrollers"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-apps-ephemeral
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups: ["apps"]
    apiVersions: ["v1"]
    resources: ["deployments", "statefulsets"]
`

	Context("references for volume claim templates", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(managedDefinitions + rolebindingReference + templateReferences))
			f.RunHook()
		})

		It("marks the resources only they name", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
				{"apiGroup": "", "resource": "pods", "volumeClaimTemplatesOnly": true},
				{"apiGroup": "", "resource": "replicationcontrollers", "volumeClaimTemplatesOnly": true},
				{"apiGroup": "apps", "resource": "deployments", "volumeClaimTemplatesOnly": true},
				{"apiGroup": "apps", "resource": "statefulsets", "volumeClaimTemplatesOnly": true},
				{"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
			]`))
		})

		Context("and another reference names one of the resources", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(managedDefinitions + rolebindingReference + templateReferences + `
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterroles-pods
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods"]
`))
				f.RunHook()
			})

			It("intercepts every object of that resource", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
					{"apiGroup": "", "resource": "pods"},
					{"apiGroup": "", "resource": "replicationcontrollers", "volumeClaimTemplatesOnly": true},
					{"apiGroup": "apps", "resource": "deployments", "volumeClaimTemplatesOnly": true},
					{"apiGroup": "apps", "resource": "statefulsets", "volumeClaimTemplatesOnly": true},
					{"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
				]`))
			})
		})

		const priorityClassReferences = `
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: priorityclasses-pod
spec:
  grantableClusterResourceName: priorityclasses
  rule:
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods", "replicationcontrollers", "podtemplates"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: priorityclasses-apps
spec:
  grantableClusterResourceName: priorityclasses
  rule:
    apiGroups: ["apps"]
    apiVersions: ["v1"]
    resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
`

		priorityClassDefinition := func(enforcement string) string {
			return `
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: priorityclasses
spec:
  enforcement: ` + enforcement + `
  defaultAvailability: All
`
		}

		Context("and the PriorityClass references name the pods and workloads", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(managedDefinitions + priorityClassDefinition("Managed") + rolebindingReference + templateReferences + priorityClassReferences))
				f.RunHook()
			})

			It("intercepts every object of the resources they name", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
					{"apiGroup": "", "resource": "pods"},
					{"apiGroup": "", "resource": "podtemplates"},
					{"apiGroup": "", "resource": "replicationcontrollers"},
					{"apiGroup": "apps", "resource": "daemonsets"},
					{"apiGroup": "apps", "resource": "deployments"},
					{"apiGroup": "apps", "resource": "replicasets"},
					{"apiGroup": "apps", "resource": "statefulsets"},
					{"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
				]`))
			})
		})

		Context("and the PriorityClass references point at an External definition", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(managedDefinitions + priorityClassDefinition("External") + rolebindingReference + templateReferences + priorityClassReferences))
				f.RunHook()
			})

			It("marks the resources only the volume claim template references name", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
					{"apiGroup": "", "resource": "pods", "volumeClaimTemplatesOnly": true},
					{"apiGroup": "", "resource": "replicationcontrollers", "volumeClaimTemplatesOnly": true},
					{"apiGroup": "apps", "resource": "deployments", "volumeClaimTemplatesOnly": true},
					{"apiGroup": "apps", "resource": "statefulsets", "volumeClaimTemplatesOnly": true},
					{"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
				]`))
			})
		})
	})

	Context("a reference with the name of a volume claim template reference for another definition", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(managedDefinitions + `
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-pod-ephemeral
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods"]
`))
			f.RunHook()
		})

		It("intercepts every object of its resources", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
				{"apiGroup": "", "resource": "pods"}
			]`))
		})
	})

	Context("the target definition switches to External", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: clusterroles
spec:
  enforcement: External
  defaultAvailability: All
` + rolebindingReference))
			f.RunHook()
		})

		It("stops intercepting the resource", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[]`))
		})
	})
})

// renderShippedGrants renders the module's templates/cluster-objects-controller/grantable-*.yaml with
// the real Helm engine (a stub for the lib-helm labels helper, storage-foundation not shipping the
// StorageClass grant so this module renders it) and returns the objects as one multi-document YAML.
func renderShippedGrants() string {
	paths, err := filepath.Glob(filepath.Join("..", "templates", "cluster-objects-controller", "grantable-*.yaml"))
	Expect(err).ToNot(HaveOccurred())
	Expect(paths).ToNot(BeEmpty())
	templates := []*chart.File{
		{Name: "templates/_helm_lib_stub.tpl", Data: []byte(`{{- define "helm_lib_module_labels" }}labels: {module: multitenancy-manager}{{- end }}`)},
	}
	for _, path := range paths {
		tpl, err := os.ReadFile(path)
		Expect(err).ToNot(HaveOccurred())
		templates = append(templates, &chart.File{Name: "templates/" + filepath.Base(path), Data: tpl})
	}
	ch := &chart.Chart{
		Metadata:  &chart.Metadata{Name: "multitenancy-manager", Version: "0.0.0", APIVersion: chart.APIVersionV2},
		Templates: templates,
	}
	values := chartutil.Values{"Values": map[string]any{
		"global":              map[string]any{"enabledModules": []any{}},
		"multitenancyManager": map[string]any{"internal": map[string]any{"storageFoundationShipsStorageClassGrant": false}},
	}}
	rendered, err := helmengine.Render(ch, values)
	Expect(err).ToNot(HaveOccurred())
	var docs []string
	for _, path := range paths {
		out, ok := rendered["multitenancy-manager/templates/"+filepath.Base(path)]
		Expect(ok).To(BeTrue(), path)
		for _, doc := range strings.Split(out, "\n---") {
			if strings.Contains(doc, "kind:") {
				docs = append(docs, strings.TrimSpace(doc))
			}
		}
	}
	return strings.Join(docs, "\n---\n")
}
