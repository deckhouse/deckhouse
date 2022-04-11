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

func (a *FakeAccess) Kubernetes() kube.KubernetesClient {
	return a.client
}

func (a *FakeAccess) ServiceAccountToken() string {
	return "pewpew"
}

func (a *FakeAccess) UserAgent() string {
	return "UpmeterTestClient/1.0"
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

func (a *FakeAccess) ClusterDomain() string {
	return ""
}

func getTestDsPodsReadinessChecker() (*fake.FakeCluster, *dsPodsReadinessChecker) {
	cluster := fake.NewFakeCluster()
	access := NewFake(cluster.KubeClient)

	dsChecker := &dsPodsReadinessChecker{
		access:          access,
		namespace:       "d8-monitoring",
		name:            "node-exporter",
		requestTimeout:  5 * time.Second,
		creationTimeout: time.Minute,
		deletionTimeout: 5 * time.Second,
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

				extraNode := nodeList.Items[0]
				extraNode.ObjectMeta.SetName("abracadabra")
				nodeList.Items = append(nodeList.Items, extraNode)

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
					Key:   "kk",
					Value: "vv",
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
				makePodNotReady(unhealthy)

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
				makePodNotReady(unhealthy)

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
		{
			name: "not-ready pending pod for not too long is fine",
			err:  nil,
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				unhealthy := &podList.Items[2]
				unhealthy.Status.Phase = v1.PodPending
				makePodNotReady(unhealthy)
				unhealthy.CreationTimestamp = metav1.NewTime(time.Now().Add(-checker.creationTimeout + 10*time.Millisecond))

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "not-ready running pod for not too long is fine",
			err:  nil,
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				unhealthy := &podList.Items[2]
				unhealthy.Status.Phase = v1.PodRunning
				makePodNotReady(unhealthy)
				unhealthy.CreationTimestamp = metav1.NewTime(time.Now().Add(-checker.creationTimeout + 10*time.Millisecond))

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "terminating pod for not too long is fine",
			err:  nil,
			setup: func(cluster *fake.FakeCluster, checker *dsPodsReadinessChecker) {
				nodeList, daemonSet, podList := getTestingClusterSetupForDaemonsSetChecker()

				unhealthy := &podList.Items[2]
				makePodNotReady(unhealthy)
				preDeadline := metav1.NewTime(time.Now().Add(-checker.deletionTimeout + 10*time.Millisecond))
				unhealthy.DeletionTimestamp = &preDeadline

				if err := setupTestingClusterForDaemonsSetChecker(cluster, checker.namespace, nodeList, daemonSet, podList); err != nil {
					panic(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runDaemonsetCheckerWithCluster(tt.setup)

			// not expecting error
			if tt.err == nil {
				if err == nil {
					return
				}
				t.Errorf("expected success, got error status=%q, err=%q", err.Status(), err.Error())
				return
			}

			// expecting error
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
		return fmt.Errorf("DaemonSet did not create: %v", err)
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

func makePodNotReady(pod *v1.Pod) {
	for i, cond := range pod.Status.Conditions {
		if cond.Type != v1.PodReady {
			continue
		}
		pod.Status.Conditions[i].Status = v1.ConditionFalse
	}
}

// This is a consistent state of four working nodes (3×M+W), node-exporter daemonset, and four working pods
func getTestingClusterSetupForDaemonsSetChecker() (v1.NodeList, appsv1.DaemonSet, v1.PodList) {
	daemonSetYAML := `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: node-exporter
  name: node-exporter
  namespace: d8-monitoring
spec:
  selector:
    matchLabels:
      app: node-exporter
  template:
    metadata:
      labels:
        app: node-exporter
      name: node-exporter
    spec:
      containers:
        - {}
        - {}
        - {}
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
      creationTimestamp: "2020-12-18T08:38:59Z"
      name: test-master-0
    spec:
      taints:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
    status:
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
  - apiVersion: v1
    kind: Node
    metadata:
      creationTimestamp: "2020-12-18T08:38:59Z"
      name: test-master-1
    spec:
      taints:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
    status:
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
  - apiVersion: v1
    kind: Node
    metadata:
      creationTimestamp: "2020-12-18T08:38:59Z"
      name: test-master-2
    spec:
      taints:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
    status:
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
  - apiVersion: v1
    kind: Node
    metadata:
      creationTimestamp: "2020-12-18T08:38:59Z"
      name: test-worker-e0af82d5-8d97b-zhbxg
    spec: {}
    status:
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
      creationTimestamp: "2021-05-08T14:19:41Z"
      generateName: node-exporter-
      labels:
        app: node-exporter
      name: node-exporter-fq5fj
      namespace: d8-monitoring
      ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: DaemonSet
          name: node-exporter
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
        - {}
        - {}
        - {}
      nodeName: test-worker-e0af82d5-8d97b-zhbxg
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
      phase: Running
      startTime: "2021-05-08T14:19:41Z"
  - apiVersion: v1
    kind: Pod
    metadata:
      creationTimestamp: "2021-05-08T14:19:24Z"
      generateName: node-exporter-
      labels:
        app: node-exporter
      name: node-exporter-g8m6x
      namespace: d8-monitoring
      ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: DaemonSet
          name: node-exporter
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
        - {}
        - {}
        - {}
      nodeName: test-master-2
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
      creationTimestamp: "2021-05-08T14:18:41Z"
      generateName: node-exporter-
      labels:
        app: node-exporter
      name: node-exporter-hrtwz
      namespace: d8-monitoring
      ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: DaemonSet
          name: node-exporter
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
        - {}
        - {}
        - {}
      nodeName: test-master-0
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
      creationTimestamp: "2021-05-08T14:18:22Z"
      labels:
        app: node-exporter
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
        - {}
        - {}
        - {}
      nodeName: test-master-1
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
