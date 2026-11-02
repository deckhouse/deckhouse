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
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: quota_ledger_janitor ::", func() {
	state := func(renew time.Time) string {
		return fmt.Sprintf(`
---
apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  name: multitenancy-manager-quota-ledger
  namespace: d8-multitenancy-manager
spec:
  holderIdentity: mtm-a_r1_v1
  renewTime: %s
---
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: multitenancy-manager-quota-ledger
  namespace: d8-multitenancy-manager
addressType: IPv4
endpoints:
- addresses: ["10.1.0.5"]
  targetRef:
    kind: Pod
    name: mtm-a
    namespace: d8-multitenancy-manager
`, renew.UTC().Format("2006-01-02T15:04:05.000000Z"))
	}

	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{}}}`, `{}`)

	Context("a lease renewed a moment ago", func() {
		BeforeEach(func() {
			quotaLedgerObservation.key = ""
			f.KubeStateSet(state(time.Now()))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * * *"))
			f.RunHook()
		})

		It("leaves the address of the leader", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("EndpointSlice", "d8-multitenancy-manager", "multitenancy-manager-quota-ledger").Field("endpoints").Array()).To(HaveLen(1))
		})
	})

	Context("a lease that stands still between runs by the clock of Deckhouse", func() {
		var renew, clock time.Time
		run := func() {
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * * *"))
			f.RunHook()
		}
		BeforeEach(func() {
			clock, renew = time.Now(), time.Now()
			quotaLedgerNow = func() time.Time { return clock }
			quotaLedgerObservation.key = ""
			f.KubeStateSet(state(renew))
		})
		AfterEach(func() {
			quotaLedgerNow = time.Now
		})

		It("leaves the slice on the first run after a start of Deckhouse", func() {
			run()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("EndpointSlice", "d8-multitenancy-manager", "multitenancy-manager-quota-ledger").Field("endpoints").Array()).To(HaveLen(1))
		})

		It("empties the slice after 10 seconds, however the clocks of the masters disagree", func() {
			run()
			clock = clock.Add(5 * time.Second)
			run()
			Expect(f.KubernetesResource("EndpointSlice", "d8-multitenancy-manager", "multitenancy-manager-quota-ledger").Field("endpoints").Array()).To(HaveLen(1))
			clock = clock.Add(5 * time.Second)
			run()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("EndpointSlice", "d8-multitenancy-manager", "multitenancy-manager-quota-ledger").Field("endpoints").Array()).To(BeEmpty())
		})

		It("leaves the slice of a lease that is renewed", func() {
			for range 3 {
				run()
				clock, renew = clock.Add(5*time.Second), renew.Add(5*time.Second)
				f.KubeStateSet(state(renew))
			}
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("EndpointSlice", "d8-multitenancy-manager", "multitenancy-manager-quota-ledger").Field("endpoints").Array()).To(HaveLen(1))
		})
	})

	Context("a lease not renewed for a minute", func() {
		BeforeEach(func() {
			quotaLedgerObservation.key = ""
			f.KubeStateSet(state(time.Now().Add(-time.Minute)))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * * *"))
			f.RunHook()
		})

		It("empties the slice, so the calls fail at once", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("EndpointSlice", "d8-multitenancy-manager", "multitenancy-manager-quota-ledger").Field("endpoints").Array()).To(BeEmpty())
		})
	})
})

// A watch of Deckhouse that lags makes a renewed lease look still: the lease read again from the API
// server keeps the address of a leader that renewed it since.
func TestQuotaLedgerRenewedSince(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	seen := quotaLedgerLease{Holder: "mtm-a_r1_v1", Renew: now.Add(-20 * time.Second)}
	tests := []struct {
		name     string
		live     quotaLedgerLease
		expected bool
	}{
		{name: "renewed since", live: quotaLedgerLease{Holder: "mtm-a_r1_v1", Renew: now.Add(-time.Second)}, expected: true},
		{name: "taken by another replica", live: quotaLedgerLease{Holder: "mtm-b_r2_v1", Renew: now.Add(-time.Second)}, expected: true},
		{name: "the same renewal", live: seen},
		{name: "released", live: quotaLedgerLease{Renew: now.Add(-time.Second)}},
		{name: "renewed since, long ago", live: quotaLedgerLease{Holder: "mtm-b_r2_v1", Renew: now.Add(-time.Minute)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quotaLedgerRenewedSince(seen, tt.live, now); got != tt.expected {
				t.Errorf("quotaLedgerRenewedSince() = %v, want %v", got, tt.expected)
			}
		})
	}
}
