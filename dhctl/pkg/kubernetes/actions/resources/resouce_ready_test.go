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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestResourceReadinessChecker_checkObjectReadiness(t *testing.T) {
	createResourceChecker := func() *resourceReadinessChecker {
		return &resourceReadinessChecker{
			logger: log.NewJSONLogger(log.LoggerOptions{IsDebug: true}),
		}
	}

	toUnstructured := func(resource string) (*unstructured.Unstructured, string) {
		res := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(resource), res)
		require.NoError(t, err)
		return res, fmt.Sprintf("%s/%s", res.GetNamespace(), res.GetName())
	}

	t.Run("Simple resource without status should return `ready`", func(t *testing.T) {
		resourceUnstruct, name := toUnstructured(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: dotfile-cm
  namespace: d8-system
data:
  .file: "content"
`)
		c := createResourceChecker()

		res := c.checkObjectReadiness(resourceUnstruct, name)

		require.Equal(t, true, res)
		require.Equal(t, 0, c.waitingConditionAttempts)
	})

	t.Run("Static instance should return `not ready` if status.currentStatus.phase == Pending", func(t *testing.T) {
		resourceUnstruct, name := toUnstructured(`
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
    phase: Pending
`)
		c := createResourceChecker()

		res := c.checkObjectReadiness(resourceUnstruct, name)

		require.Equal(t, false, res)
		require.Equal(t, 0, c.waitingConditionAttempts)
	})

	t.Run("Static instance should return `ready` if status.currentStatus.phase == Running", func(t *testing.T) {
		resourceUnstruct, name := toUnstructured(`
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
    phase: Running
`)
		c := createResourceChecker()

		res := c.checkObjectReadiness(resourceUnstruct, name)

		require.Equal(t, true, res)
		require.Equal(t, 0, c.waitingConditionAttempts)
	})

	t.Run("NodeGroup should return `not ready` if condition Ready is False", func(t *testing.T) {
		resourceUnstruct, name := toUnstructured(`
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  creationTimestamp: "2024-08-17T19:30:21Z"
  generation: 4
  name: static
  resourceVersion: "74791163"
  uid: e12c653e-b333-4cf6-a93c-632e6dbee00e
spec:
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: Auto
  nodeType: Static
  staticInstances:
    count: 0
status:
  conditionSummary:
    ready: "True"
    statusMessage: ""
  conditions:
  - lastTransitionTime: "2024-08-17T19:30:21Z"
    status: "False"
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
  kubernetesVersion: "1.29"
  nodes: 0
  ready: 0
  upToDate: 0
`)
		c := createResourceChecker()

		res := c.checkObjectReadiness(resourceUnstruct, name)

		require.Equal(t, false, res)
		require.Equal(t, 0, c.waitingConditionAttempts)
	})

	t.Run("Deployment should return `ready` if condition Available is True", func(t *testing.T) {
		resourceUnstruct, name := toUnstructured(`
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
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: deckhouse
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
  strategy:
    rollingUpdate:
      maxSurge: 0
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      annotations:
        checksum/config: dd73f062a6e4691a2e91914c31127ddb0548aa0a89ac9ef35520de0c8f47b918
        kubectl.kubernetes.io/restartedAt: "2023-06-26T11:14:40Z"
      creationTimestamp: null
      labels:
        app: webhook-handler
    spec:
      containers:
        image: alpine:latest
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 9680
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        name: handler
        ports:
        - containerPort: 9680
          name: validating-http
          protocol: TCP
        - containerPort: 9681
          name: conversion-http
          protocol: TCP
        resources:
          requests:
            ephemeral-storage: 60Mi
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      imagePullSecrets:
      - name: deckhouse-registry
      nodeSelector:
        node-role.kubernetes.io/control-plane: ""
      priorityClassName: system-cluster-critical
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext:
        runAsGroup: 64535
        runAsNonRoot: true
        runAsUser: 64535
      serviceAccount: webhook-handler
      serviceAccountName: webhook-handler
      terminationGracePeriodSeconds: 30
status:
  availableReplicas: 1
  conditions:
  - lastTransitionTime: "2024-04-09T16:06:46Z"
    lastUpdateTime: "2024-04-09T16:06:46Z"
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailable
    status: "True"
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
`)
		c := createResourceChecker()

		res := c.checkObjectReadiness(resourceUnstruct, name)

		require.Equal(t, true, res)
		require.Equal(t, 0, c.waitingConditionAttempts)
	})

	t.Run("Deployment should return `not ready` if condition Available is False", func(t *testing.T) {
		resourceUnstruct, name := toUnstructured(`
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
  creationTimestamp: "2024-05-03T12:21:12Z"
  generation: 1
  name: alerts-proxy
  namespace: default
  resourceVersion: "1113075555"
  uid: a141fd43-6b85-4566-b40d-98df7c3b0e01
spec:
  progressDeadlineSeconds: 600
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      run: alerts-proxy
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        run: alerts-proxy
    spec:
      containers:
      - args:
        - /controller
        - --addr=0.0.0.0:80
        image: alpine:latest
        imagePullPolicy: IfNotPresent
        name: my-nginx
        ports:
        - containerPort: 80
          protocol: TCP
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
status:
  conditions:
  - lastTransitionTime: "2024-05-03T12:21:12Z"
    lastUpdateTime: "2024-05-03T12:21:14Z"
    message: ReplicaSet "alerts-proxy-86df5dff58" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing
  - lastTransitionTime: "2024-06-10T11:13:40Z"
    lastUpdateTime: "2024-06-10T11:13:40Z"
    message: Deployment does not have minimum availability.
    reason: MinimumReplicasUnavailable
    status: "False"
    type: Available
  observedGeneration: 1
  replicas: 1
  unavailableReplicas: 1
  updatedReplicas: 1
`)
		c := createResourceChecker()

		res := c.checkObjectReadiness(resourceUnstruct, name)

		require.Equal(t, false, res)
		require.Equal(t, 0, c.waitingConditionAttempts)
	})

	// job is not supported directly, use in tests for case `ready with attempts`
	t.Run("Job should be ready with some attempts", func(t *testing.T) {
		resourceUnstruct, name := toUnstructured(`
apiVersion: batch/v1
kind: Job
metadata:
  annotations:
    batch.kubernetes.io/job-tracking: ""
  creationTimestamp: "2024-12-09T21:42:00Z"
  generation: 1
  labels:
    batch.kubernetes.io/controller-uid: bfc9952e-862e-4310-802b-f6beea96ad5f
    batch.kubernetes.io/job-name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3-28896342
    controller-uid: bfc9952e-862e-4310-802b-f6beea96ad5f
    job-name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3-28896342
  name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3-28896342
  namespace: kube-system
  ownerReferences:
  - apiVersion: batch/v1
    blockOwnerDeletion: true
    controller: true
    kind: CronJob
    name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3
    uid: d05500c0-818e-4a5d-9fc1-57a5283ec632
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
      creationTimestamp: null
      labels:
        batch.kubernetes.io/controller-uid: bfc9952e-862e-4310-802b-f6beea96ad5f
        batch.kubernetes.io/job-name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3-28896342
        controller-uid: bfc9952e-862e-4310-802b-f6beea96ad5f
        job-name: d8-etcd-backup-1037e1653436c137a42be8cf996416ce3-28896342
    spec:
      containers:
      - image: alpine:latest
        imagePullPolicy: IfNotPresent
        name: backup
        resources:
          requests:
            ephemeral-storage: 100Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirstWithHostNet
      hostNetwork: true
      imagePullSecrets:
      - name: deckhouse-registry
      nodeSelector:
        kubernetes.io/hostname: sandbox-master-0
      priorityClassName: cluster-low
      restartPolicy: Never
      schedulerName: default-scheduler
      securityContext:
        runAsGroup: 0
        runAsNonRoot: false
        runAsUser: 0
      terminationGracePeriodSeconds: 30
status:
  completionTime: "2024-12-09T21:42:30Z"
  conditions:
  - lastProbeTime: "2024-12-09T21:42:30Z"
    lastTransitionTime: "2024-12-09T21:42:30Z"
    status: "True"
    type: Complete
  ready: 0
  startTime: "2024-12-09T21:42:00Z"
  succeeded: 1
  uncountedTerminatedPods: {}
`)
		c := createResourceChecker()

		for i := 0; i < 5; i++ {
			res := c.checkObjectReadiness(resourceUnstruct, name)

			require.Equal(t, false, res)
			require.Equal(t, i+1, c.waitingConditionAttempts)
		}

		res := c.checkObjectReadiness(resourceUnstruct, name)

		require.Equal(t, true, res)
		require.Equal(t, 6, c.waitingConditionAttempts)
	})
}
