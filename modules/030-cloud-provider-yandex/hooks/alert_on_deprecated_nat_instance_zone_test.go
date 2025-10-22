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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: alert_on_deprecated_nat_instance_zone ::", func() {
	const (
		metricName                     = "d8_cloud_provider_yandex_nat_instance_zone_deprecated"
		initValuesStringStandardLayout = `
cloudProviderYandex:
  internal:
    providerClusterConfiguration:
      apiVersion: deckhouse.io/v1
      kind: YandexClusterConfiguration
      layout: Standard
      masterNodeGroup:
        instanceClass:
          cores: 4
          etcdDiskSizeGb: 10
          externalIPAddresses:
            - Auto
          imageID: test
          memory: 8192
          platform: standard-v2
        replicas: 1
      nodeNetworkCIDR: 10.100.0.0/21
      provider:
        cloudID: test
        folderID: test
        serviceAccountJSON: |-
          {
            "id": "test"
          }
      sshPublicKey: ssh-rsa test
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      defaultLbTargetGroupNetworkId: test
      internalNetworkIDs:
        - test
      kind: YandexCloudDiscoveryData
      natInstanceName: ""
      natInstanceZone: ""
      region: ru-central1
      routeTableID: test
      shouldAssignPublicIPAddress: false
      zoneToSubnetIdMap:
        ru-central1-a: test
        ru-central1-b: test
        ru-central1-c: test
      zones:
        - ru-central1-a
        - ru-central1-b
        - ru-central1-c
`
		initValuesStringNormalZone = `
cloudProviderYandex:
  internal:
    providerClusterConfiguration:
      apiVersion: deckhouse.io/v1
      kind: YandexClusterConfiguration
      layout: WithNATInstance
      withNATInstance:
        exporterAPIKey: ""
        externalSubnetID: testsubnetid
      masterNodeGroup:
        instanceClass:
          cores: 4
          etcdDiskSizeGb: 10
          externalIPAddresses:
            - Auto
          imageID: test
          memory: 8192
          platform: standard-v2
        replicas: 1
      nodeNetworkCIDR: 10.100.0.0/21
      provider:
        cloudID: test
        folderID: test
        serviceAccountJSON: |-
          {
            "id": "test"
          }
      sshPublicKey: ssh-rsa test
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      defaultLbTargetGroupNetworkId: test
      internalNetworkIDs:
        - test
      kind: YandexCloudDiscoveryData
      natInstanceName: d8-nat-instance
      natInstanceZone: ru-central1-a
      region: ru-central1
      routeTableID: test
      shouldAssignPublicIPAddress: false
      zoneToSubnetIdMap:
        ru-central1-a: test
        ru-central1-b: test
        ru-central1-c: test
      zones:
        - ru-central1-a
        - ru-central1-b
        - ru-central1-c
`
		initValuesStringDeprecatedZone = `
cloudProviderYandex:
  internal:
    providerClusterConfiguration:
      apiVersion: deckhouse.io/v1
      kind: YandexClusterConfiguration
      layout: WithNATInstance
      withNATInstance:
        exporterAPIKey: ""
        externalSubnetID: testsubnetid
      masterNodeGroup:
        instanceClass:
          cores: 4
          etcdDiskSizeGb: 10
          externalIPAddresses:
            - Auto
          imageID: test
          memory: 8192
          platform: standard-v2
        replicas: 1
      nodeNetworkCIDR: 10.100.0.0/21
      provider:
        cloudID: test
        folderID: test
        serviceAccountJSON: |-
          {
            "id": "test"
          }
      sshPublicKey: ssh-rsa test
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      defaultLbTargetGroupNetworkId: test
      internalNetworkIDs:
        - test
      kind: YandexCloudDiscoveryData
      natInstanceName: d8-nat-instance
      natInstanceZone: ru-central1-c
      region: ru-central1
      routeTableID: test
      shouldAssignPublicIPAddress: false
      zoneToSubnetIdMap:
        ru-central1-a: test
        ru-central1-b: test
        ru-central1-c: test
      zones:
        - ru-central1-a
        - ru-central1-b
        - ru-central1-c
`
	)

	standardLayout := HookExecutionConfigInit(initValuesStringStandardLayout, `{}`)
	Context("Without NAT Instance", func() {
		BeforeEach(func() {
			standardLayout.BindingContexts.Set(standardLayout.KubeStateSet(""))
			standardLayout.RunHook()
		})

		It("Should expire metric and successfully executed without any log output", func() {
			Expect(standardLayout).To(ExecuteSuccessfully())

			Expect(string(standardLayout.LoggerOutput.Contents())).To(HaveLen(0))
			m := standardLayout.MetricsCollector.CollectedMetrics()

			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  metricName,
				Action: operation.ActionExpireMetrics,
			}))

		})
	})

	normalZone := HookExecutionConfigInit(initValuesStringNormalZone, `{}`)
	Context("With normal zone", func() {
		BeforeEach(func() {
			normalZone.BindingContexts.Set(normalZone.KubeStateSet(""))
			normalZone.RunHook()
		})

		It("Should expire metric and successfully executed without any log output", func() {
			Expect(normalZone).To(ExecuteSuccessfully())

			Expect(string(normalZone.LoggerOutput.Contents())).To(HaveLen(0))
			m := normalZone.MetricsCollector.CollectedMetrics()

			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  metricName,
				Action: operation.ActionExpireMetrics,
			}))

		})
	})

	deprecatedZone := HookExecutionConfigInit(initValuesStringDeprecatedZone, `{}`)
	Context("With deprecated zone", func() {
		BeforeEach(func() {
			deprecatedZone.BindingContexts.Set(deprecatedZone.KubeStateSet(""))
			deprecatedZone.RunHook()
		})

		It("Should expire metric first and then successfully executed setting metric to 1", func() {
			Expect(deprecatedZone).To(ExecuteSuccessfully())

			Expect(string(deprecatedZone.LoggerOutput.Contents())).To(HaveLen(0))
			m := deprecatedZone.MetricsCollector.CollectedMetrics()

			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  metricName,
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   metricName,
				Value:  ptr.To(1.0),
				Labels: map[string]string{"name": "d8-nat-instance", "zone": "ru-central1-c"},
				Action: operation.ActionGaugeSet,
				Group:  metricName,
			}))

		})
	})
})
