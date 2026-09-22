/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// providerSecretYAML returns a d8-node-manager-cloud-provider Secret whose data.openstack is a
// b64-encoded JSON tree with the given authURL under connection. Empty authURL means "connection
// block absent" — used to exercise the bootstrap-race branch of the hook.
func providerSecretYAML(authURL string) string {
	tree := map[string]any{}
	if authURL != "" {
		tree["connection"] = map[string]any{"authURL": authURL}
	}
	raw, _ := json.Marshal(tree)
	return fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-node-manager-cloud-provider
  namespace: kube-system
data:
  openstack: %s
`, base64.StdEncoding.EncodeToString(raw))
}

const (
	preemptibleICEnabled = `
---
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: worker-preempt
spec:
  flavorName: SL1.1-2048
  preemptible: true
`
	preemptibleICDisabled = `
---
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: worker-plain
spec:
  flavorName: SL1.1-2048
  preemptible: false
`
	ngCAPIWithPreempt = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-preempt-capi
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker-preempt
    minPerZone: 1
    maxPerZone: 1
status:
  engine: CAPI
`
	ngMCMWithPreempt = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-preempt-mcm
  annotations:
    node.deckhouse.io/use-mcm: "true"
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker-preempt
    minPerZone: 1
    maxPerZone: 1
status:
  engine: MCM
`
	// No status.engine — the filter falls back to the useMCM heuristic; without the annotation
	// it should resolve to CAPI, matching what node-controller will pin on first reconcile.
	ngFreshDefaultsToCAPI = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-fresh
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker-preempt
    minPerZone: 1
    maxPerZone: 1
`
	ngStaticWithPreemptClassRef = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static
spec:
  nodeType: Static
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker-preempt
`
	ngCAPIWithPlainClass = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-plain-capi
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker-plain
    minPerZone: 1
    maxPerZone: 1
status:
  engine: CAPI
`
	ngYandexKindWithPreemptName = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-yandex
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: worker-preempt
    minPerZone: 1
    maxPerZone: 1
status:
  engine: CAPI
`
)

var _ = Describe("Modules :: cloudProviderOpenstack :: hooks :: metrics_preemptible_unsupported ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	nodeGroupGVR := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
	openstackInstanceClassGVR := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "openstackinstanceclasses"}
	f.RegisterCRD(nodeGroupGVR.Group, nodeGroupGVR.Version, "NodeGroup", false)
	f.RegisterCRD(openstackInstanceClassGVR.Group, openstackInstanceClassGVR.Version, "OpenStackInstanceClass", false)

	// A metric operation shape: Expire is always emitted first (so a fixed/deleted NG loses its
	// timeseries the same tick), and any active reasons follow.
	expireOnly := func() operation.MetricOperation {
		return operation.MetricOperation{
			Group:  preemptibleUnsupportedMetricGroup,
			Action: operation.ActionExpireMetrics,
		}
	}
	unsupported := func(nodeGroup, reason string) operation.MetricOperation {
		return operation.MetricOperation{
			Name:   preemptibleUnsupportedMetricName,
			Group:  preemptibleUnsupportedMetricGroup,
			Action: operation.ActionGaugeSet,
			Value:  ptr.To(1.0),
			Labels: map[string]string{"node_group": nodeGroup, "reason": reason},
		}
	}

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunGoHook()
		})

		// Even with nothing to report, Expire must fire — otherwise a metric published by a
		// previous run of the hook would linger indefinitely.
		It("Emits only Expire and no active timeseries", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("MCM NodeGroup + preemptible IC on a Selectel cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				preemptibleICEnabled +
					ngMCMWithPreempt +
					providerSecretYAML("https://cloud.api.selcloud.ru/identity/v3"),
			))
			f.RunGoHook()
		})

		// MCM overrides the authURL check — MachineClass has no field for a raw Nova tag no
		// matter what the provider is, so the reason must be `mcm`, not `non-selectel`.
		It("Emits reason=mcm for the MCM NodeGroup", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
			Expect(m[1]).To(BeEquivalentTo(unsupported("worker-preempt-mcm", "mcm")))
		})
	})

	Context("CAPI NodeGroup + preemptible IC on a non-Selectel cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				preemptibleICEnabled +
					ngCAPIWithPreempt +
					providerSecretYAML("https://public.infra.mail.ru:5000/v3/"),
			))
			f.RunGoHook()
		})

		It("Emits reason=non-selectel for the CAPI NodeGroup", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
			Expect(m[1]).To(BeEquivalentTo(unsupported("worker-preempt-capi", "non-selectel")))
		})
	})

	Context("CAPI NodeGroup + preemptible IC on a Selectel cluster (selcloud.ru)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				preemptibleICEnabled +
					ngCAPIWithPreempt +
					providerSecretYAML("https://cloud.api.selcloud.ru/identity/v3"),
			))
			f.RunGoHook()
		})

		// The healthy path — tag reaches Nova, alert must stay silent.
		It("Emits only Expire, no active reasons", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("CAPI NodeGroup + preemptible IC on a Selectel cluster (selectel.ru)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				preemptibleICEnabled +
					ngCAPIWithPreempt +
					providerSecretYAML("https://api.selectel.ru/identity/v3"),
			))
			f.RunGoHook()
		})

		It("Matches the alternative Selectel domain and stays silent", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("CAPI NodeGroup + preemptible IC but authURL not yet published (bootstrap)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				preemptibleICEnabled + ngCAPIWithPreempt + providerSecretYAML(""),
			))
			f.RunGoHook()
		})

		// A cluster is "not yet Selectel" until the discovery hook populates connection —
		// otherwise every fresh cluster would raise the non-Selectel alert during bootstrap.
		It("Suppresses the alert on the first pass before authURL is known", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("CAPI NodeGroup with preemptible: false IC", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				preemptibleICDisabled +
					ngCAPIWithPlainClass +
					providerSecretYAML("https://public.infra.mail.ru:5000/v3/"),
			))
			f.RunGoHook()
		})

		// The operator did not ask for preemption, so an unusual provider is irrelevant.
		It("Emits nothing", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("Static NodeGroup referencing a preemptible IC", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				preemptibleICEnabled +
					ngStaticWithPreemptClassRef +
					providerSecretYAML("https://public.infra.mail.ru:5000/v3/"),
			))
			f.RunGoHook()
		})

		// Static NGs never create cloud VMs — the field cannot even attempt to reach Nova, so
		// the metric must ignore them regardless of provider.
		It("Ignores non-CloudEphemeral NodeGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("NodeGroup referencing YandexInstanceClass with the same name", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				preemptibleICEnabled +
					ngYandexKindWithPreemptName +
					providerSecretYAML("https://public.infra.mail.ru:5000/v3/"),
			))
			f.RunGoHook()
		})

		// The field is provider-scoped: a foreign kind must not accidentally raise the alert
		// even when the class name collides with an OpenStackInstanceClass in the cluster.
		It("Ignores NodeGroups whose classReference.kind is not OpenStackInstanceClass", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
		})
	})

	Context("Fresh NodeGroup with no status.engine on a non-Selectel cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				preemptibleICEnabled +
					ngFreshDefaultsToCAPI +
					providerSecretYAML("https://public.infra.mail.ru:5000/v3/"),
			))
			f.RunGoHook()
		})

		// Openstack publishes both engine kinds, default is CAPI unless useMCM is set. Without
		// this fallback the alert would lag by one reconcile on a fresh NG — the operator would
		// think their apply was silently swallowed.
		It("Defaults to CAPI and emits reason=non-selectel", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(expireOnly()))
			Expect(m[1]).To(BeEquivalentTo(unsupported("worker-fresh", "non-selectel")))
		})
	})
})
