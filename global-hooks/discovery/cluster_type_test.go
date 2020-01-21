/*

User-stories:
1. There are controller-manager pod in cluster. It has --cloud-provider=XXX arg. It has different labels in different types of clusters. Hook must parse this arg and store to `global.discovery.clusterType`
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/cluster_type ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		state_ByComponent_Command_External = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kube-controller-manager
    tier: control-plane
  name: kube-controller-manager-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    command:
    - kube-controller-manager
    - --client-ca-file=/etc/kubernetes/pki/ca.crt
    - --cloud-provider=external
    - --use-service-account-credentials=true
    args:
    - qqq
    - www
`

		state_ByK8SApp_Command_AWS = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: kube-controller-manager
  name: kube-controller-manager-ip-10-241-62-185.eu-central-1.compute.internal
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    command:
    - /bin/sh
    - -c
    - mkfifo /tmp/pipe; (tee -a /var/log/kube-controller-manager.log < /tmp/pipe &
      ) ; exec /usr/local/bin/kube-controller-manager --allocate-node-cidrs=true --attach-detach-reconcile-sync-period=1m0s
      --cloud-config=/etc/kubernetes/cloud.config --cloud-provider=aws --cluster-cidr=100.96.0.0/11
      --cluster-name=k-dev.k8s --cluster-signing-cert-file=/srv/kubernetes/ca.crt
      --cluster-signing-key-file=/srv/kubernetes/ca.key --configure-cloud-routes=true
      --kubeconfig=/var/lib/kube-controller-manager/kubeconfig --leader-elect=true
      --root-ca-file=/srv/kubernetes/ca.crt --service-account-private-key-file=/srv/kubernetes/server.key
      --use-service-account-credentials=true --v=2 > /tmp/pipe 2>&1
`

		state_ByComponent_Args_GCE = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kube-controller-manager
    tier: control-plane
  name: kube-controller-manager-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    command:
    - kube-controller-manager
    - --client-ca-file=/etc/kubernetes/pki/ca.crt
    - --use-service-account-credentials=true
    args:
    - qqq
    - --cloud-provider=gce
    - www
`

		state_ByK8SApp_Args_ACS = `
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
    args:
    - qqq
    - "--aaa --bbb --cloud-provider=azure"
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("controller-manager has label 'component:', arg in command[], --cloud-provider=external", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state_ByComponent_Command_External))
			f.RunHook()
		})

		It("filterResult must be 'external'; `global.discovery.clusterType` must be 'Manual'", func() {
			Expect(f).To(ExecuteSuccessfully())

			// binding: ControllerManagerByComponent
			Expect(f.BindingContexts.Get("0.objects.0.filterResult").String()).To(Equal("external"))
			// binding: ControllerManagerByK8SApp
			Expect(f.BindingContexts.Get("1.objects.0.filterResult").Value()).To(BeNil())

			Expect(f.ValuesGet("global.discovery.clusterType").String()).To(Equal("Manual"))
		})
	})

	Context("controller-manager has label 'k8s-app:', arg in command[], --cloud-provider=aws", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state_ByK8SApp_Command_AWS))
			f.RunHook()
		})

		It("filterResult must be 'aws'; `global.discovery.clusterType` must be 'AWS'", func() {
			Expect(f).To(ExecuteSuccessfully())

			// binding: ControllerManagerByComponent
			Expect(f.BindingContexts.Get("0.objects.0.filterResult").Value()).To(BeNil())
			// binding: ControllerManagerByK8SApp
			Expect(f.BindingContexts.Get("1.objects.0.filterResult").String()).To(Equal("aws"))

			Expect(f.ValuesGet("global.discovery.clusterType").String()).To(Equal("AWS"))
		})
	})

	Context("controller-manager has label 'component:', arg in args[], --cloud-provider=gce", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state_ByComponent_Args_GCE))
			f.RunHook()
		})

		It("filterResult must be 'gce'; `global.discovery.clusterType` must be 'GCE'", func() {
			Expect(f).To(ExecuteSuccessfully())

			// binding: ControllerManagerByComponent
			Expect(f.BindingContexts.Get("0.objects.0.filterResult").String()).To(Equal("gce"))
			// binding: ControllerManagerByK8SApp
			Expect(f.BindingContexts.Get("1.objects.0.filterResult").Value()).To(BeNil())

			Expect(f.ValuesGet("global.discovery.clusterType").String()).To(Equal("GCE"))
		})
	})

	Context("controller-manager has label 'k8s-app:', arg in args[], --cloud-provider=azure", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state_ByK8SApp_Args_ACS))
			f.RunHook()
		})

		It("filterResult must be 'azure'; `global.discovery.clusterType` must be 'ACS'", func() {
			Expect(f).To(ExecuteSuccessfully())

			// binding: ControllerManagerByComponent
			Expect(f.BindingContexts.Get("0.objects.0.filterResult").Value()).To(BeNil())
			// binding: ControllerManagerByK8SApp
			Expect(f.BindingContexts.Get("1.objects.0.filterResult").String()).To(Equal("azure"))

			Expect(f.ValuesGet("global.discovery.clusterType").String()).To(Equal("ACS"))
		})
	})
})
