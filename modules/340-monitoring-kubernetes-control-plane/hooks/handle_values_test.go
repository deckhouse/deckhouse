package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-kubernetes-control-plane :: hooks :: handle_values ::", func() {
	initValues := `
{
  "monitoringKubernetesControlPlane": {
    "defaults": {
      "kubeApiserver": {
        "accessType": "DefaultService",
        "pod": {
          "podSelector": {}
        },
        "metricsPath": "/metrics",
        "throughNode": {
          "nodeSelector": {
            "node-role.kubernetes.io/master": ""
          },
          "proxyListenPort": 10361
        }
      },
      "kubeControllerManager": {
        "accessType": "ThroughNode",
        "pod": {
          "podSelector": {}
        },
        "metricsPath": "/metrics",
        "throughNode": {
          "authenticationMethod": "None",
          "localPort": 10252,
          "nodeSelector": {
            "node-role.kubernetes.io/master": ""
          },
          "proxyListenPort": 10362,
          "scheme": "http"
        }
      },
      "kubeEtcd": {
        "accessType": "ThroughNode",
        "pod": {
          "podSelector": {}
        },
        "metricsPath": "/metrics",
        "throughNode": {
          "authenticationMethod": "HostPathCertificate",
          "hostPathCertificate": "/etc/kubernetes/pki/apiserver-etcd-client.crt",
          "hostPathCertificateKey": "/etc/kubernetes/pki/apiserver-etcd-client.key",
          "localPort": 2379,
          "nodeSelector": {
            "node-role.kubernetes.io/master": ""
          },
        }
      },
      "kubeScheduler": {
        "accessType": "ThroughNode",
        "pod": {
          "podSelector": {}
        },
        "metricsPath": "/metrics",
        "throughNode": {
          "authenticationMethod": "None",
          "localPort": 10251,
          "nodeSelector": {
            "node-role.kubernetes.io/master": ""
          },
          "proxyListenPort": 10363,
          "scheme": "http"
        }
      }
    },
    "discovery": {
      "kubeApiserver": {
        "pod": {},
        "throughNode": {}
      },
      "kubeControllerManager": {
        "pod": {},
        "throughNode": {}
      },
      "kubeEtcd": {
        "pod": {},
        "throughNode": {}
      },
      "kubeEtcdEvents": {
        "pod": {},
        "throughNode": {}
      },
      "kubeScheduler": {
        "pod": {},
        "throughNode": {}
      }
    },
    "internal": {
      "kubeApiserver": {},
      "kubeControllerManager": {},
      "kubeEtcd": [],
      "kubeScheduler": {},
      "proxy": {}
    }
  }
}
`
	f := HookExecutionConfigInit(initValues, `{}`)

	Context("Nothing configured, Nothing discovered, Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail, component values must be filled from defaults", func() {
			Expect(f).To(ExecuteSuccessfully())
			mergedKubeApiserver := `
accessType: DefaultService
throughNode:
  nodeSelector:
    node-role.kubernetes.io/master: ""
  proxyListenPort: 10361
pod:
  podSelector: {}
metricsPath: /metrics`

			mergedKubeControllerManager := `
accessType: ThroughNode
throughNode:
  nodeSelector:
    node-role.kubernetes.io/master: ""
  localPort: 10252
  scheme: http
  authenticationMethod: None
  proxyListenPort: 10362
pod:
  podSelector: {}
metricsPath: /metrics`

			mergedKubeScheduler := `
accessType: ThroughNode
throughNode:
  nodeSelector:
    node-role.kubernetes.io/master: ""
  localPort: 10251
  scheme: http
  authenticationMethod: None
  proxyListenPort: 10363
pod:
  podSelector: {}
metricsPath: /metrics`

			mergedKubeEtcd := `
- name: main
  accessType: ThroughNode
  throughNode:
    nodeSelector:
      node-role.kubernetes.io/master: ""
    localPort: 2379
    authenticationMethod: HostPathCertificate
    hostPathCertificate: /etc/kubernetes/pki/apiserver-etcd-client.crt
    hostPathCertificateKey: /etc/kubernetes/pki/apiserver-etcd-client.key
    proxyListenPort: 10370
  pod:
    podSelector: {}
  metricsPath: /metrics`

			mergedProxy := `
instances:
  425f55b4:
    components:
    - name: KubeControllerManager
      values:
        accessType: ThroughNode
        pod:
          podSelector: {}
        metricsPath: /metrics
        throughNode:
          authenticationMethod: None
          localPort: 10252
          nodeSelector:
            node-role.kubernetes.io/master: ""
          proxyListenPort: 10362
          scheme: http
    - name: KubeScheduler
      values:
        accessType: ThroughNode
        pod:
          podSelector: {}
        metricsPath: /metrics
        throughNode:
          authenticationMethod: None
          localPort: 10251
          nodeSelector:
            node-role.kubernetes.io/master: ""
          proxyListenPort: 10363
          scheme: http
    - name: KubeEtcdMain
      values:
        accessType: ThroughNode
        pod:
          podSelector: {}
        metricsPath: /metrics
        name: main
        throughNode:
          authenticationMethod: HostPathCertificate
          hostPathCertificate: /etc/kubernetes/pki/apiserver-etcd-client.crt
          hostPathCertificateKey: /etc/kubernetes/pki/apiserver-etcd-client.key
          localPort: 2379
          nodeSelector:
            node-role.kubernetes.io/master: ""
          proxyListenPort: 10370
    nodeSelector:
      node-role.kubernetes.io/master: ""
`

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeApiserver").String()).To(MatchYAML(mergedKubeApiserver))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeControllerManager").String()).To(MatchYAML(mergedKubeControllerManager))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeScheduler").String()).To(MatchYAML(mergedKubeScheduler))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeEtcd").String()).To(MatchYAML(mergedKubeEtcd))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.proxy").String()).To(MatchYAML(mergedProxy))

		})
	})

	Context("KubeScheduler configuration: {InsideKuberneters, Certificate}, Nothing discovered, Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.accessType", "Pod")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.pod.authenticationMethod", "Certificate")

			f.RunHook()
		})

		It("Hook must fail with message", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: monitoringKubernetesControlPlane.kubeScheduler.pod.certificateSecret is mandatory when accessType is 'Pod' and authenticationMethod is 'Certificate'`))
		})
	})

	Context("KubeScheduler configuration: {InsideKuberneters, Certificate, certificateSecret}, Nothing discovered, Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.accessType", "Pod")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.pod.authenticationMethod", "Certificate")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.pod.certificateSecret", "kube-scheduler-client")

			f.RunHook()
		})

		It("Hook must fail with message", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: there isn't a secret 'kube-scheduler-client' with 'client.crt' and 'client.key' data in ns d8-system to handle authentication method 'Certificate' for component 'KubeScheduler' with accessType 'Pod'.`))

		})
	})

	Context("KubeScheduler configuration: {InsideKuberneters, Certificate, certificateSecret}, Nothing discovered, Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.accessType", "Pod")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.pod.authenticationMethod", "Certificate")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.pod.certificateSecret", "kube-scheduler-client")

			f.RunHook()
		})

		It("Hook must fail with message", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: there isn't a secret 'kube-scheduler-client' with 'client.crt' and 'client.key' data in ns d8-system to handle authentication method 'Certificate' for component 'KubeScheduler' with accessType 'Pod'.`))
		})
	})

	Context("KubeScheduler configuration: {InsideKuberneters, Certificate, certificateSecret}, Nothing discovered, Cluster is empty", func() {
		BeforeEach(func() {
			kubeState := `
apiVersion: v1
kind: Secret
metadata:
  name: kube-scheduler-client
  namespace: d8-system
data:
  client.crt: YWJj # abc
  client.key: eHl6 # xyz
`
			f.BindingContexts.Set(f.KubeStateSet(kubeState))
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.accessType", "Pod")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.pod.authenticationMethod", "Certificate")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.pod.certificateSecret", "kube-scheduler-client")

			f.RunHook()
		})

		It("Hook not fail and secret must be placed to internal.kubeScheduler.clientCertificate", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeScheduler.clientCertificate.clientCrt").String()).To(Equal("abc"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeScheduler.clientCertificate.clientKey").String()).To(Equal("xyz"))
		})
	})

	Context("Some data discovered, Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSetFromYaml("monitoringKubernetesControlPlane.discovery.kubeControllerManager.throughNode.nodeSelector", []byte(`{"abc": "xyz"}`))

			f.RunHook()
		})

		It("Hook not fail and some values should be stored from discovery", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeControllerManager.throughNode.nodeSelector").String()).To(MatchJSON(`{"abc": "xyz"}`))
		})
	})

	Context("KubeApiserver configured as ThroughNode, nothing discovered, Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))

			// Getting rid of KubeControllerManager and KubeEtcd, only KubeScheduler left
			f.ValuesSet("monitoringKubernetesControlPlane.kubeControllerManager.accessType", "Extra")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeEtcd.accessType", "Extra")

			f.ValuesSet("monitoringKubernetesControlPlane.kubeApiserver.accessType", "ThroughNode")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeApiserver.throughNodeKubernetes.authenticationMethod", "ProxyServiceAccount")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeApiserver.throughNode.nodeSelector", []byte(`{"abc": "xyz"}`))

			f.RunHook()
		})

		It("Hook not fail and two proxy instances should appear", func() {
			Expect(f).To(ExecuteSuccessfully())

			mergedProxy := `
instances:
  425f55b4:
    components:
    - name: KubeScheduler
      values:
        accessType: ThroughNode
        pod:
          podSelector: {}
        metricsPath: /metrics
        throughNode:
          authenticationMethod: None
          localPort: 10251
          nodeSelector:
            node-role.kubernetes.io/master: ""
          proxyListenPort: 10363
          scheme: http
    nodeSelector:
      node-role.kubernetes.io/master: ""
  9dc5c6b4:
    components:
    - name: KubeApiserver
      values:
        accessType: ThroughNode
        pod:
          podSelector: {}
        metricsPath: /metrics
        throughNode:
          nodeSelector:
            abc: xyz
          proxyListenPort: 10361
        throughNodeKubernetes:
          authenticationMethod: ProxyServiceAccount
    nodeSelector:
      abc: xyz
`
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.proxy").String()).To(MatchYAML(mergedProxy))
		})
	})

	Context("kubeEtcdAdditionalInstances are badly configured, Cluster is empty", func() {
		BeforeEach(func() {
			additionalKubeEtcd := `
- {}
- name: superlongname
- name: lack0
- name: lack1
  accessType: Pod
- name: lack2
  accessType: Pod
  pod:
    podSelector:
      qqq: zzz
- name: lack3
  accessType: Pod
  pod:
    podSelector:
      qqq: zzz
    podNamespace: kuku
- name: lack4
  accessType: Pod
  pod:
    podSelector:
      qqq: zzz
    podNamespace: kuku
    certificateSecret: lack4-secret
- name: lack5
  accessType: ThroughNode
- name: lack6
  accessType: ThroughNode
  throughNode:
    nodeSelector:
      xxx: yyy
- name: lack7
  accessType: ThroughNode
  throughNode:
    nodeSelector:
      xxx: yyy
    localPort: 4242
- name: lack8
  accessType: ThroughNode
  throughNode:
    nodeSelector:
      xxx: yyy
    localPort: 4242
    authenticationMethod: Certificate
- name: lack9
  accessType: ThroughNode
  throughNode:
    nodeSelector:
      xxx: yyy
    localPort: 4242
    authenticationMethod: Certificate
    certificateSecret: lack9-secret
- name: lack10
  accessType: ThroughNode
  throughNode:
    nodeSelector:
      xxx: yyy
    localPort: 4242
    authenticationMethod: HostPathCertificate
`

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSetFromYaml("monitoringKubernetesControlPlane.kubeEtcdAdditionalInstances", []byte(additionalKubeEtcd))
			f.RunHook()
		})

		It("Hook must fail with lots of errors", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))

			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: name is mandatory for additional kube-etcd instances`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: additional kube-etcd instance name can't be larger than 12 chars \(name 'superlongname' is bad\)`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: accessType is mandatory for additional kube-etcd instance 'lack0'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: pod.podSelector is mandatory for additional kube-etcd instance 'lack1' because of accessType is 'Pod'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: pod.podNamespace is mandatory for additional kube-etcd instance 'lack2' because of accessType is 'Pod'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: pod.certificateSecret is mandatory for additional kube-etcd instance 'lack3' because of accessType is 'Pod'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: there isn't a secret 'lack4-secret' with 'client.crt' and 'client.key' data in ns d8-system to handle authentication method 'Certificate' for additional kube-etcd instance 'lack4' with accessType 'Pod'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: throughNode.nodeSelector is mandatory for additional kube-etcd instance 'lack5' because of accessType is 'ThroughNode'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: throughNode.localPort is mandatory for additional kube-etcd instance 'lack6' because of accessType is 'ThroughNode'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: throughNode.authenticationMethod is mandatory for additional kube-etcd instance 'lack7' because of accessType is 'ThroughNode'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: throughNode.certificateSecret is mandatory for additional kube-etcd instance 'lack8' because of accessType is 'ThroughNode' and authenticationMethod is 'Certificate'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: there isn't a secret 'lack9-secret' with 'client.crt' and 'client.key' data in ns d8-system to handle authentication method 'Certificate' for additional kube-etcd instance 'lack9' with accessType 'ThroughNode'`))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: throughNode.hostPathCertificate and throughNode.hostPathCertificateKey are mandatory for additional kube-etcd instance 'lack10' because of accessType is 'ThroughNode' and authenticationMethod is 'HostPathCertificate'`))
		})
	})

	Context("Single kubeEtcdAdditionalInstances is configured, Cluster has clientCertificate-secret for him", func() {
		BeforeEach(func() {
			additionalKubeEtcd := `
- name: nice0
  accessType: Pod
  pod:
    podSelector:
      qqq: zzz
    podNamespace: kuku
    certificateSecret: nice0-secret
- name: nice1
  accessType: ThroughNode
  throughNode:
    nodeSelector:
      xxx: yyy
    localPort: 4242
    authenticationMethod: Certificate
    certificateSecret: nice1-secret
`
			kubeState := `
---
apiVersion: v1
kind: Secret
metadata:
  name: nice0-secret
  namespace: d8-system
data:
  client.crt: YWJj # abc
  client.key: eHl6 # xyz
---
apiVersion: v1
kind: Secret
metadata:
  name: nice1-secret
  namespace: d8-system
data:
  client.crt: cXdl # qwe
  client.key: YXNk # asd
`
			f.BindingContexts.Set(f.KubeStateSet(kubeState))
			f.ValuesSetFromYaml("monitoringKubernetesControlPlane.kubeEtcdAdditionalInstances", []byte(additionalKubeEtcd))
			f.ValuesSet("monitoringKubernetesControlPlane.kubeApiserver.accessType", "Extra")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeControllerManager.accessType", "Extra")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.accessType", "Extra")

			f.RunHook()
		})

		It("Hook must not fail, KubeEtcd instances must be handled properly", func() {
			Expect(f).To(ExecuteSuccessfully())

			mergedKubeEtcd := `
- accessType: ThroughNode
  pod:
    podSelector: {}
  metricsPath: /metrics
  name: main
  throughNode:
    authenticationMethod: HostPathCertificate
    hostPathCertificate: /etc/kubernetes/pki/apiserver-etcd-client.crt
    hostPathCertificateKey: /etc/kubernetes/pki/apiserver-etcd-client.key
    localPort: 2379
    nodeSelector:
      node-role.kubernetes.io/master: ""
    proxyListenPort: 10370
- accessType: Pod
  clientCertificate:
    clientCrt: abc
    clientKey: xyz
  pod:
    certificateSecret: nice0-secret
    podNamespace: kuku
    podSelector:
      qqq: zzz
  metricsPath: /metrics
  name: nice0
- accessType: ThroughNode
  clientCertificate:
    clientCrt: qwe
    clientKey: asd
  metricsPath: /metrics
  name: nice1
  throughNode:
    authenticationMethod: Certificate
    certificateSecret: nice1-secret
    localPort: 4242
    nodeSelector:
      xxx: yyy
    proxyListenPort: 10372
`
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeEtcd").String()).To(MatchYAML(mergedKubeEtcd))

			mergedProxy := `
instances:
  425f55b4:
    components:
    - name: KubeEtcdMain
      values:
        accessType: ThroughNode
        pod:
          podSelector: {}
        metricsPath: /metrics
        name: main
        throughNode:
          authenticationMethod: HostPathCertificate
          hostPathCertificate: /etc/kubernetes/pki/apiserver-etcd-client.crt
          hostPathCertificateKey: /etc/kubernetes/pki/apiserver-etcd-client.key
          localPort: 2379
          nodeSelector:
            node-role.kubernetes.io/master: ""
          proxyListenPort: 10370
    nodeSelector:
      node-role.kubernetes.io/master: ""
  e5a31108:
    components:
    - name: KubeEtcdNice1
      values:
        accessType: ThroughNode
        metricsPath: /metrics
        name: nice1
        throughNode:
          authenticationMethod: Certificate
          certificateSecret: nice1-secret
          localPort: 4242
          nodeSelector:
            xxx: yyy
          proxyListenPort: 10372
    nodeSelector:
      xxx: yyy
`
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.proxy").String()).To(MatchYAML(mergedProxy))
		})
	})

	Context("Single kubeEtcdAdditionalInstances is configured, Cluster has d8-pki secret with etcd client cert", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))

			additionalKubeEtcd := `
- name: nice0
  accessType: Pod
  metricsPath: /metrics
  pod:
    podSelector:
      qqq: zzz
    podNamespace: kuku
    authenticationMethod: D8PKI
  d8PKI:
    clientCrt: abc
    clientKey: xyz
`
			f.ValuesSetFromYaml("monitoringKubernetesControlPlane.kubeEtcdAdditionalInstances", []byte(additionalKubeEtcd))
			f.ValuesSet("monitoringKubernetesControlPlane.kubeApiserver.accessType", "Extra")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeControllerManager.accessType", "Extra")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.accessType", "Extra")

			f.RunHook()
		})

		It("Hook must not fail, KubeEtcd instances must be handled properly", func() {
			Expect(f).To(ExecuteSuccessfully())

			mergedKubeEtcd := `
- accessType: ThroughNode
  metricsPath: /metrics
  name: main
  pod:
    podSelector: {}
  throughNode:
    authenticationMethod: HostPathCertificate
    hostPathCertificate: /etc/kubernetes/pki/apiserver-etcd-client.crt
    hostPathCertificateKey: /etc/kubernetes/pki/apiserver-etcd-client.key
    localPort: 2379
    nodeSelector:
      node-role.kubernetes.io/master: ""
    proxyListenPort: 10370
- accessType: Pod
  clientCertificate:
    clientCrt: abc
    clientKey: xyz
  pod:
    podNamespace: kuku
    podSelector:
      qqq: zzz
    authenticationMethod: D8PKI
  metricsPath: /metrics
  name: nice0
  d8PKI:
    clientCrt: abc
    clientKey: xyz
`
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeEtcd").String()).To(MatchYAML(mergedKubeEtcd))
		})
	})

	Context("kubeEtcdEvents is discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))

			f.ValuesSet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.discovered", true)
			f.ValuesSet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.scheme", "https")
			f.ValuesSet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.localPort", 4002)
			f.ValuesSet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.hostPathCertificate", "/etc/qqq.crt")
			f.ValuesSet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.hostPathCertificateKey", "/etc/qqq.key")

			f.ValuesSet("monitoringKubernetesControlPlane.kubeApiserver.accessType", "Extra")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeControllerManager.accessType", "Extra")
			f.ValuesSet("monitoringKubernetesControlPlane.kubeScheduler.accessType", "Extra")

			f.RunHook()
		})

		It("Hook must not fail, KubeEtcd instances must be handled properly", func() {
			Expect(f).To(ExecuteSuccessfully())

			mergedKubeEtcd := `
- accessType: ThroughNode
  pod:
    podSelector: {}
  metricsPath: /metrics
  name: main
  throughNode:
    authenticationMethod: HostPathCertificate
    hostPathCertificate: /etc/kubernetes/pki/apiserver-etcd-client.crt
    hostPathCertificateKey: /etc/kubernetes/pki/apiserver-etcd-client.key
    localPort: 2379
    nodeSelector:
      node-role.kubernetes.io/master: ""
    proxyListenPort: 10370
- accessType: ThroughNode
  discovered: true
  pod:
    podSelector: {}
  metricsPath: /metrics
  name: events
  throughNode:
    authenticationMethod: HostPathCertificate
    hostPathCertificate: /etc/qqq.crt
    hostPathCertificateKey: /etc/qqq.key
    localPort: 4002
    nodeSelector:
      node-role.kubernetes.io/master: ""
    proxyListenPort: 10371
    scheme: https
`
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.kubeEtcd").String()).To(MatchYAML(mergedKubeEtcd))

			mergedProxy := `
instances:
  425f55b4:
    components:
    - name: KubeEtcdMain
      values:
        accessType: ThroughNode
        pod:
          podSelector: {}
        metricsPath: /metrics
        name: main
        throughNode:
          authenticationMethod: HostPathCertificate
          hostPathCertificate: /etc/kubernetes/pki/apiserver-etcd-client.crt
          hostPathCertificateKey: /etc/kubernetes/pki/apiserver-etcd-client.key
          localPort: 2379
          nodeSelector:
            node-role.kubernetes.io/master: ""
          proxyListenPort: 10370
    - name: KubeEtcdEvents
      values:
        accessType: ThroughNode
        discovered: true
        pod:
          podSelector: {}
        metricsPath: /metrics
        name: events
        throughNode:
          authenticationMethod: HostPathCertificate
          hostPathCertificate: /etc/qqq.crt
          hostPathCertificateKey: /etc/qqq.key
          localPort: 4002
          nodeSelector:
            node-role.kubernetes.io/master: ""
          proxyListenPort: 10371
          scheme: https
    nodeSelector:
      node-role.kubernetes.io/master: ""
`
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.internal.proxy").String()).To(MatchYAML(mergedProxy))
		})
	})

})
