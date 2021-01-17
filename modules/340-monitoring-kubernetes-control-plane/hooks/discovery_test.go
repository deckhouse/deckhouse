package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	etcdByManifest = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: etcd
    tier: control-plane
  name: etcd-sandbox-24-master
  namespace: kube-system
spec:
  containers:
  - command:
    - etcd
    - --key-file=/etc/kubernetes/pki/etcd/server.key
    - --listen-client-urls=https://127.0.0.1:2379,https://10.0.3.240:2379
    - --listen-peer-urls=https://10.0.3.240:2380
    image: k8s.gcr.io/etcd:3.3.10
`

	etcdByManager = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: etcd-manager-main
  name: etcd-manager-main-ip-10-1-2-3.eu-central-1.compute.internal
  namespace: kube-system
spec:
  containers:
  - command:
    - /bin/sh
    - -c
    - exec /etcd-manager
      --backup-store=s3://aaa/bbb
      --client-urls=https://__name__:4001 --cluster-name=etcd --containerized=true
      > /tmp/pipe 2>&1
    image: kopeio/etcd-manager:3.0.20190516
`

	etcdEventsByManager = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: etcd-manager-events
  name: etcd-manager-events-ip-10-1-2-3.eu-central-1.compute.internal
  namespace: kube-system
spec:
  containers:
  - command:
    - /bin/sh
    - -c
    - exec /etcd-manager
      --backup-store=s3://aaa/bbb
      --wow --client-urls=https://__name__:4002 --cluster-name=etcd --containerized=true
      > /tmp/pipe 2>&1
    image: kopeio/etcd-manager:3.42
`

	kubeApiserverByComponent = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kube-apiserver
    tier: control-plane
  name: kube-apiserver-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    command:
    - kube-apiserver
    - --etcd-certfile=/etc/qqq.crt
    - --etcd-keyfile=/etc/qqq.key
    - --secure-port=42
    args:
    - qqq
`

	kubeApiserverByK8SApp = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: kube-apiserver
  name: kube-apiserver-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    command:
    - kube-apiserver
    args:
    - --service-cluster-ip-range=192.168.30.0/24
    - --etcd-certfile=/etc/zzz.crt
    - --etcd-keyfile=/etc/zzz.key
    - --secure-port=42
`

	kubeSchedulerByComponentMinimal = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kube-scheduler
    tier: control-plane
  name: kube-scheduler-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-scheduler
    command:
    - kube-scheduler
    - --bind-address=127.0.0.1
    - --kubeconfig=/etc/kubernetes/scheduler.conf
    - --leader-elect=true
    args:
    - qqq
`

	kubeSchedulerByK8SAppMaximal = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: kube-scheduler
  name: kube-scheduler-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-scheduler
    command:
    - kube-scheduler
    - --bind-address=1.2.3.4
    - --secure-port=4242
    - --kubeconfig=/etc/kubernetes/scheduler.conf
    - --authentication-kubeconfig=/etc/kubernetes/admin.conf
    - --authorization-kubeconfig=/etc/kubernetes/admin.conf
    - --leader-elect=true
    args:
    - qqq
`

	kubeControllerManagerByComponentMinimal = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kube-scheduler
    tier: control-plane
  name: kube-controller-manager-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    command:
    - kube-controller-manager
    - --bind-address=127.0.0.1
    args:
    - qqq
`

	kubeControllerManagerByK8SAppMaximal = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: kube-controller-manager
  name: kube-controller-manager-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    command:
    - kube-controller-manager
    - --bind-address=1.2.3.4
    - --secure-port=2424
    args:
    - qqq
`
	d8PKISecret = `
---
apiVersion: v1
data:
  etcd-ca.crt: YWJj # abc
  etcd-ca.key: eHl6 # xyz
kind: Secret
metadata:
  name: d8-pki
  namespace: kube-system
