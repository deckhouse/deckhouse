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
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: grafana notification channels ::", func() {
	const (
		testAlertsChannelYAML = `
apiVersion: deckhouse.io/v1alpha1
kind: GrafanaAlertsChannel
metadata:
  name: test
spec:
  type: PrometheusAlertManager
  description: Another text
  isDefault: true
  disableResolveMessage: false
  alertManager:
    address: "http://test-alert-manager-url"
    auth:
      basic:
        username: user
        password: password
`
		testAlertsChanelUpdatedYAML = `
apiVersion: deckhouse.io/v1alpha1
kind: GrafanaAlertsChannel
metadata:
  name: test
spec:
  type: PrometheusAlertManager
  description: SomeText
  isDefault: false
  disableResolveMessage: true
  alertManager:
    address: "https://new-test-url"
    auth:
      basic:
        username: user
        password: new-password
`
		testAlertsChannelWithoutAuthYAML = `
apiVersion: deckhouse.io/v1alpha1
kind: GrafanaAlertsChannel
metadata:
  name: another
spec:
  type: PrometheusAlertManager
  alertManager:
    address: "https://another-url"
`
	)

	var (
		testAlertsChannel = GrafanaAlertsChannel{
			OrgID:                 1,
			Type:                  "prometheus-alertmanager",
			Name:                  "test",
			UID:                   "test",
			IsDefault:             true,
			DisableResolveMessage: false,
			SendReminder:          false,
			Frequency:             time.Duration(0),
			Settings: map[string]interface{}{
				"url":           "http://test-alert-manager-url",
				"basicAuthUser": "user",
			},
			SecureSettings: map[string]interface{}{
				"basicAuthPassword": "password",
			},
		}

		testAlertsChanelUpdated = GrafanaAlertsChannel{
			OrgID:                 1,
			Type:                  "prometheus-alertmanager",
			Name:                  "test",
			UID:                   "test",
			IsDefault:             false,
			DisableResolveMessage: true,
			SendReminder:          false,
			Frequency:             time.Duration(0),
			Settings: map[string]interface{}{
				"url":           "https://new-test-url",
				"basicAuthUser": "user",
			},
			SecureSettings: map[string]interface{}{
				"basicAuthPassword": "new-password",
			},
		}

		testAlertsChannelWithoutAuth = GrafanaAlertsChannel{
			OrgID:                 1,
			Type:                  "prometheus-alertmanager",
			Name:                  "another",
			UID:                   "another",
			IsDefault:             false,
			DisableResolveMessage: false,
			SendReminder:          false,
			Frequency:             time.Duration(0),
			Settings: map[string]interface{}{
				"url": "https://another-url",
			},
			SecureSettings: make(map[string]interface{}),
		}
	)

	f := HookExecutionConfigInit(`
{
  "global": {
    "enabledModules": [],
    "discovery":{
		"clusterDomain": "cluster.my"
    }
  },
  "prometheus":{
    "internal":{
      "grafana":{}
    }
 }
}`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "GrafanaAlertsChannel", false)

	assertChannelsInValues := func(f *HookExecutionConfig, expectChannels []GrafanaAlertsChannel) {
		cfgJSON := f.ValuesGet("prometheus.internal.grafana.alertsChannelsConfig").Raw

		var cfg GrafanaAlertsChannelsConfig
		err := json.Unmarshal([]byte(cfgJSON), &cfg)
		Expect(err).ToNot(HaveOccurred())

		channels := cfg.Notifiers

		Expect(channels).To(HaveLen(len(expectChannels)))

		nameToChannel := make(map[string]GrafanaAlertsChannel)
		for _, c := range expectChannels {
			nameToChannel[c.UID] = c
		}

		for _, c := range channels {
			expected, ok := nameToChannel[c.UID]

			Expect(ok).To(BeTrue())
			Expect(expected).To(Equal(*c))
		}
	}

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should store config in values", func() {
			Expect(f).To(ExecuteSuccessfully())

			exists := f.ValuesGet("prometheus.internal.grafana.alertsChannelsConfig").Exists()
			Expect(exists).To(BeTrue())

			exists = f.ValuesGet("prometheus.internal.grafana.alertsChannelsConfig.notifiers").Exists()
			Expect(exists).To(BeTrue())
		})

		It("Does not set any channels in values", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertChannelsInValues(f, make([]GrafanaAlertsChannel, 0))
		})

		Context("Add channel", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(testAlertsChannelYAML, 1))
				f.RunHook()
			})

			It("Should store channel in values", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertChannelsInValues(f, []GrafanaAlertsChannel{testAlertsChannel})
			})

			Context("Add another channel without auth", func() {
				BeforeEach(func() {
					JoinKubeResourcesAndSet(f, testAlertsChannelYAML, testAlertsChannelWithoutAuthYAML)
					f.RunHook()
				})

				It("Should add new channel in values", func() {
					Expect(f).To(ExecuteSuccessfully())

					assertChannelsInValues(f, []GrafanaAlertsChannel{testAlertsChannel, testAlertsChannelWithoutAuth})
				})

				Context("Deleting channel without auth", func() {
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(testAlertsChannelYAML, 0))
						f.RunHook()
					})

					It("Should only delete channel without auth", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertChannelsInValues(f, []GrafanaAlertsChannel{testAlertsChannel})
					})
				})

				Context("Updating test channel", func() {
					BeforeEach(func() {
						JoinKubeResourcesAndSet(f, testAlertsChanelUpdatedYAML, testAlertsChannelWithoutAuthYAML)
						f.RunHook()
					})

					It("Should only update test channel in values", func() {
						Expect(f).To(ExecuteSuccessfully())

						assertChannelsInValues(f, []GrafanaAlertsChannel{testAlertsChanelUpdated, testAlertsChannelWithoutAuth})
					})
				})
			})
		})
	})

	Context("Alerts channels in cluster", func() {
		BeforeEach(func() {
			JoinKubeResourcesAndSet(f, testAlertsChannelYAML, testAlertsChannelWithoutAuthYAML)
			f.RunHook()
		})

		It("Should store all alerts channel into values", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertChannelsInValues(f, []GrafanaAlertsChannel{testAlertsChannel, testAlertsChannelWithoutAuth})
		})
	})
})
