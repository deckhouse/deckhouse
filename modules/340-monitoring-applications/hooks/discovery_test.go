package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	singleServiceWithPrometheusTarget = `
---
apiVersion: v1
kind: Service
metadata:
  name: s0
  labels:
    prometheus-target: php-fpm
`
	twoServicesWithOldAnnotations = `
---
apiVersion: v1
kind: Service
metadata:
  name: s1
  labels:
    prometheus-target: php-fpm
---
apiVersion: v1
kind: Service
metadata:
  name: s2
  labels:
    prometheus-target: winword
`
	newAnnotationDiscoveryService = `
---
apiVersion: v1
kind: Service
metadata:
  name: new
  labels:
    prometheus.deckhouse.io/target: test
`
)

var _ = Describe("Modules :: monitoring-applications :: hooks :: discovery ::", func() {
	f := HookExecutionConfigInit(`{"monitoringApplications":{"discovery":{"enabledApplications": []},"internal":{"enabledApplicationsSummary": []}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Single Service added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(singleServiceWithPrometheusTarget))
				f.RunHook()
			})

			It("enabledApplications must contain single application 'php-fpm'", func() {
				Expect(f).To(ExecuteSuccessfully())
				// null in enabledApplications appears only because fake kubernetes client do not support proper label selection
				Expect(f.ValuesGet("monitoringApplications.discovery.enabledApplications").String()).To(MatchJSON(`["php-fpm"]`))
			})
		})

		Context("Single Service with new annotation added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(newAnnotationDiscoveryService))
				f.RunHook()
			})

			It("enabledApplications must contain single application 'test'", func() {
				Expect(f).To(ExecuteSuccessfully())
				// null in enabledApplications appears only because fake kubernetes client do not support proper label selection
				Expect(f.ValuesGet("monitoringApplications.discovery.enabledApplications").String()).To(MatchJSON(`["test"]`))
			})
		})
	})

	Context("Single Service in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(singleServiceWithPrometheusTarget))
			f.RunHook()
		})

		It("enabledApplications must contain single application 'php-fpm'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringApplications.discovery.enabledApplications").String()).To(MatchJSON(`["php-fpm"]`))
		})

		Context("Two more Services added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(singleServiceWithPrometheusTarget + twoServicesWithOldAnnotations))
				f.RunHook()
			})

			It("enabledApplications must contain single application 'php-fpm' and 'winword'", func() {
				Expect(f).To(ExecuteSuccessfully())
				// null in enabledApplications appears only because fake kubernetes client do not support proper label selection
				Expect(f.ValuesGet("monitoringApplications.discovery.enabledApplications").String()).To(MatchJSON(`["php-fpm", "winword"]`))
			})
		})
	})

	Context("BeforeHelm — nothing discovered, nothing configured", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("monitoringApplications.internal.enabledApplicationsSummary must be []", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringApplications.internal.enabledApplicationsSummary").String()).To(MatchJSON(`[]`))
		})
	})

	Context("BeforeHelm — discovered and configured", func() {
		BeforeEach(func() {
			f.ValuesSet("monitoringApplications.enabledApplications", []string{"nats", "redis"})
			f.ValuesSet("monitoringApplications.discovery.enabledApplications", []string{"winword", "nats"})
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("monitoringApplications.internal.enabledApplicationsSummary must be unique sum of two lists", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringApplications.internal.enabledApplicationsSummary").String()).To(MatchJSON(`["nats","redis","winword"]`))
		})
	})

})