`
)

const initValues = `
{
  "monitoringKubernetesControlPlane": {
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
    }
  }
}`

var _ = Describe("Modules :: monitoring-kubernetes-control-plane :: hooks :: discovery ::", func() {
	f := HookExecutionConfigInit(initValues, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.Session.Err).Should(gbytes.Say(`WARNING: Can't find etcd pod to discover scheme and port.`))
			Expect(f.Session.Err).Should(gbytes.Say(`WARNING: Can't find kube-apiserver pod to discover metrics port, selector and etcd client cert.`))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.pod.scheme").Exists()).To(BeFalse())
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.pod.localPort").Exists()).To(BeFalse())
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.scheme").Exists()).To(BeFalse())
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.localPort").Exists()).To(BeFalse())

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.port").Exists()).To(BeFalse())
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.throughNode.localPort").Exists()).To(BeFalse())
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.podNamespace").Exists()).To(BeFalse())
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.podSelector").Exists()).To(BeFalse())
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.hostPathCertificate").Exists()).To(BeFalse())
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.hostPathCertificateKey").Exists()).To(BeFalse())
		})
	})

	Context("apiserver by component labels; single etcd by manifest; minimal kube-scheduler; minimal kube-controller-manager", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeApiserverByComponent + etcdByManifest + kubeSchedulerByComponentMinimal + kubeControllerManagerByComponentMinimal))
			f.RunHook()
		})

		It("everything must be parsed", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.pod.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.pod.localPort").String()).To(Equal("2379"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.localPort").String()).To(Equal("2379"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.discovered").Bool()).To(BeFalse())

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.port").String()).To(Equal("42"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.throughNode.localPort").String()).To(Equal("42"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.podNamespace").String()).To(Equal("kube-system"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.podSelector").String()).To(MatchJSON(`{"component": "kube-apiserver","tier": "control-plane"}`))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.hostPathCertificate").String()).To(Equal("/etc/qqq.crt"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.hostPathCertificateKey").String()).To(Equal("/etc/qqq.key"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.hostPathCertificate").String()).To(Equal("/etc/qqq.crt"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.hostPathCertificateKey").String()).To(Equal("/etc/qqq.key"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeScheduler").String()).To(MatchJSON(`{"pod":{},"throughNode":{}}`))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeControllerManager").String()).To(MatchJSON(`{"pod":{},"throughNode":{}}`))
		})
	})

	Context("apiserver by k8s-app labels; single etcd by manager; maximal kube-scheduler; maximal kube-controller-manager", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeApiserverByK8SApp + etcdByManager + kubeSchedulerByK8SAppMaximal + kubeControllerManagerByK8SAppMaximal))
			f.RunHook()
		})

		It("etcd must be parsed, apiserver must be parsed", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.pod.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.pod.localPort").String()).To(Equal("4001"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.localPort").String()).To(Equal("4001"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.discovered").Bool()).To(BeFalse())

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.port").String()).To(Equal("42"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.throughNode.localPort").String()).To(Equal("42"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.podNamespace").String()).To(Equal("kube-system"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.podSelector").String()).To(MatchJSON(`{"k8s-app": "kube-apiserver"}`))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.hostPathCertificate").String()).To(Equal("/etc/zzz.crt"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.hostPathCertificateKey").String()).To(Equal("/etc/zzz.key"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.hostPathCertificate").String()).To(Equal("/etc/zzz.crt"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.hostPathCertificateKey").String()).To(Equal("/etc/zzz.key"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeScheduler.accessType").String()).To(Equal("Pod"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeScheduler.pod.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeScheduler.pod.port").String()).To(Equal("4242"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeScheduler.pod.podSelector").String()).To(MatchJSON(`{"k8s-app": "kube-scheduler"}`))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeScheduler.pod.podNamespace").String()).To(Equal("kube-system"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeScheduler.pod.authenticationMethod").String()).To(Equal("PrometheusCertificate"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeControllerManager.accessType").String()).To(Equal("Pod"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeControllerManager.pod.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeControllerManager.pod.port").String()).To(Equal("2424"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeControllerManager.pod.podSelector").String()).To(MatchJSON(`{"k8s-app": "kube-controller-manager"}`))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeControllerManager.pod.podNamespace").String()).To(Equal("kube-system"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeControllerManager.pod.authenticationMethod").String()).To(Equal("PrometheusCertificate"))
		})
	})

	Context("apiserver by k8s-app labels; etcd by manager; etcd-events by manager; d8-pki secret is in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeApiserverByK8SApp + etcdByManager + etcdEventsByManager + d8PKISecret))
			f.RunHook()
		})

		It("etcd must be parsed, apiserver must be parsed", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.pod.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.pod.localPort").String()).To(Equal("4001"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.localPort").String()).To(Equal("4001"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.discovered").Bool()).To(BeTrue())
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.pod.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.pod.localPort").String()).To(Equal("4002"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.localPort").String()).To(Equal("4002"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.port").String()).To(Equal("42"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.throughNode.localPort").String()).To(Equal("42"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.podNamespace").String()).To(Equal("kube-system"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeApiserver.pod.podSelector").String()).To(MatchJSON(`{"k8s-app": "kube-apiserver"}`))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.hostPathCertificate").String()).To(Equal("/etc/zzz.crt"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.throughNode.hostPathCertificateKey").String()).To(Equal("/etc/zzz.key"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.hostPathCertificate").String()).To(Equal("/etc/zzz.crt"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.throughNode.hostPathCertificateKey").String()).To(Equal("/etc/zzz.key"))

			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.accessType").String()).To(Equal("Pod"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.pod.authenticationMethod").String()).To(Equal("D8PKI"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcd.d8PKI").String()).To(MatchJSON(`{"clientCrt":"abc","clientKey":"xyz"}`))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.accessType").String()).To(Equal("Pod"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.pod.authenticationMethod").String()).To(Equal("D8PKI"))
			Expect(f.ValuesGet("monitoringKubernetesControlPlane.discovery.kubeEtcdEvents.d8PKI").String()).To(MatchJSON(`{"clientCrt":"abc","clientKey":"xyz"}`))
		})
	})

})
