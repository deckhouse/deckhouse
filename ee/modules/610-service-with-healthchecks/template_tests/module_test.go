/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
discovery:
  clusterControlPlaneIsHighlyAvailable: true
  d8SpecificNodeCountByRole:
      worker: 3
      master: 1
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler-crd", "service-with-healthchecks"]
`
	goodModuleValuesA = `
debug: true
`
	desiredDaemonSetContainerSpecA = `
- args:
  - --debugging=true
  env:
  - name: NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName
  image: registry.example.com@imageHash-serviceWithHealthchecks-agent
  imagePullPolicy: IfNotPresent
  livenessProbe:
    failureThreshold: 3
    httpGet:
      path: /healthz
      port: 9873
    initialDelaySeconds: 10
    periodSeconds: 10
    successThreshold: 1
    timeoutSeconds: 1
  name: agent
  readinessProbe:
    failureThreshold: 3
    httpGet:
      path: /readyz
      port: 9873
    initialDelaySeconds: 10
    periodSeconds: 10
    successThreshold: 1
    timeoutSeconds: 1
  resources:
    requests:
      ephemeral-storage: 50Mi
  securityContext:
    readOnlyRootFilesystem: true
    allowPrivilegeEscalation: false
    capabilities:
      drop:
        - ALL
    runAsUser:   64535
    runAsGroup:  64535
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
- args:
  - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):8383
  - --v=2
  - --logtostderr=true
  - --stale-cache-interval=1h30m
  - --livez-path=/livez
  env:
  - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
    valueFrom:
      fieldRef:
        fieldPath: status.podIP
  - name: KUBE_RBAC_PROXY_CONFIG
    value: |
      upstreams:
      - upstream: http://127.0.0.1:9874/metrics
        path: /metrics
        authorization:
          resourceAttributes:
            namespace: d8-service-with-healthchecks
            apiGroup: apps
            apiVersion: v1
            resource: daemonsets
            subresource: prometheus-metrics
            name: agent
  image: registry.example.com@imageHash-common-kubeRbacProxy
  name: kube-rbac-proxy
  ports:
  - containerPort: 8383
    name: https-metrics
  livenessProbe:
    httpGet:
      path: /livez
      port: 8383
      scheme: HTTPS
  readinessProbe:
    httpGet:
      path: /livez
      port: 8383
      scheme: HTTPS
  resources:
    requests:
      ephemeral-storage: 50Mi
  securityContext:
    readOnlyRootFilesystem: true
    allowPrivilegeEscalation: false
    capabilities:
      drop:
        - ALL
    runAsUser:   64535
    runAsGroup:  64535
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
`
)

var _ = Describe("Module :: serviceWithHealthchecks :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Good test A", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("serviceWithHealthchecks", goodModuleValuesA)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ds := f.KubernetesResource("DaemonSet", "d8-service-with-healthchecks", "agent")
			Expect(ds.Exists()).To(BeTrue())

			Expect(ds.Field("spec.template.spec.containers").String()).To(MatchYAML(desiredDaemonSetContainerSpecA))
		})
	})
})
