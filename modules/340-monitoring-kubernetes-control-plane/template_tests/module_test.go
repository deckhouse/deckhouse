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

const globalValues = `
  enabledModules: ["vertical-pod-autoscaler-crd"]
  modulesImages:
    registry: registry.flant.com
    registryDockercfg: cfg
    tags:
      monitoringKubernetesControlPlane:
        proxy: tagstring
  discovery:
    clusterVersion: 1.15.4
`

var _ = Describe("Module :: monitoring-kubernetes-control-plane :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("All components are allocated ThroughNode and authenticationMethod is Certificate", func() {
		BeforeEach(func() {
			moduleValues := `
internal:
  kubeApiserver: # accessType "ThroughNode" and authenticationMethod "Certificate" for all components; all variants of KubeEtcd
    accessType: ThroughNode
    throughNode:
      authenticationMethod: Certificate
    metricsPath: /metrics
    clientCertificate:
      clientCrt: mycert
      clientKey: mykey
  kubeControllerManager:
    accessType: ThroughNode
    throughNode:
      authenticationMethod: Certificate
    metricsPath: /metrics
    clientCertificate:
      clientCrt: mycert
      clientKey: mykey
  kubeScheduler:
    accessType: ThroughNode
    throughNode:
      authenticationMethod: Certificate
    metricsPath: /metrics
    clientCertificate:
      clientCrt: mycert
      clientKey: mykey
  kubeEtcd:
  - name: main0
    accessType: ThroughNode
    throughNode:
      authenticationMethod: Certificate
    clientCertificate:
      clientCrt: mycert
      clientKey: mykey
  - name: main1
    accessType: ThroughNode
    throughNode:
      authenticationMethod: HostPathCertificate
  - name: main2
    accessType: Pod
    pod:
      authenticationMethod: Certificate
      podSelector:
        popopo: qqq
      podNamespace: kuku
      port: 4001
    clientCertificate:
      clientCrt: mycert
      clientKey: mykey

  proxy:
    instances:
      aaaaaa:
        nodeSelector:
          aaa: aaa
        components:
        - name: KubeApiserver
          values:
            accessType: ThroughNode
            metricsPath: /metrics
            throughNode:
              authenticationMethod: Certificate
              localPort: 6443
              proxyListenPort: 10361
              scheme: https
        - name: KubeControllerManager
          values:
            accessType: ThroughNode
            metricsPath: /metrics
            throughNode:
              authenticationMethod: Certificate
              localPort: 10252
              proxyListenPort: 10362
              scheme: http
        - name: KubeScheduler
          values:
            accessType: ThroughNode
            metricsPath: /metrics
            throughNode:
              authenticationMethod: Certificate
              localPort: 10251
              proxyListenPort: 10363
              scheme: http
        - name: KubeEtcdMain0
          values:
            accessType: ThroughNode
            metricsPath: /metrics
            name: main0
            throughNode:
              authenticationMethod: HostPathCertificate
              hostPathCertificate: /etc/client.crt
              hostPathCertificateKey: /etc/client.key
              localPort: 2379
              proxyListenPort: 10370
        - name: KubeEtcdMain1
          values:
            accessType: ThroughNode
            metricsPath: /metrics
            name: main1
            throughNode:
              authenticationMethod: Certificate
              localPort: 2379
              proxyListenPort: 10371
`

			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("monitoringKubernetesControlPlane", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			sa := f.KubernetesResource("ServiceAccount", "d8-monitoring", "control-plane-proxy")
			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:monitoring-kubernetes-control-plane:control-plane-proxy")
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-monitoring", "control-plane-proxy-aaaaaa")
			ds := f.KubernetesResource("DaemonSet", "d8-monitoring", "control-plane-proxy-aaaaaa")

			pmKubeApiserver := f.KubernetesResource("PodMonitor", "d8-monitoring", "kube-apiserver")
			pmKubeControllerManager := f.KubernetesResource("PodMonitor", "d8-monitoring", "kube-apiserver")
			pmKubeScheduler := f.KubernetesResource("PodMonitor", "d8-monitoring", "kube-apiserver")
			pmKubeEtcdMain0 := f.KubernetesResource("PodMonitor", "d8-monitoring", "kube-etcd-main0")
			pmKubeEtcdMain1 := f.KubernetesResource("PodMonitor", "d8-monitoring", "kube-etcd-main1")
			smKubeEtcdMain2 := f.KubernetesResource("ServiceMonitor", "d8-monitoring", "kube-etcd-main2")

			secretKubeApiserver := f.KubernetesResource("Secret", "d8-monitoring", "monitoring-control-plane-kube-apiserver-client-cert")
			secretKubeControllerManager := f.KubernetesResource("Secret", "d8-monitoring", "monitoring-control-plane-kube-controller-manager-client-cert")
			secretKubeScheduler := f.KubernetesResource("Secret", "d8-monitoring", "monitoring-control-plane-kube-scheduler-client-cert")
			secretKubeEtcdMain0 := f.KubernetesResource("Secret", "d8-monitoring", "monitoring-control-plane-kube-etcd-client-cert-main0")
			secretKubeEtcdMain1 := f.KubernetesResource("Secret", "d8-monitoring", "monitoring-control-plane-kube-etcd-client-cert-main1")
			secretKubeEtcdMain2 := f.KubernetesResource("Secret", "d8-monitoring", "monitoring-control-plane-kube-etcd-client-cert-main2")

			Expect(sa.Exists()).To(BeTrue())
			Expect(crb.Exists()).To(BeTrue())
			Expect(vpa.Exists()).To(BeTrue())
			Expect(ds.Exists()).To(BeTrue())

			Expect(ds.Field("spec.template.spec.containers").Array()).To(HaveLen(5))
			Expect(ds.Field("spec.template.spec.volumes").Array()).To(HaveLen(6))

			Expect(pmKubeApiserver.Exists()).To(BeTrue())
			Expect(pmKubeControllerManager.Exists()).To(BeTrue())
			Expect(pmKubeScheduler.Exists()).To(BeTrue())
			Expect(pmKubeEtcdMain0.Exists()).To(BeTrue())
			Expect(pmKubeEtcdMain1.Exists()).To(BeTrue())
			Expect(smKubeEtcdMain2.Exists()).To(BeTrue())

			Expect(secretKubeApiserver.Exists()).To(BeTrue())
			Expect(secretKubeControllerManager.Exists()).To(BeTrue())
			Expect(secretKubeScheduler.Exists()).To(BeTrue())
			Expect(secretKubeEtcdMain0.Exists()).To(BeTrue())
			Expect(secretKubeEtcdMain1.Exists()).To(BeFalse()) // authenticationMethod is HostPathCertificate
			Expect(secretKubeEtcdMain2.Exists()).To(BeTrue())

			Expect(secretKubeApiserver.Field("data").String()).To(MatchJSON(`{"client.crt": "bXljZXJ0", "client.key": "bXlrZXk="}`))
			Expect(secretKubeControllerManager.Field("data").String()).To(MatchJSON(`{"client.crt": "bXljZXJ0", "client.key": "bXlrZXk="}`))
			Expect(secretKubeScheduler.Field("data").String()).To(MatchJSON(`{"client.crt": "bXljZXJ0", "client.key": "bXlrZXk="}`))
			Expect(secretKubeEtcdMain0.Field("data").String()).To(MatchJSON(`{"client.crt": "bXljZXJ0", "client.key": "bXlrZXk="}`))
			Expect(secretKubeEtcdMain2.Field("data").String()).To(MatchJSON(`{"client.crt": "bXljZXJ0", "client.key": "bXlrZXk="}`))
		})
	})
})
