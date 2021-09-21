// Copyright 2021 Flant JSC
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

/*

User-stories:
1. There is CM d8-monitoring/prometheus-scrape-interval with prometheus scrape interval. Hook must store it to `global.discovery.prometheusScrapeInterval`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: prometheus_scrape_interval ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		prometheusScrapeInterval30sCM = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-scrape-interval
  namespace: d8-monitoring
data:
  scrapeInterval: 30s
`
		prometheusScrapeInterval5sCM = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-scrape-interval
  namespace: d8-monitoring
data:
  scrapeInterval: 5s`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.prometheusScrapeInterval").String()).To(Equal("30"))
		})

		Context("CM d8-monitoring/prometheus-scrape-interval created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(prometheusScrapeInterval30sCM))
				f.RunHook()
			})

			It("global.discovery.prometheusScrapeInterval must be '30'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.prometheusScrapeInterval").String()).To(Equal("30"))
			})

			Context("CM d8-monitoring/prometheus-scrape-interval deleted", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(``))
					f.RunHook()
				})

				It("Hook must execute successfully and global.discovery.prometheusScrapeInterval must equals to default value '30'", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.discovery.prometheusScrapeInterval").String()).To(Equal("30"))
				})
			})
		})
	})

	Context("CM d8-monitoring/prometheus-scrape-interval exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(prometheusScrapeInterval30sCM))
			f.RunHook()
		})

		It("global.discovery.prometheusScrapeInterval must be '30'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.prometheusScrapeInterval").String()).To(Equal("30"))
		})

		Context("CM d8-monitoring/prometheus-scrape-interval modified", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(prometheusScrapeInterval5sCM))
				f.RunHook()
			})

			It("global.discovery.prometheusScrapeInterval must be '5'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.prometheusScrapeInterval").String()).To(Equal("5"))
			})

			Context("CM d8-monitoring/prometheus-scrape-interval deleted", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(``))
					f.RunHook()
				})

				It("global.discovery.prometheusScrapeInterval must equals to default value '30'", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.discovery.prometheusScrapeInterval").String()).To(Equal("30"))
				})
			})
		})
	})
})
