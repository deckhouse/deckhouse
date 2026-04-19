// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resources

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func TestResourceReadinessChecker(t *testing.T) {
	t.Run("Simple resources should return `ready` after 1 attempt", func(t *testing.T) {
		type test struct {
			kind         string
			resourceYAML string
		}

		tests := []test{
			{
				kind: "Configmap",
				resourceYAML: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: dotfile-cm
  namespace: default
data:
  .file: "content"
`,
			},
			{
				kind: "Secret",
				resourceYAML: `
apiVersion: v1
kind: Secret
metadata:
  name: tst-empty
  namespace: default
type: Opaque
`,
			},
			{
				kind: "DaemonSet",
				resourceYAML: `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: chrony
  namespace: default
spec:
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: chrony
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: chrony
        tier: node
    spec:
      containers:
      - image: image:latest
        name: app
        args:
        - --arg=val
      priorityClassName: cluster-medium
      restartPolicy: Always
      schedulerName: default-scheduler
      terminationGracePeriodSeconds: 30
status:
  currentNumberScheduled: 1
  desiredNumberScheduled: 1
  numberMisscheduled: 0
  numberReady: 1
  observedGeneration: 42
`,
			},
			{
				kind: "Job",
				resourceYAML: `
apiVersion: batch/v1
kind: Job
metadata:
  annotations:
    batch.kubernetes.io/job-tracking: ""
  creationTimestamp: "2024-12-09T21:42:00Z"
  generation: 1
  labels:
    batch.kubernetes.io/job-name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3-28896342
  name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3-28896342
  namespace: kube-system
  resourceVersion: "1192489206"
  uid: bfc9952e-862e-4310-802b-f6beea96ad5f
spec:
  backoffLimit: 0
  completionMode: NonIndexed
  completions: 1
  parallelism: 1
  selector:
    matchLabels:
      batch.kubernetes.io/controller-uid: bfc9952e-862e-4310-802b-f6beea96ad5f
  suspend: false
  template:
    metadata:
      labels:
        batch.kubernetes.io/controller-uid: bfc9952e-862e-4310-802b-f6beea96ad5f
        batch.kubernetes.io/job-name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3-28896342
        job-name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3-28896342
    spec:
      containers:
      - image: alpine:latest
        name: backup
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirstWithHostNet
      nodeSelector:
        kubernetes.io/hostname: sandbox-master-0
      restartPolicy: Never
status:
  completionTime: "2024-12-09T21:42:30Z"
  conditions:
  - lastProbeTime: "2024-12-09T21:42:30Z"
    lastTransitionTime: "2024-12-09T21:42:30Z"
    status: "False"
    type: Complete
  ready: 0
  startTime: "2024-12-09T21:42:00Z"
  succeeded: 0
  uncountedTerminatedPods: {}
`,
			},
		}

		for _, tst := range tests {
			t.Run(tst.kind, func(t *testing.T) {
				checker := testResourceReadinessChecker(t, tst.resourceYAML)
				assertCheckResourceReady(t, checker, false, 1, false)
				assertCheckResourceReady(t, checker, true, 2, true)
			})
		}
	})

	t.Run("Static instance should return correct status after 1 attempt", func(t *testing.T) {
		type test struct {
			phase string
			ready bool
		}

		assertStaticInstance := func(t *testing.T, ready bool, resourceYAML string) {
			checker := testResourceReadinessChecker(t, resourceYAML)
			assertCheckResourceReady(t, checker, false, 1, false)
			assertCheckResourceReady(t, checker, ready, 2, true)
		}

		t.Run("without status", func(t *testing.T) {
			assertStaticInstance(t, false, `
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: master-0
spec:
  address: 192.168.199.233
  credentialsRef:
    kind: SSHCredentials
    name: credentials
`)
		})

		tests := []test{
			{
				phase: "Pending",
				ready: false,
			},
			{
				phase: "Bootstrapping",
				ready: false,
			},
			{
				phase: "Cleaning",
				ready: false,
			},
			{
				phase: "Error",
				ready: false,
			},
			{
				phase: "Running",
				ready: true,
			},
		}

		contentFmt := `
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: master-0
spec:
  address: 192.168.199.233
  credentialsRef:
    kind: SSHCredentials
    name: credentials
status:
  conditions:
  - lastTransitionTime: "2024-12-09T21:26:45Z"
    status: "True"
    type: AddedToNodeGroup
  - lastTransitionTime: "2024-12-09T21:26:45Z"
    status: "True"
    type: WaitingForCredentialsRefToBeAssigned
  currentStatus:
    lastUpdateTime: "2024-12-09T21:26:45Z"
    phase: %s
`

		for _, tst := range tests {
			t.Run(tst.phase, func(t *testing.T) {
				assertStaticInstance(t, tst.ready, fmt.Sprintf(contentFmt, tst.phase))
			})
		}
	})

	t.Run("Node group should return correct status after 5 attempts", func(t *testing.T) {
		generateStatus := func(readyStatus string) string {
			generateConditionReady := func(status string) string {
				condition := ""
				if status != "" {
					condition = fmt.Sprintf(`
  - lastTransitionTime: "2024-08-17T19:30:21Z"
    status: "%s"
    type: Ready
`, status)
				}
				return condition
			}

			return fmt.Sprintf(`
status:
  conditionSummary:
    ready: "True"
    statusMessage: ""
  conditions:
%s
  - lastTransitionTime: "2024-08-19T14:49:18Z"
    status: "False"
    type: Updating
  - lastTransitionTime: "2024-08-17T19:30:21Z"
    status: "False"
    type: WaitingForDisruptiveApproval
  - lastTransitionTime: "2024-08-17T19:30:21Z"
    status: "False"
    type: Error
  deckhouse:
    observed:
      checkSum: 192a4757872c64c03feed7563070bea0
      lastTimestamp: "2024-12-09T21:30:00Z"
    processed:
      checkSum: 192a4757872c64c03feed7563070bea0
      lastTimestamp: "2024-12-09T20:00:46Z"
      synced: "True"
  error: ""
  kubernetesVersion: "1.32"
  nodes: 1
  ready: 1
  upToDate: 1
`, generateConditionReady(readyStatus))
		}

		type test struct {
			name                 string
			readyConditionStatus string
			ready                bool
		}

		tests := []test{
			{
				name:                 "without status",
				readyConditionStatus: "",
				ready:                false,
			},
			{
				name:                 "without ready condition",
				readyConditionStatus: generateStatus(""),
				ready:                false,
			},
			{
				name:                 "false ready condition",
				readyConditionStatus: generateStatus("False"),
				ready:                false,
			},
			{
				name:                 "true ready condition",
				readyConditionStatus: generateStatus("True"),
				ready:                true,
			},
		}

		contentFmt := `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static
spec:
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: Auto
  nodeType: Static
  staticInstances:
    count: 1
%s
`

		for _, tst := range tests {
			t.Run(tst.name, func(t *testing.T) {
				checker := testResourceReadinessChecker(t, fmt.Sprintf(contentFmt, tst.readyConditionStatus))
				assertResourceReadyAfterAttempts(t, checker, 5, tst.ready)
			})
		}
	})

	t.Run("Deployment should return correct status after 3 attempts", func(t *testing.T) {
		const deploymentFmt = `
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "248"
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2021-03-18T13:41:29Z"
  generation: 249
  labels:
    app: webhook-handler
  name: webhook-handler
  namespace: d8-system
  resourceVersion: "1189577736"
  uid: 77e6d2e9-73a3-4b71-a6e4-cb7270eb238b
spec:
  progressDeadlineSeconds: 600
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: webhook-handler
  template:
    metadata:
      annotations:
        kubectl.kubernetes.io/restartedAt: "2023-06-26T11:14:40Z"
      labels:
        app: webhook-handler
    spec:
      containers:
        image: alpine:latest
        imagePullPolicy: IfNotPresent
        name: handler
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      restartPolicy: Always
      schedulerName: default-scheduler
      terminationGracePeriodSeconds: 30
status:
  availableReplicas: 1
  conditions:
  - lastTransitionTime: "2024-04-09T16:06:46Z"
    lastUpdateTime: "2024-04-09T16:06:46Z"
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailable
    status: "%s"
    type: Available
  - lastTransitionTime: "2021-03-18T13:41:29Z"
    lastUpdateTime: "2024-12-02T21:22:11Z"
    message: ReplicaSet "webhook-handler-599688f598" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing
  observedGeneration: 249
  readyReplicas: 1
  replicas: 1
  updatedReplicas: 1
`

		type test struct {
			availableConditionStatus string
			ready                    bool
		}

		tests := []test{
			{
				availableConditionStatus: "False",
				ready:                    false,
			},
			{
				availableConditionStatus: "True",
				ready:                    true,
			},
		}

		for _, tst := range tests {
			t.Run(tst.availableConditionStatus, func(t *testing.T) {
				checker := testResourceReadinessChecker(t, fmt.Sprintf(deploymentFmt, tst.availableConditionStatus))
				assertResourceReadyAfterAttempts(t, checker, 3, tst.ready)
			})
		}
	})

	t.Run("APIService should return correct status after 2 attempts", func(t *testing.T) {
		const apiServiceFmt = `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  creationTimestamp: "2024-12-17T08:13:37Z"
  labels:
    kube-aggregator.kubernetes.io/automanaged: "true"
  name: v2.cilium.io
  resourceVersion: "1199701868"
  uid: dbae65df-a9b3-4768-9853-14d99e80ba4c
spec:
  group: cilium.io
  groupPriorityMinimum: 1000
  version: v2
  versionPriority: 100
status:
  conditions:
  - lastTransitionTime: "2024-12-17T08:13:37Z"
    message: Local APIServices are always available
    reason: Local
    status: "%s"
    type: Available
`

		type test struct {
			availableConditionStatus string
			ready                    bool
		}

		tests := []test{
			{
				availableConditionStatus: "False",
				ready:                    false,
			},
			{
				availableConditionStatus: "True",
				ready:                    true,
			},
		}

		for _, tst := range tests {
			t.Run(tst.availableConditionStatus, func(t *testing.T) {
				checker := testResourceReadinessChecker(t, fmt.Sprintf(apiServiceFmt, tst.availableConditionStatus))
				assertResourceReadyAfterAttempts(t, checker, 2, tst.ready)
			})
		}
	})

	t.Run("Pod should return correct status after 3 attempts", func(t *testing.T) {
		assertPod := func(t *testing.T, ready bool, resourceYAML string) {
			checker := testResourceReadinessChecker(t, resourceYAML)
			assertResourceReadyAfterAttempts(t, checker, 3, ready)
		}

		const podFmt = `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2024-08-20T11:23:50Z"
  name: nettools
  namespace: default
  resourceVersion: "1275777278"
  uid: aa541bfe-73d7-4ad7-a6b1-283138456e21
spec:
  containers:
  - command:
    - sleep
    - "3600"
    image: jrecord/nettools:latest
    imagePullPolicy: IfNotPresent
    name: nettools
    resources: {}
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-5cczl
      readOnly: true
  dnsPolicy: ClusterFirst
  enableServiceLinks: true
  nodeName: sandbox-master-0
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  preemptionPolicy: PreemptLowerPriority
  priority: 1000
  priorityClassName: develop
  restartPolicy: Never
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: default
  serviceAccountName: default
  terminationGracePeriodSeconds: 30
  tolerations:
  - operator: Exists
%s
`
		const statusFmt = `
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2025-03-10T18:17:59Z"
    status: "False"
    type: PodReadyToStartContainers
  - lastProbeTime: null
    lastTransitionTime: "2024-08-20T11:23:50Z"
    reason: PodCompleted
    status: "True"
    type: Initialized
  - lastProbeTime: null
    lastTransitionTime: "2024-08-20T12:24:18Z"
    reason: PodCompleted
    status: "False"
    type: Ready
  - lastProbeTime: null
    lastTransitionTime: "2024-08-20T12:24:18Z"
    reason: PodCompleted
    status: "False"
    type: ContainersReady
  - lastProbeTime: null
    lastTransitionTime: "2024-08-20T11:23:50Z"
    status: "True"
    type: PodScheduled
  containerStatuses:
  - containerID: containerd://7adf9d3ab43af63b4c1d86b34440ca53b2a00a19f8bffd9513c898464167befe
    image: docker.io/jrecord/nettools:latest
    imageID: docker.io/jrecord/nettools@sha256:d6b2f71f5c41ea4f55b187eb0d2f074ad97b080f100f68e82e53414e6636425b
    lastState: {}
    name: nettools
    ready: false
    restartCount: 0
    started: false
    state:
      terminated:
        containerID: containerd://7adf9d3ab43af63b4c1d86b34440ca53b2a00a19f8bffd9513c898464167befe
        exitCode: 0
        finishedAt: "2024-08-20T12:24:17Z"
        reason: Completed
        startedAt: "2024-08-20T11:24:17Z"
  hostIP: 192.168.199.169
  hostIPs:
  - ip: 192.168.199.169
  phase: %s
  qosClass: BestEffort
  startTime: "2024-08-20T11:23:50Z"
`
		t.Run("without status", func(t *testing.T) {
			assertPod(t, false, fmt.Sprintf(podFmt, ""))
		})

		type test struct {
			phase string
			ready bool
		}

		tests := []test{
			{
				phase: "Pending",
				ready: false,
			},
			{
				phase: "Failed",
				ready: false,
			},
			{
				phase: "Incorrect",
				ready: false,
			},
			{
				phase: "Succeeded",
				ready: true,
			},
			{
				phase: "Running",
				ready: true,
			},
		}

		for _, tst := range tests {
			t.Run(tst.phase, func(t *testing.T) {
				status := fmt.Sprintf(statusFmt, tst.phase)
				checker := testResourceReadinessChecker(t, fmt.Sprintf(podFmt, status))
				assertResourceReadyAfterAttempts(t, checker, 3, tst.ready)
			})
		}
	})

	t.Run("PVC should return correct status after 3 attempts", func(t *testing.T) {
		const pvcFmt = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: cinder.csi.openstack.org
    volume.kubernetes.io/selected-node: sandbox-cluster-api-1-8ef4a622-n58dx-m6dbf
    volume.kubernetes.io/storage-provisioner: cinder.csi.openstack.org
  creationTimestamp: "2025-05-04T16:07:54Z"
  finalizers:
  - kubernetes.io/pvc-protection
  labels:
    app: nginx
  name: www-web-0
  namespace: default
  resourceVersion: "1342526443"
  uid: 88dd79ea-55c9-42ab-a528-2741d5c550cd
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ceph-ssd
  volumeMode: Filesystem
  volumeName: pvc-88dd79ea-55c9-42ab-a528-2741d5c550cd
status:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  phase: %s
`
		type test struct {
			phase string
			ready bool
		}

		tests := []test{
			{
				phase: "Pending",
				ready: false,
			},
			{
				phase: "Lost",
				ready: false,
			},
			{
				phase: "Incorrect",
				ready: false,
			},
			{
				phase: "Bound",
				ready: true,
			},
		}

		for _, tst := range tests {
			t.Run(tst.phase, func(t *testing.T) {
				checker := testResourceReadinessChecker(t, fmt.Sprintf(pvcFmt, tst.phase))
				assertResourceReadyAfterAttempts(t, checker, 3, tst.ready)
			})
		}
	})

	t.Run("PV should return correct status after 3 attempts", func(t *testing.T) {
		const pvFmt = `
apiVersion: v1
kind: PersistentVolume
metadata:
  annotations:
    pv.kubernetes.io/provisioned-by: cinder.csi.openstack.org
    volume.kubernetes.io/provisioner-deletion-secret-name: ""
    volume.kubernetes.io/provisioner-deletion-secret-namespace: ""
  creationTimestamp: "2025-05-04T16:07:55Z"
  finalizers:
  - external-provisioner.volume.kubernetes.io/finalizer
  - kubernetes.io/pv-protection
  - external-attacher/cinder-csi-openstack-org
  name: pvc-88dd79ea-55c9-42ab-a528-2741d5c550cd
  resourceVersion: "1342526450"
  uid: 46965f94-1b9e-421c-b967-ed9ebfe1c15e
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: www-web-0
    namespace: default
    resourceVersion: "1342526429"
    uid: 88dd79ea-55c9-42ab-a528-2741d5c550cd
  csi:
    driver: cinder.csi.openstack.org
    fsType: ext4
    volumeAttributes:
      storage.kubernetes.io/csiProvisionerIdentity: 1746288888488-5774-cinder.csi.openstack.org
    volumeHandle: 14c61ff2-eea5-4044-ae3c-a615ade6eb96
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: topology.cinder.csi.openstack.org/zone
          operator: In
          values:
          - nova
  persistentVolumeReclaimPolicy: Delete
  storageClassName: ceph-ssd
  volumeMode: Filesystem
status:
  lastPhaseTransitionTime: "2025-05-04T16:07:55Z"
  phase: %s
`
		type test struct {
			phase string
			ready bool
		}

		tests := []test{
			{
				phase: "Released",
				ready: false,
			},
			{
				phase: "Failed",
				ready: false,
			},
			{
				phase: "Incorrect",
				ready: false,
			},
			{
				phase: "Available",
				ready: true,
			},
			{
				phase: "Bound",
				ready: true,
			},
		}

		for _, tst := range tests {
			t.Run(tst.phase, func(t *testing.T) {
				checker := testResourceReadinessChecker(t, fmt.Sprintf(pvFmt, tst.phase))
				assertResourceReadyAfterAttempts(t, checker, 3, tst.ready)
			})
		}
	})

	t.Run("Not known resource should return true or by condition Ready/Available after 3 attempts", func(t *testing.T) {
		const certFmt = `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  creationTimestamp: "2023-05-24T14:51:55Z"
  generation: 4
  labels:
    app: documentation
    heritage: deckhouse
    module: documentation
  name: documentation
  namespace: d8-system
  resourceVersion: "1403818629"
  uid: 2ab33a76-5582-4f29-be71-c48488ca983b
spec:
  certificateOwnerRef: false
  commonName: documentation.example.com
  dnsNames:
  - documentation.example.com
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt
  secretName: ingress-tls
%s
`
		const statusWithConditionsFmt = `
status:
  conditions:
%s
  - lastTransitionTime: "2025-05-09T18:28:20Z"
    message: Renewing certificate as renewal was scheduled at 2025-05-09 18:28:20
      +0000 UTC
    observedGeneration: 4
    reason: Renewing
    status: "True"
    type: Issuing
  nextPrivateKeySecretName: documentation-65gqj
  notAfter: "2025-06-08T18:28:20Z"
  notBefore: "2025-03-10T18:28:21Z"
  renewalTime: "2025-05-09T18:28:20Z"
  revision: 7
`
		const conditionFmt = `
  - lastTransitionTime: "2025-06-19T11:21:13Z"
    message: Certificate expired on Sun, 08 Jun 2025 18:28:20 UTC
    observedGeneration: 4
    reason: Expired
    status: "%s"
    type: %s
`
		generateStatusWithConditions := func(conditions map[string]string) string {
			conditionsList := make([]string, 0, len(conditions))
			for tp, status := range conditions {
				conditionsList = append(conditionsList, fmt.Sprintf(conditionFmt, status, tp))
			}

			conditionsStr := strings.Join(conditionsList, "\n")

			return fmt.Sprintf(statusWithConditionsFmt, conditionsStr)
		}

		type test struct {
			name   string
			status string
			ready  bool
		}

		tests := []test{
			{
				name:   "without status",
				status: "",
				ready:  true,
			},
			{
				name:   "with empty status",
				status: "status: {}",
				ready:  true,
			},
			{
				name: "without conditions",
				status: `
status:
  nextPrivateKeySecretName: documentation-65gqj
  notAfter: "2025-06-08T18:28:20Z"
  notBefore: "2025-03-10T18:28:21Z"
  renewalTime: "2025-05-09T18:28:20Z"
  revision: 7
`,
				ready: true,
			},
			{
				name:   "without Ready/Available condition",
				status: fmt.Sprintf(statusWithConditionsFmt, ""),
				ready:  true,
			},
			{
				name: "with Ready false condition",
				status: generateStatusWithConditions(map[string]string{
					"Ready": "False",
				}),
				ready: false,
			},
			{
				name: "with Ready true condition",
				status: generateStatusWithConditions(map[string]string{
					"Ready": "True",
				}),
				ready: true,
			},
			{
				name: "with Available false condition",
				status: generateStatusWithConditions(map[string]string{
					"Available": "False",
				}),
				ready: false,
			},
			{
				name: "with Available true condition",
				status: generateStatusWithConditions(map[string]string{
					"Available": "True",
				}),
				ready: true,
			},
			{
				name: "with Ready and Available false condition",
				status: generateStatusWithConditions(map[string]string{
					"Ready":     "False",
					"Available": "False",
				}),
				ready: false,
			},
			{
				name: "with Ready true and Available false condition",
				status: generateStatusWithConditions(map[string]string{
					"Ready":     "True",
					"Available": "False",
				}),
				ready: true,
			},
			{
				name: "with Ready false and Available true condition",
				status: generateStatusWithConditions(map[string]string{
					"Ready":     "False",
					"Available": "True",
				}),
				ready: true,
			},
			{
				name: "with Ready true and Available true condition",
				status: generateStatusWithConditions(map[string]string{
					"Ready":     "True",
					"Available": "True",
				}),
				ready: true,
			},
		}

		for _, tst := range tests {
			t.Run(tst.name, func(t *testing.T) {
				checker := testResourceReadinessChecker(t, fmt.Sprintf(certFmt, tst.status))
				assertResourceReadyAfterAttempts(t, checker, 3, tst.ready)
			})
		}
	})

	const resourceForTestingKubeAPI = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static
spec:
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: Auto
  nodeType: Static
  staticInstances:
    count: 1
status:
  conditionSummary:
    ready: "True"
    statusMessage: ""
  conditions:
  - lastTransitionTime: "2024-08-17T19:30:21Z"
    status: "True"
    type: Ready
  - lastTransitionTime: "2024-08-19T14:49:18Z"
    status: "False"
    type: Updating
  - lastTransitionTime: "2024-08-17T19:30:21Z"
    status: "False"
    type: WaitingForDisruptiveApproval
  - lastTransitionTime: "2024-08-17T19:30:21Z"
    status: "False"
    type: Error
  deckhouse:
    observed:
      checkSum: 192a4757872c64c03feed7563070bea0
      lastTimestamp: "2024-12-09T21:30:00Z"
    processed:
      checkSum: 192a4757872c64c03feed7563070bea0
      lastTimestamp: "2024-12-09T20:00:46Z"
      synced: "True"
  error: ""
  kubernetesVersion: "1.32"
  nodes: 1
  ready: 1
  upToDate: 1
`
	const ngCoolDownAttempts = 5

	t.Run("Returns not ready if object did not create", func(t *testing.T) {
		checker := testResourceReadinessCheckerWithOptionalCreatingResource(t, resourceForTestingKubeAPI, false)
		assertResourceReadyAfterAttempts(t, checker, ngCoolDownAttempts, false)
	})

	t.Run("Returns not ready if api resource got error", func(t *testing.T) {
		checker := testResourceReadinessCheckerWithOptionalCreatingResource(t, resourceForTestingKubeAPI, false)
		checker.getAPIResources = func(_ *client.KubernetesClient, _, _ string) (*metav1.APIResource, error) {
			return nil, fmt.Errorf("not found")
		}
		assertResourceReadyAfterAttempts(t, checker, ngCoolDownAttempts, false)
	})

}

func assertResourceReadyAfterAttempts(t *testing.T, checker *resourceReadinessChecker, cooldownAttempts int, ready bool) {
	i := 1
	for ; i <= cooldownAttempts; i++ {
		assertCheckResourceReady(t, checker, false, i, false)
	}
	require.Equal(t, cooldownAttempts+1, i, "cooldownAttempts")
	assertCheckResourceReady(t, checker, ready, i, true)
}

func assertCheckResourceReady(t *testing.T, checker *resourceReadinessChecker, expectedReady bool, attempt int, cooldownPassed bool) {
	ctx := context.TODO()
	ready, err := checker.IsReady(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedReady, ready, "ready")
	require.Equal(t, attempt, checker.attempt, "attempt")
	require.Equal(t, cooldownPassed, checker.cooldownPassed, "cooldown")
}

func testResourceReadinessChecker(t *testing.T, resourceYAML string) *resourceReadinessChecker {
	return testResourceReadinessCheckerWithOptionalCreatingResource(t, resourceYAML, true)
}

func testResourceReadinessCheckerWithOptionalCreatingResource(t *testing.T, resourceYAML string, createResource bool) *resourceReadinessChecker {
	require.NotEmpty(t, resourceYAML)

	resources, err := template.ParseResourcesContent(resourceYAML, make(map[string]any))
	require.NoError(t, err)
	require.Len(t, resources, 1)

	resource := resources[0]
	require.NotNil(t, resource)

	gvr := schema.GroupVersionResource{
		Group:    resource.GVK.Group,
		Version:  resource.GVK.Version,
		Resource: pluralize(resource.GVK.Kind),
	}

	kubeCl := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		gvr: resource.GVK.Kind + "List",
	})

	ns := resource.Object.GetNamespace()

	if createResource {
		var created *unstructured.Unstructured
		var errCreate error
		ctx := context.TODO()
		if ns != "" {
			created, errCreate = kubeCl.Dynamic().Resource(gvr).Namespace(ns).Create(ctx, &resource.Object, metav1.CreateOptions{})
		} else {
			created, errCreate = kubeCl.Dynamic().Resource(gvr).Create(ctx, &resource.Object, metav1.CreateOptions{})
		}

		require.NoError(t, errCreate)
		require.NotNil(t, created)

		createdJson, err := created.MarshalJSON()
		require.NoError(t, err)
		originalJSON, err := resource.Object.MarshalJSON()
		require.NoError(t, err)

		require.JSONEq(t, string(originalJSON), string(createdJson))
	}

	kubeProvider := kubernetes.NewSimpleKubeClientGetter(kubeCl)
	logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())

	checker, err := newResourceIsReadyChecker(resource, constructorParams{
		loggerProvider: log.SimpleLoggerProvider(logger),
		// do not need
		metaConfig:   nil,
		kubeProvider: kubeProvider,
	})
	require.NoError(t, err)

	checker.getAPIResources = func(kubeCl *client.KubernetesClient, apiVersion, kind string) (*metav1.APIResource, error) {
		return &metav1.APIResource{
			Name:       gvr.Resource,
			Namespaced: ns != "",
			// another fields do not necessary
		}, nil
	}

	return checker
}

func pluralize(s string) string {
	s = strings.ToLower(s)
	return s + "s"
}
