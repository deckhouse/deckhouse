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

/*

User-stories:
1. There are services with label `prometheus.deckhous.io/alertmanager: <prometheus_instance>. Hook must discover them and store to values `prometheus.internal.alertmanagers` in format {"<prometheus_instance>": [{<service_description>}, ...], ...}.
   There is optional annotation `prometheus.deckhouse.io/alertmanager-path-prefix` with default value "/". It must be stored in service description.

*/

package hooks

import (
	"context"

	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: alertmanager discovery", func() {
	const (
		initValuesString       = `{"prometheus": {"internal": {"alertmanagers": {}}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateExternalAlertManagerByAddress = `
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: external-alertmanager
spec:
  external:
    address: http://alerts.mycompany.com
  type: External
`

		stateExternalAlertManagerByService = `
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: external-alertmanager
spec:
  external:
    service:
      name: test
      namespace: test
  type: External
`

		stateDeprecatedLabeledService = `
---
apiVersion: v1
kind: Service
metadata:
  name: deprecatedsvc1
  namespace: myns1
  labels:
    prometheus.deckhouse.io/alertmanager: main
  annotations:
    prometheus.deckhouse.io/alertmanager-path-prefix: /myprefix/
spec:
  ports:
  - name: test
    port: 81
`

		stateInternalAlertManager = `
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: wechat
spec:
  internal:
    receivers:
    - name: wechat-example
      wechatConfigs:
      - apiSecret:
          key: apiSecret
          name: wechat-config
        apiURL: http://wechatserver:8080/
        corpID: wechat-corpid
    route:
      groupBy:
      - job
      groupInterval: 5m
      groupWait: 30s
      receiver: wechat-example
      repeatInterval: 12h
  type: Internal
`
		service = `
piVersion: v1
kind: Service
metadata:
  name: test
  namespace: test
spec:
  ports:
  - name: https
    port: 443
    protocol: TCP
    targetPort: 8443
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "CustomAlertmanager", false)

	Context("Cluster has external CustomAlertManager by address", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateExternalAlertManagerByAddress, 1))
			f.RunHook()
		})
		It("prometheus.internal.alertmanagers.byAddress must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.alertmanagers.byAddress").String()).To(MatchJSON(`[
          {
            "name": "external-alertmanager",
            "scheme": "http",
            "target": "alerts.mycompany.com",
            "basicAuth": {},
            "tlsConfig": {}
          }
        ]`))
		})
	})

	Context("Cluster has external CustomAlertManager by service, corresponding svc is absent", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateExternalAlertManagerByService, 1))
			f.RunHook()
		})

		It("corresponding service absent, hook should fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster has external CustomAlertManager by service, corresponding svc is present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateExternalAlertManagerByService, 1))

			var s v1.Service
			_ = yaml.Unmarshal([]byte(service), &s)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Services("test").
				Create(context.TODO(), &s, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunHook()
		})

		It("corresponding service present, prometheus.internal.alertmanagers.byService must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.alertmanagers.byService").String()).To(MatchJSON(`[
          {
            "resourceName": "external-alertmanager",
            "name": "test",
            "namespace": "test",
            "pathPrefix": "/",
            "port": "https"
          }
        ]`))
		})
	})

	Context("Cluster has external CustomAlertManager by service, corresponding svc is present, and deprecated labeled service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateExternalAlertManagerByService+stateDeprecatedLabeledService, 1))

			var s v1.Service
			_ = yaml.Unmarshal([]byte(service), &s)

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Services("test").
				Create(context.TODO(), &s, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunHook()
		})

		It("corresponding service present, prometheus.internal.alertmanagers.byService must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.alertmanagers.byService").String()).To(MatchJSON(`[
          {
            "resourceName": "external-alertmanager",
            "name": "test",
            "namespace": "test",
            "pathPrefix": "/",
            "port": "https"
          },
          {
            "resourceName": "",
            "name": "deprecatedsvc1",
            "namespace": "myns1",
            "pathPrefix": "/myprefix/",
            "port": "test"
          }
        ]`))
		})
	})

	Context("Cluster has internal CustomAlertManager", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateInternalAlertManager, 1))
			f.RunHook()
		})
		It("prometheus.internal.alertmanagers.internal must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.alertmanagers.internal").String()).To(MatchJSON(`[
          {
			"name": "wechat",
            "receivers": [
              {
                "name": "wechat-example",
                "wechatConfigs": [
                  {
                    "apiSecret": {
                      "key": "apiSecret",
                      "name": "wechat-config"
                    },
                    "apiURL": "http://wechatserver:8080/",
                    "corpID": "wechat-corpid"
                  }
                ]
              }
            ],
            "route": {
              "groupBy": [
                "job"
              ],
              "groupInterval": "5m",
              "groupWait": "30s",
              "receiver": "wechat-example",
              "repeatInterval": "12h"
            }
          }
        ]`))
		})
	})

	Context("Cluster has multiple custom alert managers", func() {
		var alertManagers = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: test-alert-emailer
spec:
  internal:
    receivers:
    - emailConfigs:
      - from: test
        requireTLS: false
        sendResolved: false
        smarthost: test
        to: test@test.ru
      name: test-alert-emailer
    route:
      groupBy:
      - job
      groupInterval: 5m
      groupWait: 30s
      receiver: test-alert-emailer
      repeatInterval: 4h
      routes:
      - matchers:
        - name: namespace
          regex: false
          value: app-airflow
        receiver: test-alert-emailer
  type: Internal
---
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: airflow-alert-emailer
spec:
  internal:
    receivers:
    - emailConfigs:
      - from: test
        requireTLS: false
        sendResolved: false
        smarthost: test
        to: test@test.ru
      name: airflow-alert-emailer
    route:
      groupBy:
      - job
      groupInterval: 5m
      groupWait: 30s
      receiver: airflow-alert-emailer
      repeatInterval: 4h
      routes:
      - matchers:
        - name: namespace
          regex: false
          value: app-airflow
        receiver: airflow-alert-emailer
  type: Internal
`

		const values = `
- name: airflow-alert-emailer
  receivers:
    - emailConfigs:
        - from: test
          requireTLS: false
          sendResolved: false
          smarthost: test
          to: test@test.ru
      name: airflow-alert-emailer
  route:
    groupBy:
      - job
    groupInterval: 5m
    groupWait: 30s
    receiver: airflow-alert-emailer
    repeatInterval: 4h
    routes:
      - matchers:
          - name: namespace
            regex: false
            value: app-airflow
        receiver: airflow-alert-emailer
- name: test-alert-emailer
  receivers:
    - emailConfigs:
        - from: test
          requireTLS: false
          sendResolved: false
          smarthost: test
          to: test@test.ru
      name: test-alert-emailer
  route:
    groupBy:
      - job
    groupInterval: 5m
    groupWait: 30s
    receiver: test-alert-emailer
    repeatInterval: 4h
    routes:
      - matchers:
          - name: namespace
            regex: false
            value: app-airflow
        receiver: test-alert-emailer
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(alertManagers))
			f.RunHook()
		})

		It("prometheus.internal.alertmanagers.internal must contain multiple values", func() {
			Expect(f).To(ExecuteSuccessfully())
			alertmanagers, err := yaml.Marshal(f.ValuesGet("prometheus.internal.alertmanagers.internal").Value())
			Expect(err).To(BeNil())
			Expect(alertmanagers).To(MatchYAML(values))
		})
	})
})
