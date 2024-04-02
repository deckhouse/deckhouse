/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node local dns :: hooks :: cleanup daemonset hostports ::", func() {
	const (
		initValuesString       = `{"nodeLocalDns": {"internal": {}}}`
		initConfigValuesString = `{}`
		dsToBePatched          = `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: node-local-dns
    heritage: deckhouse
    module: node-local-dns
  name: node-local-dns
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: node-local-dns
  template:
    metadata:
      labels:
        app: node-local-dns
        k8s-app: node-local-dns
    spec:
      containers:
      - command:
        - /bin/bash
        - -l
        - -c
        - /start.sh
        image: coredns
        name: coredns
        ports:
        - containerPort: 53
          hostPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          hostPort: 53
          name: dns-tcp
          protocol: TCP
      - args:
        - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9254
        - --v=2
        - --logtostderr=true
        - --stale-cache-interval=1h30m
        env:
        - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        image: kube-rbac-proxy
        name: kube-rbac-proxy
        ports:
        - containerPort: 9254
          hostPort: 9254
          name: https-metrics
          protocol: TCP
      dnsPolicy: Default
`
		dsToBeSkipped = `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: node-local-dns
    heritage: deckhouse
    module: node-local-dns
  name: node-local-dns
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: node-local-dns
  template:
    metadata:
      labels:
        app: node-local-dns
        k8s-app: node-local-dns
    spec:
      containers:
      - command:
        - /bin/bash
        - -l
        - -c
        - /start.sh
        env:
        - name: SHOULD_SETUP_IPTABLES
          value: "yes"
        image: coredns
        name: coredns
        ports:
        - containerPort: 53
          hostPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          hostPort: 53
          name: dns-tcp
          protocol: TCP
      - args:
        - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9254
        - --v=2
        - --logtostderr=true
        - --stale-cache-interval=1h30m
        env:
        - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        image: kube-rbac-proxy
        name: kube-rbac-proxy
        ports:
        - containerPort: 9254
          hostPort: 9254
          name: https-metrics
          protocol: TCP
      dnsPolicy: ClusterFirstWithHostNet
      hostNetwork: true
`
		dsWithoutHostPorts = `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: node-local-dns
    heritage: deckhouse
    module: node-local-dns
  name: node-local-dns
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: node-local-dns
  template:
    metadata:
      labels:
        app: node-local-dns
        k8s-app: node-local-dns
    spec:
      containers:
      - command:
        - /bin/bash
        - -l
        - -c
        - /start.sh
        image: coredns
        name: coredns
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
      - args:
        - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9254
        - --v=2
        - --logtostderr=true
        - --stale-cache-interval=1h30m
        env:
        - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        image: kube-rbac-proxy
        name: kube-rbac-proxy
        ports:
        - containerPort: 9254
          name: https-metrics
          protocol: TCP
      dnsPolicy: Default
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Without DaemonSet", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())

			ds, _ := dependency.TestDC.MustGetK8sClient().AppsV1().DaemonSets("kube-system").Get(context.TODO(), "node-local-dns", metav1.GetOptions{})
			Expect(ds).To(BeNil())
		})
	})

	Context("With hostNetwork=false (null) and hostPorts", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsToBePatched))
			var ds appsv1.DaemonSet
			_ = yaml.Unmarshal([]byte(dsToBePatched), &ds)
			_, err := dependency.TestDC.MustGetK8sClient().
				AppsV1().
				DaemonSets("kube-system").
				Create(context.TODO(), &ds, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.RunHook()
		})

		It("Should remove hostPorts from manifest", func() {
			Expect(f).To(ExecuteSuccessfully())

			ds, _ := dependency.TestDC.MustGetK8sClient().AppsV1().DaemonSets("kube-system").Get(context.TODO(), "node-local-dns", metav1.GetOptions{})
			Expect(ds.Spec.Template.Spec.HostNetwork).To(BeFalse())
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].HostPort).To(BeZero())
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[1].HostPort).To(BeZero())
			Expect(ds.Spec.Template.Spec.Containers[1].Ports[0].HostPort).To(BeZero())
		})
	})

	Context("With hostNetwork=false (null) and without hostPorts", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsWithoutHostPorts))
			var ds appsv1.DaemonSet
			_ = yaml.Unmarshal([]byte(dsWithoutHostPorts), &ds)
			_, err := dependency.TestDC.MustGetK8sClient().
				AppsV1().
				DaemonSets("kube-system").
				Create(context.TODO(), &ds, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.RunHook()
		})

		It("Shouldn't change anything", func() {
			Expect(f).To(ExecuteSuccessfully())

			ds, _ := dependency.TestDC.MustGetK8sClient().AppsV1().DaemonSets("kube-system").Get(context.TODO(), "node-local-dns", metav1.GetOptions{})
			Expect(ds.Spec.Template.Spec.HostNetwork).To(BeFalse())
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].HostPort).To(BeZero())
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[1].HostPort).To(BeZero())
			Expect(ds.Spec.Template.Spec.Containers[1].Ports[0].HostPort).To(BeZero())
		})
	})

	Context("With hostNetwork=true", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsToBeSkipped))
			var ds appsv1.DaemonSet
			_ = yaml.Unmarshal([]byte(dsToBeSkipped), &ds)
			_, err := dependency.TestDC.MustGetK8sClient().
				AppsV1().
				DaemonSets("kube-system").
				Create(context.TODO(), &ds, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.RunHook()
		})

		It("Should do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			ds, _ := dependency.TestDC.MustGetK8sClient().AppsV1().DaemonSets("kube-system").Get(context.TODO(), "node-local-dns", metav1.GetOptions{})
			Expect(ds.Spec.Template.Spec.HostNetwork).To(BeTrue())
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].HostPort).NotTo(BeZero())
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[1].HostPort).NotTo(BeZero())
			Expect(ds.Spec.Template.Spec.Containers[1].Ports[0].HostPort).NotTo(BeZero())
		})
	})
})
