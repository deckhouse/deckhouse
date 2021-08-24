/*
Copyright 2021 Flant CJSC

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

package checker

import (
	"fmt"
	"testing"
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/kube/fake"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"d8.io/upmeter/pkg/check"
	k8saccess "d8.io/upmeter/pkg/kubernetes"
)

func NewFake(client kube.KubernetesClient) *FakeAccess {
	return &FakeAccess{client: client}
}

type FakeAccess struct {
	client kube.KubernetesClient
}

func (a *FakeAccess) Kubernetes() kubernetes.Interface {
	return a.client
}

func (a *FakeAccess) ServiceAccountToken() string {
	return "pewpew"
}

func (a *FakeAccess) SchedulerProbeImage() *k8saccess.ProbeImage {
	return createTestProbeImage("test-image:latest", nil)
}

func (a *FakeAccess) SchedulerProbeNode() string {
	return ""
}

func (a *FakeAccess) CloudControllerManagerNamespace() string {
	return ""
}

func getTestDsPodsReadinessChecker() (*fake.FakeCluster, *dsPodsReadinessChecker) {
	cluster := fake.NewFakeCluster()
	access := NewFake(cluster.KubeClient)

	dsChecker := &dsPodsReadinessChecker{
		access:        access,
		namespace:     "d8-monitoring",
		daemonSetName: "node-exporter",
		timeout:       5 * time.Minute,
	}

	return cluster, dsChecker
}

func runDaemonsetCheckerWithCluster(setupCluster func(*fake.FakeCluster, *dsPodsReadinessChecker)) check.Error {
	cluster, checker := getTestDsPodsReadinessChecker()
	setupCluster(cluster, checker)
	return checker.Check()
}

func Test_checker_DsPodsReadiness(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fake.FakeCluster, *dsPodsReadinessChecker)
		err   check.Error
	}{
		{
			name:  "empty cluster, something is wrong",
			err:   check.ErrUnknown(""),
			setup: func(*fake.FakeCluster, *dsPodsReadinessChecker) {},
		},
		{
			name: "daemonset exists, but no nodes and no pods, nothing to complain about",
			err:  nil,
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				_, ds, _ := getTestingClusterSetupForDaemonsSetChecker()
				_, err := cluster.KubeClient.AppsV1().DaemonSets(checker.namespace).Create(&ds)
				if err != nil {
					panic("atata")
				}
			},
		},
		{
			name: "everything in place, nothing to complain about",
			err:  nil,
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()
				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "node without a daemonset pod is a problem",
			err:  check.ErrFail(""),
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				additionalNode := nodeList.Items[0]
				additionalNode.ObjectMeta.SetName("abracadabra")
				nodeList.Items = append(nodeList.Items, additionalNode)

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "missing node is fine",
			err:  nil,
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				notAllNodes := make([]v1.Node, 0)
				notAllNodes = append(notAllNodes, nodeList.Items[:1]...)
				notAllNodes = append(notAllNodes, nodeList.Items[2:]...)
				nodeList.Items = notAllNodes

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "not-ready node is fine",
			err:  nil,
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				badNode := nodeList.Items[2]
				for _, cond := range badNode.Status.Conditions {
					if cond.Type != v1.NodeReady {
						continue
					}
					cond.Status = v1.ConditionFalse
				}

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "additional node with an additional taint is fine",
			err:  nil,
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				tainted := nodeList.Items[2]
				tainted.SetName("tainted")
				tainted.Spec.Taints = append(tainted.Spec.Taints, v1.Taint{
					Key:   "some",
					Value: "staff",
				})
				nodeList.Items = append(nodeList.Items, tainted)

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "not-ready pod is not fine",
			err:  check.ErrFail(""),
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				unhealthy := &podList.Items[2]
				for i, cond := range unhealthy.Status.Conditions {
					if cond.Type != v1.PodReady {
						continue
					}
					unhealthy.Status.Conditions[i].Status = v1.ConditionFalse
				}

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "not-running pod is not fine",
			err:  check.ErrFail(""),
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				unhealthy := &podList.Items[2]
				unhealthy.Status.Phase = v1.PodFailed

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "missing pod is not fine",
			err:  check.ErrFail(""),
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				notAllPods := make([]v1.Pod, 0)
				notAllPods = append(notAllPods, podList.Items[:1]...)
				notAllPods = append(notAllPods, podList.Items[2:]...)
				podList.Items = notAllPods

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},

		{
			name: "unhealthy pod on very fresh node is fine",
			err:  nil,
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				// Make pod unhealthy
				unhealthy := &podList.Items[2]
				unhealthy.Status.Phase = v1.PodFailed

				// Make its node very fresh
				for ni, node := range nodeList.Items {
					if node.GetName() != unhealthy.Spec.NodeName {
						continue
					}
					for ci, cond := range node.Status.Conditions {
						if cond.Type != v1.NodeReady {
							continue
						}
						nodeList.Items[ni].Status.Conditions[ci].LastTransitionTime = metav1.Now()
					}
				}

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runDaemonsetCheckerWithCluster(tt.setup)

			if tt.err == nil {
				if err == nil {
					return
				}
				t.Fatalf("expected success, got error status=%q, err=%q", err.Status(), err.Error())
			}

			if err == nil {
				t.Fatalf("got success, expected error with status %q", tt.err.Status())
			}

			if err.Status() != tt.err.Status() {
				t.Errorf("expected status %q, got status=%q  err=%v", tt.err.Status(), err.Status(), err.Error())
			}
		})
	}
}

func setupTestingClusterForDaemonsSetChecker(cluster *fake.FakeCluster, namespace string, nodeList v1.NodeList, ds appsv1.DaemonSet, podList v1.PodList) error {
	if _, err := cluster.KubeClient.AppsV1().DaemonSets(namespace).Create(&ds); err != nil {
		return fmt.Errorf("ds did not create: %v", err)
	}

	for _, node := range nodeList.Items {
		if _, err := cluster.KubeClient.CoreV1().Nodes().Create(&node); err != nil {
			return fmt.Errorf("node did not create: %v", err)
		}
	}

	for _, pod := range podList.Items {
		if _, err := cluster.KubeClient.CoreV1().Pods(namespace).Create(&pod); err != nil {
			return fmt.Errorf("pod did not create: %v", err)
		}
	}

	return nil
}

// This is a consistent state of four working nodes (3×M+W), node-exporter daemonset, and four working pods
func getTestingClusterSetupForDaemonsSetChecker() (v1.NodeList, appsv1.DaemonSet, v1.PodList) {
	daemonSetYAML := `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  annotations:
    deprecated.daemonset.template.generation: "6"
    meta.helm.sh/release-name: monitoring-kubernetes
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: node-exporter
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: monitoring-kubernetes
  name: node-exporter
  namespace: d8-monitoring
spec:
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: node-exporter
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: node-exporter
      name: node-exporter
    spec:
      containers:
      - args:
        - --web.listen-address=127.0.0.1:9101
        - --path.rootfs=/host/root
        - --collector.ntp
        - --collector.ntp.server-is-local
        - --collector.filesystem.ignored-mount-points
        - (^/(dev|proc|sys|run|var/lib/kubelet)($|/))|(^/var/lib/docker/)
        - --collector.filesystem.ignored-fs-types
        - ^(autofs|binfmt_misc|cgroup|configfs|debugfs|devpts|devtmpfs|fusectl|fuse\.lxcfs|hugetlbfs|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|sysfs|tracefs|squashfs)$
        - --collector.textfile.directory
        - /host/textfile
        image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter:05e2b2361ad1d69b3ae23b369308e759540b3413834f0011e318768b
        imagePullPolicy: IfNotPresent
        name: node-exporter
        resources:
          requests:
            ephemeral-storage: 60Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /host/root
          name: root
          readOnly: true
        - mountPath: /host/textfile
          name: textfile
          readOnly: true
      - env:
        - name: MY_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter:455222c1194ecff9aa77c8ae0daa936260e43d420ae6e512a1026dae
        imagePullPolicy: IfNotPresent
        name: kubelet-eviction-thresholds-exporter
        resources:
          requests:
            ephemeral-storage: 50Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /host/
          name: root
          readOnly: true
        - mountPath: /var/run/node-exporter-textfile
          name: textfile
        - mountPath: /var/run/docker.sock
          name: dockersock
        - mountPath: /var/run/containerd/containerd.sock
          name: containerdsock
        - mountPath: /usr/local/bin/crictl
          name: crictl
      - args:
        - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9101
        - --client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        - --v=2
        - --logtostderr=true
        - --stale-cache-interval=1h30m
        env:
        - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        - name: KUBE_RBAC_PROXY_CONFIG
          value: |
            upstreams:
            - upstream: http://127.0.0.1:9101/metrics
              path: /metrics
              authorization:
                resourceAttributes:
                  namespace: d8-monitoring
                  apiGroup: apps
                  apiVersion: v1
                  resource: daemonsets
                  subresource: prometheus-metrics
                  name: node-exporter
        image: registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy:526c5255969dcd342888947e2d3bab781eed225bb969ca0dd3bdbd30
        imagePullPolicy: IfNotPresent
        name: kube-rbac-proxy
        ports:
        - containerPort: 9101
          hostPort: 9101
          name: https-metrics
          protocol: TCP
        resources:
          requests:
            ephemeral-storage: 50Mi
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      hostNetwork: true
      hostPID: true
      imagePullSecrets:
      - name: deckhouse-registry
      priorityClassName: system-node-critical
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext:
        runAsGroup: 0
        runAsNonRoot: false
        runAsUser: 0
      serviceAccount: node-exporter
      serviceAccountName: node-exporter
      terminationGracePeriodSeconds: 30
      tolerations:
      - key: node-role.kubernetes.io/master
      - key: dedicated.deckhouse.io
        operator: Exists
      - key: dedicated
        operator: Exists
      - key: node.kubernetes.io/not-ready
      - key: node.kubernetes.io/out-of-disk
      - key: node.kubernetes.io/memory-pressure
      - key: node.kubernetes.io/disk-pressure
      volumes:
      - hostPath:
          path: /var/run/node-exporter-textfile
          type: DirectoryOrCreate
        name: textfile
      - hostPath:
          path: /
          type: ""
        name: root
      - hostPath:
          path: /var/run/docker.sock
          type: ""
        name: dockersock
      - hostPath:
          path: /var/run/containerd/containerd.sock
          type: ""
        name: containerdsock
      - hostPath:
          path: /usr/local/bin/crictl
          type: ""
        name: crictl
  updateStrategy:
    rollingUpdate:
      maxUnavailable: 1
    type: RollingUpdate
status:
  currentNumberScheduled: 4
  desiredNumberScheduled: 4
  numberAvailable: 4
  numberMisscheduled: 0
  numberReady: 4
  observedGeneration: 6
  updatedNumberScheduled: 4
`

	nodeListYAML := `
apiVersion: v1
items:
  - apiVersion: v1
    kind: Node
    metadata:
      annotations:
        csi.volume.kubernetes.io/nodeid: '{"cinder.csi.openstack.org":"54c0f31e-1f63-4913-a52d-f1b56fcd5371"}'
        flannel.alpha.coreos.com/backend-data: "null"
        flannel.alpha.coreos.com/backend-type: host-gw
        flannel.alpha.coreos.com/kube-subnet-manager: "true"
        flannel.alpha.coreos.com/public-ip: 192.168.199.224
        node-manager.deckhouse.io/last-applied-node-template: '{"annotations":{},"labels":{"node-role.kubernetes.io/master":""},"taints":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master"}]}'
        node.alpha.kubernetes.io/ttl: "0"
        node.deckhouse.io/configuration-checksum: f34ab6552971b3933ad2a13546e11cd81603850553ad28aaa4089ac2c6750ad0
        node.deckhouse.io/virtualization: kvm
        volumes.kubernetes.io/controller-managed-attach-detach: "true"
      labels:
        beta.kubernetes.io/arch: amd64
        beta.kubernetes.io/instance-type: 69a1095b-ceec-404f-8545-800391cdbce1
        beta.kubernetes.io/os: linux
        failure-domain.beta.kubernetes.io/region: HetznerFinland
        failure-domain.beta.kubernetes.io/zone: nova
        kubernetes.io/arch: amd64
        kubernetes.io/hostname: test-master-0
        kubernetes.io/os: linux
        node-role.kubernetes.io/master: ""
        node.deckhouse.io/group: master
        node.deckhouse.io/type: CloudPermanent
        node.kubernetes.io/instance-type: 69a1095b-ceec-404f-8545-800391cdbce1
        topology.cinder.csi.openstack.org/zone: nova
        topology.kubernetes.io/region: HetznerFinland
        topology.kubernetes.io/zone: nova
      name: test-master-0
    spec:
      podCIDR: 10.111.0.0/24
      podCIDRs:
        - 10.111.0.0/24
      providerID: openstack:///54c0f31e-1f63-4913-a52d-f1b56fcd5371
      taints:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
    status:
      addresses:
        - address: 192.168.199.224
          type: InternalIP
        - address: 95.217.82.161
          type: ExternalIP
        - address: test-master-0
          type: Hostname
      allocatable:
        cpu: "4"
        ephemeral-storage: "19597760292"
        hugepages-1Gi: "0"
        hugepages-2Mi: "0"
        memory: "8264994695"
        pods: "110"
      capacity:
        cpu: "4"
        ephemeral-storage: 20145724Ki
        hugepages-1Gi: "0"
        hugepages-2Mi: "0"
        memory: 8152812Ki
        pods: "110"
      conditions:
        - lastHeartbeatTime: "2021-03-31T12:12:54Z"
          lastTransitionTime: "2021-03-31T12:12:54Z"
          message: Flannel is running on this node
          reason: FlannelIsUp
          status: "False"
          type: NetworkUnavailable
        - lastHeartbeatTime: "2021-05-13T03:54:27Z"
          lastTransitionTime: "2020-12-18T08:26:20Z"
          message: kubelet has sufficient memory available
          reason: KubeletHasSufficientMemory
          status: "False"
          type: MemoryPressure
        - lastHeartbeatTime: "2021-05-13T03:54:27Z"
          lastTransitionTime: "2020-12-18T08:26:20Z"
          message: kubelet has no disk pressure
          reason: KubeletHasNoDiskPressure
          status: "False"
          type: DiskPressure
        - lastHeartbeatTime: "2021-05-13T03:54:27Z"
          lastTransitionTime: "2020-12-18T08:26:20Z"
          message: kubelet has sufficient PID available
          reason: KubeletHasSufficientPID
          status: "False"
          type: PIDPressure
        - lastHeartbeatTime: "2021-05-13T03:54:27Z"
          lastTransitionTime: "2021-05-12T11:10:01Z"
          message: kubelet is posting ready status. AppArmor enabled
          reason: KubeletReady
          status: "True"
          type: Ready
      daemonEndpoints:
        kubeletEndpoint:
          Port: 10250
      nodeInfo:
        architecture: amd64
        bootID: 61075cae-ae02-46b8-a493-9d36ee97bc2d
        containerRuntimeVersion: docker://18.9.7
        kernelVersion: 5.3.0-51-generic
        kubeProxyVersion: v1.19.10
        kubeletVersion: v1.19.10
        machineID: 54c0f31e1f634913a52df1b56fcd5371
        operatingSystem: linux
        osImage: Ubuntu 18.04.3 LTS
        systemUUID: 54c0f31e-1f63-4913-a52d-f1b56fcd5371
  - apiVersion: v1
    kind: Node
    metadata:
      annotations:
        csi.volume.kubernetes.io/nodeid: '{"cinder.csi.openstack.org":"1dd50ddb-ece7-4385-97a0-05f1196c887c"}'
        flannel.alpha.coreos.com/backend-data: "null"
        flannel.alpha.coreos.com/backend-type: host-gw
        flannel.alpha.coreos.com/kube-subnet-manager: "true"
        flannel.alpha.coreos.com/public-ip: 192.168.199.94
        node-manager.deckhouse.io/last-applied-node-template: '{"annotations":{},"labels":{"node-role.kubernetes.io/master":""},"taints":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master"}]}'
        node.alpha.kubernetes.io/ttl: "0"
        node.deckhouse.io/configuration-checksum: f34ab6552971b3933ad2a13546e11cd81603850553ad28aaa4089ac2c6750ad0
        node.deckhouse.io/virtualization: kvm
        volumes.kubernetes.io/controller-managed-attach-detach: "true"
      labels:
        beta.kubernetes.io/arch: amd64
        beta.kubernetes.io/instance-type: 69a1095b-ceec-404f-8545-800391cdbce1
        beta.kubernetes.io/os: linux
        failure-domain.beta.kubernetes.io/region: HetznerFinland
        failure-domain.beta.kubernetes.io/zone: nova
        kubernetes.io/arch: amd64
        kubernetes.io/hostname: test-master-1
        kubernetes.io/os: linux
        node-role.kubernetes.io/master: ""
        node.deckhouse.io/group: master
        node.deckhouse.io/type: CloudPermanent
        node.kubernetes.io/instance-type: 69a1095b-ceec-404f-8545-800391cdbce1
        topology.cinder.csi.openstack.org/zone: nova
        topology.kubernetes.io/region: HetznerFinland
        topology.kubernetes.io/zone: nova
      name: test-master-1
    spec:
      podCIDR: 10.111.2.0/24
      podCIDRs:
        - 10.111.2.0/24
      providerID: openstack:///1dd50ddb-ece7-4385-97a0-05f1196c887c
      taints:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
    status:
      addresses:
        - address: 192.168.199.94
          type: InternalIP
        - address: 95.217.68.221
          type: ExternalIP
        - address: test-master-1
          type: Hostname
      allocatable:
        cpu: "4"
        ephemeral-storage: "19597760292"
        hugepages-1Gi: "0"
        hugepages-2Mi: "0"
        memory: "8264994695"
        pods: "110"
      capacity:
        cpu: "4"
        ephemeral-storage: 20145724Ki
        hugepages-1Gi: "0"
        hugepages-2Mi: "0"
        memory: 8152812Ki
        pods: "110"
      conditions:
        - lastHeartbeatTime: "2021-03-24T14:13:57Z"
          lastTransitionTime: "2021-03-24T14:13:57Z"
          message: Flannel is running on this node
          reason: FlannelIsUp
          status: "False"
          type: NetworkUnavailable
        - lastHeartbeatTime: "2021-05-13T03:54:33Z"
          lastTransitionTime: "2021-03-24T14:13:22Z"
          message: kubelet has sufficient memory available
          reason: KubeletHasSufficientMemory
          status: "False"
          type: MemoryPressure
        - lastHeartbeatTime: "2021-05-13T03:54:33Z"
          lastTransitionTime: "2021-03-24T14:13:22Z"
          message: kubelet has no disk pressure
          reason: KubeletHasNoDiskPressure
          status: "False"
          type: DiskPressure
        - lastHeartbeatTime: "2021-05-13T03:54:33Z"
          lastTransitionTime: "2021-03-24T14:13:22Z"
          message: kubelet has sufficient PID available
          reason: KubeletHasSufficientPID
          status: "False"
          type: PIDPressure
        - lastHeartbeatTime: "2021-05-13T03:54:33Z"
          lastTransitionTime: "2021-05-12T11:09:14Z"
          message: kubelet is posting ready status. AppArmor enabled
          reason: KubeletReady
          status: "True"
          type: Ready
      daemonEndpoints:
        kubeletEndpoint:
          Port: 10250
      nodeInfo:
        architecture: amd64
        bootID: 054c0b7c-92fc-4d98-8ffb-4ceb9a347be0
        containerRuntimeVersion: docker://19.3.13
        kernelVersion: 5.3.0-51-generic
        kubeProxyVersion: v1.19.10
        kubeletVersion: v1.19.10
        machineID: 1dd50ddbece7438597a005f1196c887c
        operatingSystem: linux
        osImage: Ubuntu 18.04.3 LTS
        systemUUID: 1dd50ddb-ece7-4385-97a0-05f1196c887c
  - apiVersion: v1
    kind: Node
    metadata:
      annotations:
        csi.volume.kubernetes.io/nodeid: '{"cinder.csi.openstack.org":"e5f8357a-23c6-49a4-9dcb-08245526439f"}'
        flannel.alpha.coreos.com/backend-data: "null"
        flannel.alpha.coreos.com/backend-type: host-gw
        flannel.alpha.coreos.com/kube-subnet-manager: "true"
        flannel.alpha.coreos.com/public-ip: 192.168.199.55
        node-manager.deckhouse.io/last-applied-node-template: '{"annotations":{},"labels":{"node-role.kubernetes.io/master":""},"taints":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master"}]}'
        node.alpha.kubernetes.io/ttl: "0"
        node.deckhouse.io/configuration-checksum: f34ab6552971b3933ad2a13546e11cd81603850553ad28aaa4089ac2c6750ad0
        node.deckhouse.io/virtualization: kvm
        volumes.kubernetes.io/controller-managed-attach-detach: "true"
      labels:
        beta.kubernetes.io/arch: amd64
        beta.kubernetes.io/instance-type: 69a1095b-ceec-404f-8545-800391cdbce1
        beta.kubernetes.io/os: linux
        failure-domain.beta.kubernetes.io/region: HetznerFinland
        failure-domain.beta.kubernetes.io/zone: nova
        kubernetes.io/arch: amd64
        kubernetes.io/hostname: test-master-2
        kubernetes.io/os: linux
        node-role.kubernetes.io/master: ""
        node.deckhouse.io/group: master
        node.deckhouse.io/type: CloudPermanent
        node.kubernetes.io/instance-type: 69a1095b-ceec-404f-8545-800391cdbce1
        topology.cinder.csi.openstack.org/zone: nova
        topology.kubernetes.io/region: HetznerFinland
        topology.kubernetes.io/zone: nova
      name: test-master-2
    spec:
      podCIDR: 10.111.3.0/24
      podCIDRs:
        - 10.111.3.0/24
      providerID: openstack:///e5f8357a-23c6-49a4-9dcb-08245526439f
      taints:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
    status:
      addresses:
        - address: 192.168.199.55
          type: InternalIP
        - address: 95.217.68.201
          type: ExternalIP
        - address: test-master-2
          type: Hostname
      allocatable:
        cpu: "4"
        ephemeral-storage: "19597760292"
        hugepages-2Mi: "0"
        memory: "8264978475"
        pods: "110"
      capacity:
        cpu: "4"
        ephemeral-storage: 20145724Ki
        hugepages-2Mi: "0"
        memory: 8152796Ki
        pods: "110"
      conditions:
        - lastHeartbeatTime: "2021-03-24T14:22:41Z"
          lastTransitionTime: "2021-03-24T14:22:41Z"
          message: Flannel is running on this node
          reason: FlannelIsUp
          status: "False"
          type: NetworkUnavailable
        - lastHeartbeatTime: "2021-05-13T03:54:33Z"
          lastTransitionTime: "2021-03-24T14:21:31Z"
          message: kubelet has sufficient memory available
          reason: KubeletHasSufficientMemory
          status: "False"
          type: MemoryPressure
        - lastHeartbeatTime: "2021-05-13T03:54:33Z"
          lastTransitionTime: "2021-03-24T14:21:31Z"
          message: kubelet has no disk pressure
          reason: KubeletHasNoDiskPressure
          status: "False"
          type: DiskPressure
        - lastHeartbeatTime: "2021-05-13T03:54:33Z"
          lastTransitionTime: "2021-03-24T14:21:31Z"
          message: kubelet has sufficient PID available
          reason: KubeletHasSufficientPID
          status: "False"
          type: PIDPressure
        - lastHeartbeatTime: "2021-05-13T03:54:33Z"
          lastTransitionTime: "2021-05-12T11:08:14Z"
          message: kubelet is posting ready status. AppArmor enabled
          reason: KubeletReady
          status: "True"
          type: Ready
      daemonEndpoints:
        kubeletEndpoint:
          Port: 10250
      nodeInfo:
        architecture: amd64
        bootID: 117112d7-d717-461d-b400-60ba08fd6f59
        containerRuntimeVersion: docker://19.3.13
        kernelVersion: 5.3.0-51-generic
        kubeProxyVersion: v1.19.10
        kubeletVersion: v1.19.10
        machineID: e5f8357a23c649a49dcb08245526439f
        operatingSystem: linux
        osImage: Ubuntu 18.04.3 LTS
        systemUUID: e5f8357a-23c6-49a4-9dcb-08245526439f
  - apiVersion: v1
    kind: Node
    metadata:
      annotations:
        csi.volume.kubernetes.io/nodeid: '{"cinder.csi.openstack.org":"9ecaed0b-6fd6-481b-8838-d6f87ee9b434"}'
        flannel.alpha.coreos.com/backend-data: "null"
        flannel.alpha.coreos.com/backend-type: host-gw
        flannel.alpha.coreos.com/kube-subnet-manager: "true"
        flannel.alpha.coreos.com/public-ip: 192.168.199.82
        node.alpha.kubernetes.io/ttl: "0"
        node.deckhouse.io/virtualization: kvm
        node.machine.sapcloud.io/last-applied-anno-labels-taints: '{"metadata":{"creationTimestamp":null,"labels":{"node-role.kubernetes.io/worker":"","node.deckhouse.io/group":"worker","node.deckhouse.io/type":"CloudEphemeral"}},"spec":{}}'
        update.node.deckhouse.io/waiting-for-approval: ""
        volumes.kubernetes.io/controller-managed-attach-detach: "true"
      creationTimestamp: "2020-12-18T08:38:59Z"
      labels:
        beta.kubernetes.io/arch: amd64
        beta.kubernetes.io/instance-type: 69a1095b-ceec-404f-8545-800391cdbce1
        beta.kubernetes.io/os: linux
        failure-domain.beta.kubernetes.io/region: HetznerFinland
        failure-domain.beta.kubernetes.io/zone: nova
        kubernetes.io/arch: amd64
        kubernetes.io/hostname: test-worker-e0af82d5-8d97b-zhbxg
        kubernetes.io/os: linux
        node-role.kubernetes.io/worker: ""
        node.deckhouse.io/group: worker
        node.deckhouse.io/type: CloudEphemeral
        node.kubernetes.io/instance-type: 69a1095b-ceec-404f-8545-800391cdbce1
        topology.cinder.csi.openstack.org/zone: nova
        topology.kubernetes.io/region: HetznerFinland
        topology.kubernetes.io/zone: nova
      name: test-worker-e0af82d5-8d97b-zhbxg
      resourceVersion: "172865786"
      selfLink: /api/v1/nodes/test-worker-e0af82d5-8d97b-zhbxg
      uid: bd32771e-8c35-4c76-839a-fb7974c88a79
    spec:
      podCIDR: 10.111.1.0/24
      podCIDRs:
        - 10.111.1.0/24
      providerID: openstack:///9ecaed0b-6fd6-481b-8838-d6f87ee9b434
    status:
      addresses:
        - address: 192.168.199.82
          type: InternalIP
        - address: test-worker-e0af82d5-8d97b-zhbxg
          type: Hostname
      allocatable:
        cpu: "4"
        ephemeral-storage: "19597760292"
        hugepages-1Gi: "0"
        hugepages-2Mi: "0"
        memory: "8264994695"
        pods: "110"
      capacity:
        cpu: "4"
        ephemeral-storage: 20145724Ki
        hugepages-1Gi: "0"
        hugepages-2Mi: "0"
        memory: 8152812Ki
        pods: "110"
      conditions:
        - lastHeartbeatTime: "2021-03-22T17:50:28Z"
          lastTransitionTime: "2021-03-22T17:50:28Z"
          message: Flannel is running on this node
          reason: FlannelIsUp
          status: "False"
          type: NetworkUnavailable
        - lastHeartbeatTime: "2021-05-13T03:54:32Z"
          lastTransitionTime: "2020-12-18T08:39:00Z"
          message: kubelet has sufficient memory available
          reason: KubeletHasSufficientMemory
          status: "False"
          type: MemoryPressure
        - lastHeartbeatTime: "2021-05-13T03:54:32Z"
          lastTransitionTime: "2021-02-21T02:00:56Z"
          message: kubelet has no disk pressure
          reason: KubeletHasNoDiskPressure
          status: "False"
          type: DiskPressure
        - lastHeartbeatTime: "2021-05-13T03:54:32Z"
          lastTransitionTime: "2020-12-18T08:39:00Z"
          message: kubelet has sufficient PID available
          reason: KubeletHasSufficientPID
          status: "False"
          type: PIDPressure
        - lastHeartbeatTime: "2021-05-13T03:54:32Z"
          lastTransitionTime: "2021-03-10T14:25:06Z"
          message: kubelet is posting ready status. AppArmor enabled
          reason: KubeletReady
          status: "True"
          type: Ready
      daemonEndpoints:
        kubeletEndpoint:
          Port: 10250
      nodeInfo:
        architecture: amd64
        bootID: 6b3172a0-beb8-46b3-a2d5-585a03bb9790
        containerRuntimeVersion: docker://18.9.7
        kernelVersion: 5.3.0-51-generic
        kubeProxyVersion: v1.19.8
        kubeletVersion: v1.19.8
        machineID: 9ecaed0b6fd6481b8838d6f87ee9b434
        operatingSystem: linux
        osImage: Ubuntu 18.04.3 LTS
        systemUUID: 9ecaed0b-6fd6-481b-8838-d6f87ee9b434
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""
`

	podListYAML := `
apiVersion: v1
items:
  - apiVersion: v1
    kind: Pod
    metadata:
      annotations:
        vpaObservedContainers: node-exporter, kubelet-eviction-thresholds-exporter, kube-rbac-proxy
        vpaUpdates: 'Pod resources updated by node-exporter: container 0: cpu request, memory request; container 1: cpu request, memory request; container 2: memory request, cpu request'
      creationTimestamp: "2021-05-08T14:19:41Z"
      generateName: node-exporter-
      labels:
        app: node-exporter
        controller-revision-hash: 6dfc97bff7
        pod-template-generation: "6"
      name: node-exporter-fq5fj
      namespace: d8-monitoring
      ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: DaemonSet
          name: node-exporter
          uid: b5ab1412-597d-400c-b3dd-1a94ef68e4ea
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchFields:
                  - key: metadata.name
                    operator: In
                    values:
                      - test-worker-e0af82d5-8d97b-zhbxg
      containers:
        - args:
            - --web.listen-address=127.0.0.1:9101
            - --path.rootfs=/host/root
            - --collector.ntp
            - --collector.ntp.server-is-local
            - --collector.filesystem.ignored-mount-points
            - (^/(dev|proc|sys|run|var/lib/kubelet)($|/))|(^/var/lib/docker/)
            - --collector.filesystem.ignored-fs-types
            - ^(autofs|binfmt_misc|cgroup|configfs|debugfs|devpts|devtmpfs|fusectl|fuse\.lxcfs|hugetlbfs|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|sysfs|tracefs|squashfs)$
            - --collector.textfile.directory
            - /host/textfile
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter:05e2b2361ad1d69b3ae23b369308e759540b3413834f0011e318768b
          imagePullPolicy: IfNotPresent
          name: node-exporter
          resources:
            requests:
              cpu: 9m
              ephemeral-storage: 60Mi
              memory: "20035632"
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /host/root
              name: root
              readOnly: true
            - mountPath: /host/textfile
              name: textfile
              readOnly: true
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
        - env:
            - name: MY_NODE_NAME
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: spec.nodeName
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter:455222c1194ecff9aa77c8ae0daa936260e43d420ae6e512a1026dae
          imagePullPolicy: IfNotPresent
          name: kubelet-eviction-thresholds-exporter
          resources:
            requests:
              cpu: 19m
              ephemeral-storage: 50Mi
              memory: "30810894"
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /host/
              name: root
              readOnly: true
            - mountPath: /var/run/node-exporter-textfile
              name: textfile
            - mountPath: /var/run/docker.sock
              name: dockersock
            - mountPath: /var/run/containerd/containerd.sock
              name: containerdsock
            - mountPath: /usr/local/bin/crictl
              name: crictl
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
        - args:
            - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9101
            - --client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
            - --v=2
            - --logtostderr=true
            - --stale-cache-interval=1h30m
          env:
            - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: status.podIP
            - name: KUBE_RBAC_PROXY_CONFIG
              value: |
                upstreams:
                - upstream: http://127.0.0.1:9101/metrics
                  path: /metrics
                  authorization:
                    resourceAttributes:
                      namespace: d8-monitoring
                      apiGroup: apps
                      apiVersion: v1
                      resource: daemonsets
                      subresource: prometheus-metrics
                      name: node-exporter
          image: registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy:526c5255969dcd342888947e2d3bab781eed225bb969ca0dd3bdbd30
          imagePullPolicy: IfNotPresent
          name: kube-rbac-proxy
          ports:
            - containerPort: 9101
              hostPort: 9101
              name: https-metrics
              protocol: TCP
          resources:
            requests:
              cpu: 9m
              ephemeral-storage: 50Mi
              memory: "14852516"
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
      dnsPolicy: ClusterFirst
      enableServiceLinks: true
      hostNetwork: true
      hostPID: true
      imagePullSecrets:
        - name: deckhouse-registry
      nodeName: test-worker-e0af82d5-8d97b-zhbxg
      preemptionPolicy: PreemptLowerPriority
      priority: 2000001000
      priorityClassName: system-node-critical
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext:
        runAsGroup: 0
        runAsNonRoot: false
        runAsUser: 0
      serviceAccount: node-exporter
      serviceAccountName: node-exporter
      terminationGracePeriodSeconds: 30
      tolerations:
        - key: node-role.kubernetes.io/master
        - key: dedicated.deckhouse.io
          operator: Exists
        - key: dedicated
          operator: Exists
        - key: node.kubernetes.io/not-ready
        - key: node.kubernetes.io/out-of-disk
        - key: node.kubernetes.io/memory-pressure
        - key: node.kubernetes.io/disk-pressure
        - effect: NoExecute
          key: node.kubernetes.io/not-ready
          operator: Exists
        - effect: NoExecute
          key: node.kubernetes.io/unreachable
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/disk-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/memory-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/pid-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/unschedulable
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/network-unavailable
          operator: Exists
      volumes:
        - hostPath:
            path: /var/run/node-exporter-textfile
            type: DirectoryOrCreate
          name: textfile
        - hostPath:
            path: /
            type: ""
          name: root
        - hostPath:
            path: /var/run/docker.sock
            type: ""
          name: dockersock
        - hostPath:
            path: /var/run/containerd/containerd.sock
            type: ""
          name: containerdsock
        - hostPath:
            path: /usr/local/bin/crictl
            type: ""
          name: crictl
        - name: node-exporter-token-gn2d7
          secret:
            defaultMode: 420
            secretName: node-exporter-token-gn2d7
    status:
      conditions:
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:19:41Z"
          status: "True"
          type: Initialized
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:19:44Z"
          status: "True"
          type: Ready
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:19:44Z"
          status: "True"
          type: ContainersReady
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:19:41Z"
          status: "True"
          type: PodScheduled
      containerStatuses:
        - containerID: docker://608aea933d83d64e63731eefdb6c6b4ea2e5976048f3c285d15f2bc0da76ec89
          image: registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy:526c5255969dcd342888947e2d3bab781eed225bb969ca0dd3bdbd30
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy@sha256:233267845d84dc0b09667abfc0fc63479001cb4d71159697f7f4542e2a4f64af
          lastState: {}
          name: kube-rbac-proxy
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:19:43Z"
        - containerID: docker://c6a127473c06af705e4e6d8af4bfb8c49a49f7cc3cddbea063340221327ae1c6
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter:455222c1194ecff9aa77c8ae0daa936260e43d420ae6e512a1026dae
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter@sha256:59d8f21a33bf92a5feed1667a3b56be759e91cc4d9e46563543fad6597bb5e88
          lastState: {}
          name: kubelet-eviction-thresholds-exporter
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:19:43Z"
        - containerID: docker://26c9c096aee5d6a8c69ce3344bbb7f3701d997c848ba621c64611f48ddab63da
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter:05e2b2361ad1d69b3ae23b369308e759540b3413834f0011e318768b
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter@sha256:0b6dcee73e71b22e5018cf5243888056978b1188b32b2ba084929cb1db9e946c
          lastState: {}
          name: node-exporter
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:19:42Z"
      hostIP: 192.168.199.82
      phase: Running
      podIP: 192.168.199.82
      podIPs:
        - ip: 192.168.199.82
      qosClass: Burstable
      startTime: "2021-05-08T14:19:41Z"
  - apiVersion: v1
    kind: Pod
    metadata:
      annotations:
        vpaObservedContainers: node-exporter, kubelet-eviction-thresholds-exporter, kube-rbac-proxy
        vpaUpdates: 'Pod resources updated by node-exporter: container 0: cpu request, memory request; container 1: memory request, cpu request; container 2: memory request, cpu request'
      creationTimestamp: "2021-05-08T14:19:24Z"
      generateName: node-exporter-
      labels:
        app: node-exporter
        controller-revision-hash: 6dfc97bff7
        pod-template-generation: "6"
      name: node-exporter-g8m6x
      namespace: d8-monitoring
      ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: DaemonSet
          name: node-exporter
          uid: b5ab1412-597d-400c-b3dd-1a94ef68e4ea
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchFields:
                  - key: metadata.name
                    operator: In
                    values:
                      - test-master-2
      containers:
        - args:
            - --web.listen-address=127.0.0.1:9101
            - --path.rootfs=/host/root
            - --collector.ntp
            - --collector.ntp.server-is-local
            - --collector.filesystem.ignored-mount-points
            - (^/(dev|proc|sys|run|var/lib/kubelet)($|/))|(^/var/lib/docker/)
            - --collector.filesystem.ignored-fs-types
            - ^(autofs|binfmt_misc|cgroup|configfs|debugfs|devpts|devtmpfs|fusectl|fuse\.lxcfs|hugetlbfs|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|sysfs|tracefs|squashfs)$
            - --collector.textfile.directory
            - /host/textfile
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter:05e2b2361ad1d69b3ae23b369308e759540b3413834f0011e318768b
          imagePullPolicy: IfNotPresent
          name: node-exporter
          resources:
            requests:
              cpu: 9m
              ephemeral-storage: 60Mi
              memory: "20035632"
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /host/root
              name: root
              readOnly: true
            - mountPath: /host/textfile
              name: textfile
              readOnly: true
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
        - env:
            - name: MY_NODE_NAME
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: spec.nodeName
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter:455222c1194ecff9aa77c8ae0daa936260e43d420ae6e512a1026dae
          imagePullPolicy: IfNotPresent
          name: kubelet-eviction-thresholds-exporter
          resources:
            requests:
              cpu: 19m
              ephemeral-storage: 50Mi
              memory: "30810894"
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /host/
              name: root
              readOnly: true
            - mountPath: /var/run/node-exporter-textfile
              name: textfile
            - mountPath: /var/run/docker.sock
              name: dockersock
            - mountPath: /var/run/containerd/containerd.sock
              name: containerdsock
            - mountPath: /usr/local/bin/crictl
              name: crictl
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
        - args:
            - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9101
            - --client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
            - --v=2
            - --logtostderr=true
            - --stale-cache-interval=1h30m
          env:
            - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: status.podIP
            - name: KUBE_RBAC_PROXY_CONFIG
              value: |
                upstreams:
                - upstream: http://127.0.0.1:9101/metrics
                  path: /metrics
                  authorization:
                    resourceAttributes:
                      namespace: d8-monitoring
                      apiGroup: apps
                      apiVersion: v1
                      resource: daemonsets
                      subresource: prometheus-metrics
                      name: node-exporter
          image: registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy:526c5255969dcd342888947e2d3bab781eed225bb969ca0dd3bdbd30
          imagePullPolicy: IfNotPresent
          name: kube-rbac-proxy
          ports:
            - containerPort: 9101
              hostPort: 9101
              name: https-metrics
              protocol: TCP
          resources:
            requests:
              cpu: 9m
              ephemeral-storage: 50Mi
              memory: "14852516"
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
      dnsPolicy: ClusterFirst
      enableServiceLinks: true
      hostNetwork: true
      hostPID: true
      imagePullSecrets:
        - name: deckhouse-registry
      nodeName: test-master-2
      preemptionPolicy: PreemptLowerPriority
      priority: 2000001000
      priorityClassName: system-node-critical
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext:
        runAsGroup: 0
        runAsNonRoot: false
        runAsUser: 0
      serviceAccount: node-exporter
      serviceAccountName: node-exporter
      terminationGracePeriodSeconds: 30
      tolerations:
        - key: node-role.kubernetes.io/master
        - key: dedicated.deckhouse.io
          operator: Exists
        - key: dedicated
          operator: Exists
        - key: node.kubernetes.io/not-ready
        - key: node.kubernetes.io/out-of-disk
        - key: node.kubernetes.io/memory-pressure
        - key: node.kubernetes.io/disk-pressure
        - effect: NoExecute
          key: node.kubernetes.io/not-ready
          operator: Exists
        - effect: NoExecute
          key: node.kubernetes.io/unreachable
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/disk-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/memory-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/pid-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/unschedulable
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/network-unavailable
          operator: Exists
      volumes:
        - hostPath:
            path: /var/run/node-exporter-textfile
            type: DirectoryOrCreate
          name: textfile
        - hostPath:
            path: /
            type: ""
          name: root
        - hostPath:
            path: /var/run/docker.sock
            type: ""
          name: dockersock
        - hostPath:
            path: /var/run/containerd/containerd.sock
            type: ""
          name: containerdsock
        - hostPath:
            path: /usr/local/bin/crictl
            type: ""
          name: crictl
        - name: node-exporter-token-gn2d7
          secret:
            defaultMode: 420
            secretName: node-exporter-token-gn2d7
    status:
      conditions:
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:19:25Z"
          status: "True"
          type: Initialized
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:19:28Z"
          status: "True"
          type: Ready
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:19:28Z"
          status: "True"
          type: ContainersReady
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:19:24Z"
          status: "True"
          type: PodScheduled
      containerStatuses:
        - containerID: docker://8bc0ce49bace801d3d4740102d6cf10bfdb74121bccb2ca7e3ac8381ae622d32
          image: registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy:526c5255969dcd342888947e2d3bab781eed225bb969ca0dd3bdbd30
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy@sha256:233267845d84dc0b09667abfc0fc63479001cb4d71159697f7f4542e2a4f64af
          lastState: {}
          name: kube-rbac-proxy
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:19:28Z"
        - containerID: docker://c5110045c6fca03d4486fef8475252d612226c4803a16c20f3b85412a223ae6f
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter:455222c1194ecff9aa77c8ae0daa936260e43d420ae6e512a1026dae
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter@sha256:59d8f21a33bf92a5feed1667a3b56be759e91cc4d9e46563543fad6597bb5e88
          lastState: {}
          name: kubelet-eviction-thresholds-exporter
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:19:28Z"
        - containerID: docker://4dda7d6e579fba532e3ef5b6fd4877e6d21a6e2c73ad9671b147eb84154e44de
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter:05e2b2361ad1d69b3ae23b369308e759540b3413834f0011e318768b
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter@sha256:0b6dcee73e71b22e5018cf5243888056978b1188b32b2ba084929cb1db9e946c
          lastState: {}
          name: node-exporter
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:19:27Z"
      hostIP: 192.168.199.55
      phase: Running
      podIP: 192.168.199.55
      podIPs:
        - ip: 192.168.199.55
      qosClass: Burstable
      startTime: "2021-05-08T14:19:25Z"
  - apiVersion: v1
    kind: Pod
    metadata:
      annotations:
        vpaObservedContainers: node-exporter, kubelet-eviction-thresholds-exporter, kube-rbac-proxy
        vpaUpdates: 'Pod resources updated by node-exporter: container 0: cpu request, memory request; container 1: cpu request, memory request; container 2: cpu request, memory request'
      creationTimestamp: "2021-05-08T14:18:41Z"
      generateName: node-exporter-
      labels:
        app: node-exporter
        controller-revision-hash: 6dfc97bff7
        pod-template-generation: "6"
      name: node-exporter-hrtwz
      namespace: d8-monitoring
      ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: DaemonSet
          name: node-exporter
          uid: b5ab1412-597d-400c-b3dd-1a94ef68e4ea
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchFields:
                  - key: metadata.name
                    operator: In
                    values:
                      - test-master-0
      containers:
        - args:
            - --web.listen-address=127.0.0.1:9101
            - --path.rootfs=/host/root
            - --collector.ntp
            - --collector.ntp.server-is-local
            - --collector.filesystem.ignored-mount-points
            - (^/(dev|proc|sys|run|var/lib/kubelet)($|/))|(^/var/lib/docker/)
            - --collector.filesystem.ignored-fs-types
            - ^(autofs|binfmt_misc|cgroup|configfs|debugfs|devpts|devtmpfs|fusectl|fuse\.lxcfs|hugetlbfs|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|sysfs|tracefs|squashfs)$
            - --collector.textfile.directory
            - /host/textfile
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter:05e2b2361ad1d69b3ae23b369308e759540b3413834f0011e318768b
          imagePullPolicy: IfNotPresent
          name: node-exporter
          resources:
            requests:
              cpu: 9m
              ephemeral-storage: 60Mi
              memory: "20035632"
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /host/root
              name: root
              readOnly: true
            - mountPath: /host/textfile
              name: textfile
              readOnly: true
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
        - env:
            - name: MY_NODE_NAME
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: spec.nodeName
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter:455222c1194ecff9aa77c8ae0daa936260e43d420ae6e512a1026dae
          imagePullPolicy: IfNotPresent
          name: kubelet-eviction-thresholds-exporter
          resources:
            requests:
              cpu: 19m
              ephemeral-storage: 50Mi
              memory: "30810894"
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /host/
              name: root
              readOnly: true
            - mountPath: /var/run/node-exporter-textfile
              name: textfile
            - mountPath: /var/run/docker.sock
              name: dockersock
            - mountPath: /var/run/containerd/containerd.sock
              name: containerdsock
            - mountPath: /usr/local/bin/crictl
              name: crictl
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
        - args:
            - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9101
            - --client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
            - --v=2
            - --logtostderr=true
            - --stale-cache-interval=1h30m
          env:
            - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: status.podIP
            - name: KUBE_RBAC_PROXY_CONFIG
              value: |
                upstreams:
                - upstream: http://127.0.0.1:9101/metrics
                  path: /metrics
                  authorization:
                    resourceAttributes:
                      namespace: d8-monitoring
                      apiGroup: apps
                      apiVersion: v1
                      resource: daemonsets
                      subresource: prometheus-metrics
                      name: node-exporter
          image: registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy:526c5255969dcd342888947e2d3bab781eed225bb969ca0dd3bdbd30
          imagePullPolicy: IfNotPresent
          name: kube-rbac-proxy
          ports:
            - containerPort: 9101
              hostPort: 9101
              name: https-metrics
              protocol: TCP
          resources:
            requests:
              cpu: 9m
              ephemeral-storage: 50Mi
              memory: "14852516"
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
      dnsPolicy: ClusterFirst
      enableServiceLinks: true
      hostNetwork: true
      hostPID: true
      imagePullSecrets:
        - name: deckhouse-registry
      nodeName: test-master-0
      preemptionPolicy: PreemptLowerPriority
      priority: 2000001000
      priorityClassName: system-node-critical
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext:
        runAsGroup: 0
        runAsNonRoot: false
        runAsUser: 0
      serviceAccount: node-exporter
      serviceAccountName: node-exporter
      terminationGracePeriodSeconds: 30
      tolerations:
        - key: node-role.kubernetes.io/master
        - key: dedicated.deckhouse.io
          operator: Exists
        - key: dedicated
          operator: Exists
        - key: node.kubernetes.io/not-ready
        - key: node.kubernetes.io/out-of-disk
        - key: node.kubernetes.io/memory-pressure
        - key: node.kubernetes.io/disk-pressure
        - effect: NoExecute
          key: node.kubernetes.io/not-ready
          operator: Exists
        - effect: NoExecute
          key: node.kubernetes.io/unreachable
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/disk-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/memory-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/pid-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/unschedulable
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/network-unavailable
          operator: Exists
      volumes:
        - hostPath:
            path: /var/run/node-exporter-textfile
            type: DirectoryOrCreate
          name: textfile
        - hostPath:
            path: /
            type: ""
          name: root
        - hostPath:
            path: /var/run/docker.sock
            type: ""
          name: dockersock
        - hostPath:
            path: /var/run/containerd/containerd.sock
            type: ""
          name: containerdsock
        - hostPath:
            path: /usr/local/bin/crictl
            type: ""
          name: crictl
        - name: node-exporter-token-gn2d7
          secret:
            defaultMode: 420
            secretName: node-exporter-token-gn2d7
    status:
      conditions:
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:18:41Z"
          status: "True"
          type: Initialized
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:18:45Z"
          status: "True"
          type: Ready
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:18:45Z"
          status: "True"
          type: ContainersReady
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:18:41Z"
          status: "True"
          type: PodScheduled
      containerStatuses:
        - containerID: docker://3c69821c8fe5272235c3f41a59f9e6f6143608bccbd4f0c2942f84532d0bf479
          image: registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy:526c5255969dcd342888947e2d3bab781eed225bb969ca0dd3bdbd30
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy@sha256:233267845d84dc0b09667abfc0fc63479001cb4d71159697f7f4542e2a4f64af
          lastState: {}
          name: kube-rbac-proxy
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:18:44Z"
        - containerID: docker://e6b8a95f2fa4a732c2b98f8e2cec4b31823c7c7c0a87aacb6fcf2f70041ec8e5
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter:455222c1194ecff9aa77c8ae0daa936260e43d420ae6e512a1026dae
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter@sha256:59d8f21a33bf92a5feed1667a3b56be759e91cc4d9e46563543fad6597bb5e88
          lastState: {}
          name: kubelet-eviction-thresholds-exporter
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:18:43Z"
        - containerID: docker://4f12f5705354fc0e93d2b44d372821a28de1a768819577b9240883ae3811abc3
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter:05e2b2361ad1d69b3ae23b369308e759540b3413834f0011e318768b
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter@sha256:0b6dcee73e71b22e5018cf5243888056978b1188b32b2ba084929cb1db9e946c
          lastState: {}
          name: node-exporter
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:18:43Z"
      hostIP: 192.168.199.224
      phase: Running
      podIP: 192.168.199.224
      podIPs:
        - ip: 192.168.199.224
      qosClass: Burstable
      startTime: "2021-05-08T14:18:41Z"
  - apiVersion: v1
    kind: Pod
    metadata:
      annotations:
        vpaObservedContainers: node-exporter, kubelet-eviction-thresholds-exporter, kube-rbac-proxy
        vpaUpdates: 'Pod resources updated by node-exporter: container 0: cpu request, memory request; container 1: cpu request, memory request; container 2: cpu request, memory request'
      creationTimestamp: "2021-05-08T14:18:22Z"
      generateName: node-exporter-
      labels:
        app: node-exporter
        controller-revision-hash: 6dfc97bff7
        pod-template-generation: "6"
      name: node-exporter-x6qwz
      namespace: d8-monitoring
      ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: DaemonSet
          name: node-exporter
          uid: b5ab1412-597d-400c-b3dd-1a94ef68e4ea
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchFields:
                  - key: metadata.name
                    operator: In
                    values:
                      - test-master-1
      containers:
        - args:
            - --web.listen-address=127.0.0.1:9101
            - --path.rootfs=/host/root
            - --collector.ntp
            - --collector.ntp.server-is-local
            - --collector.filesystem.ignored-mount-points
            - (^/(dev|proc|sys|run|var/lib/kubelet)($|/))|(^/var/lib/docker/)
            - --collector.filesystem.ignored-fs-types
            - ^(autofs|binfmt_misc|cgroup|configfs|debugfs|devpts|devtmpfs|fusectl|fuse\.lxcfs|hugetlbfs|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|sysfs|tracefs|squashfs)$
            - --collector.textfile.directory
            - /host/textfile
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter:05e2b2361ad1d69b3ae23b369308e759540b3413834f0011e318768b
          imagePullPolicy: IfNotPresent
          name: node-exporter
          resources:
            requests:
              cpu: 9m
              ephemeral-storage: 60Mi
              memory: "20035632"
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /host/root
              name: root
              readOnly: true
            - mountPath: /host/textfile
              name: textfile
              readOnly: true
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
        - env:
            - name: MY_NODE_NAME
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: spec.nodeName
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter:455222c1194ecff9aa77c8ae0daa936260e43d420ae6e512a1026dae
          imagePullPolicy: IfNotPresent
          name: kubelet-eviction-thresholds-exporter
          resources:
            requests:
              cpu: 19m
              ephemeral-storage: 50Mi
              memory: "30810894"
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /host/
              name: root
              readOnly: true
            - mountPath: /var/run/node-exporter-textfile
              name: textfile
            - mountPath: /var/run/docker.sock
              name: dockersock
            - mountPath: /var/run/containerd/containerd.sock
              name: containerdsock
            - mountPath: /usr/local/bin/crictl
              name: crictl
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
        - args:
            - --secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9101
            - --client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
            - --v=2
            - --logtostderr=true
            - --stale-cache-interval=1h30m
          env:
            - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: status.podIP
            - name: KUBE_RBAC_PROXY_CONFIG
              value: |
                upstreams:
                - upstream: http://127.0.0.1:9101/metrics
                  path: /metrics
                  authorization:
                    resourceAttributes:
                      namespace: d8-monitoring
                      apiGroup: apps
                      apiVersion: v1
                      resource: daemonsets
                      subresource: prometheus-metrics
                      name: node-exporter
          image: registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy:526c5255969dcd342888947e2d3bab781eed225bb969ca0dd3bdbd30
          imagePullPolicy: IfNotPresent
          name: kube-rbac-proxy
          ports:
            - containerPort: 9101
              hostPort: 9101
              name: https-metrics
              protocol: TCP
          resources:
            requests:
              cpu: 9m
              ephemeral-storage: 50Mi
              memory: "14852516"
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
            - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
              name: node-exporter-token-gn2d7
              readOnly: true
      dnsPolicy: ClusterFirst
      enableServiceLinks: true
      hostNetwork: true
      hostPID: true
      imagePullSecrets:
        - name: deckhouse-registry
      nodeName: test-master-1
      preemptionPolicy: PreemptLowerPriority
      priority: 2000001000
      priorityClassName: system-node-critical
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext:
        runAsGroup: 0
        runAsNonRoot: false
        runAsUser: 0
      serviceAccount: node-exporter
      serviceAccountName: node-exporter
      terminationGracePeriodSeconds: 30
      tolerations:
        - key: node-role.kubernetes.io/master
        - key: dedicated.deckhouse.io
          operator: Exists
        - key: dedicated
          operator: Exists
        - key: node.kubernetes.io/not-ready
        - key: node.kubernetes.io/out-of-disk
        - key: node.kubernetes.io/memory-pressure
        - key: node.kubernetes.io/disk-pressure
        - effect: NoExecute
          key: node.kubernetes.io/not-ready
          operator: Exists
        - effect: NoExecute
          key: node.kubernetes.io/unreachable
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/disk-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/memory-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/pid-pressure
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/unschedulable
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/network-unavailable
          operator: Exists
      volumes:
        - hostPath:
            path: /var/run/node-exporter-textfile
            type: DirectoryOrCreate
          name: textfile
        - hostPath:
            path: /
            type: ""
          name: root
        - hostPath:
            path: /var/run/docker.sock
            type: ""
          name: dockersock
        - hostPath:
            path: /var/run/containerd/containerd.sock
            type: ""
          name: containerdsock
        - hostPath:
            path: /usr/local/bin/crictl
            type: ""
          name: crictl
        - name: node-exporter-token-gn2d7
          secret:
            defaultMode: 420
            secretName: node-exporter-token-gn2d7
    status:
      conditions:
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:18:22Z"
          status: "True"
          type: Initialized
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:18:25Z"
          status: "True"
          type: Ready
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:18:25Z"
          status: "True"
          type: ContainersReady
        - lastProbeTime: null
          lastTransitionTime: "2021-05-08T14:18:22Z"
          status: "True"
          type: PodScheduled
      containerStatuses:
        - containerID: docker://04a332e41fd3eea06f046d9d4d47c23c0e7defb22b17d1eb61e740b23f94baad
          image: registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy:526c5255969dcd342888947e2d3bab781eed225bb969ca0dd3bdbd30
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/common/kube-rbac-proxy@sha256:233267845d84dc0b09667abfc0fc63479001cb4d71159697f7f4542e2a4f64af
          lastState: {}
          name: kube-rbac-proxy
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:18:24Z"
        - containerID: docker://6491c1fe866c951c1f91f4c29105885e95d5652de640068f20a767ac36c5d27d
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter:455222c1194ecff9aa77c8ae0daa936260e43d420ae6e512a1026dae
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/kubelet-eviction-thresholds-exporter@sha256:59d8f21a33bf92a5feed1667a3b56be759e91cc4d9e46563543fad6597bb5e88
          lastState: {}
          name: kubelet-eviction-thresholds-exporter
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:18:24Z"
        - containerID: docker://9dcf7b850f708d92620ab70887ddd8ce7b03bc0dc177b5195b4da99557ebd228
          image: registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter:05e2b2361ad1d69b3ae23b369308e759540b3413834f0011e318768b
          imageID: docker-pullable://registry.deckhouse.io/deckhouse/ce/monitoring-kubernetes/node-exporter@sha256:0b6dcee73e71b22e5018cf5243888056978b1188b32b2ba084929cb1db9e946c
          lastState: {}
          name: node-exporter
          ready: true
          restartCount: 0
          started: true
          state:
            running:
              startedAt: "2021-05-08T14:18:23Z"
      hostIP: 192.168.199.94
      phase: Running
      podIP: 192.168.199.94
      podIPs:
        - ip: 192.168.199.94
      qosClass: Burstable
      startTime: "2021-05-08T14:18:22Z"
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""
`

	var ds appsv1.DaemonSet
	if err := yaml.Unmarshal([]byte(daemonSetYAML), &ds); err != nil {
		panic("ds did not parse: " + err.Error())
	}

	var nodeList v1.NodeList
	if err := yaml.Unmarshal([]byte(nodeListYAML), &nodeList); err != nil {
		panic("node list did not parse: " + err.Error())
	}

	var podList v1.PodList
	if err := yaml.Unmarshal([]byte(podListYAML), &podList); err != nil {
		panic("pod list did not parse: " + err.Error())
	}

	return nodeList, ds, podList
}
