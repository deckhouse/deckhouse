// Copyright 2025 Flant JSC
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

package readiness

import (
	"context"
	"fmt"
	"testing"

	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestStaticInstanceChecker(t *testing.T) {
	checkerType := &StaticInstanceChecker{}

	createTestWithPhase := func(phase string, isReady bool) testChecker {
		return testChecker{
			testName: fmt.Sprintf("current status has phase %s", phase),
			content: fmt.Sprintf(`
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 192.168.199.33
  credentialsRef:
    kind: SSHCredentials
    name: credentials
status:
  conditions:
  - lastTransitionTime: "2024-08-16T15:50:18Z"
    status: "True"
    type: AddedToNodeGroup
  currentStatus:
    lastUpdateTime: "2024-08-16T15:50:18Z"
    phase: %s
`, phase),
			ready:       isReady,
			hasError:    false,
			checkerType: checkerType,
		}
	}

	tests := []testChecker{
		{
			testName: "without status",
			content: `
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 192.168.199.33
  credentialsRef:
    kind: SSHCredentials
    name: credentials
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "empty status",
			content: `
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 192.168.199.33
  credentialsRef:
    kind: SSHCredentials
    name: credentials
status: {}
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "status without current status",
			content: `
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 192.168.199.33
  credentialsRef:
    kind: SSHCredentials
    name: credentials
status:
  conditions:
  - lastTransitionTime: "2024-08-16T15:50:18Z"
    status: "True"
    type: AddedToNodeGroup
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "current status is empty",
			content: `
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 192.168.199.33
  credentialsRef:
    kind: SSHCredentials
    name: credentials
status:
  conditions:
  - lastTransitionTime: "2024-08-16T15:50:18Z"
    status: "True"
    type: AddedToNodeGroup
  currentStatus: {}
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		createTestWithPhase("Pending", false),
		createTestWithPhase("Bootstrapping", false),
		createTestWithPhase("Cleaning", false),
		createTestWithPhase("Error", false),
		createTestWithPhase("Running", true),
		{
			testName: "incorrect status type",
			content: `
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 192.168.199.33
  credentialsRef:
    kind: SSHCredentials
    name: credentials
status: []
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "incorrect current status type",
			content: `
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 192.168.199.33
  credentialsRef:
    kind: SSHCredentials
    name: credentials
status:
  conditions:
  - lastTransitionTime: "2024-08-16T15:50:18Z"
    status: "True"
    type: AddedToNodeGroup
  currentStatus: "string"
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "incorrect type phase",
			content: `
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 192.168.199.33
  credentialsRef:
    kind: SSHCredentials
    name: credentials
status:
  conditions:
  - lastTransitionTime: "2024-08-16T15:50:18Z"
    status: "True"
    type: AddedToNodeGroup
  currentStatus:
    lastUpdateTime: "2024-08-16T15:50:18Z"
    phase: 123
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
	}

	assertAllCheckersTest(t, tests)
}

func TestNoAdditionalCheckChecker(t *testing.T) {
	checkerType := &ExistsResourceWithoutChecker{}

	createTestForResource := func(testName, resourceYAML string) testChecker {
		return testChecker{
			testName:    testName,
			content:     resourceYAML,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		}
	}

	tests := []testChecker{
		createTestForResource("secret", `
apiVersion: v1
kind: Secret
metadata:
  name: secret
data:
  secret-file: dmFsdWUtMg0KDQo=
`),
		createTestForResource("configmap", `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-system
data:
  file: "content"
`),
		createTestForResource("namespace", `
apiVersion: v1
kind: Namespace
metadata:
  name: test-ns-with-multiple-resources
`),
		createTestForResource("role", `
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: d8:node-manager:bashible-events
  namespace: default
rules:
- apiGroups:
  - events.k8s.io
  resources:
  - events
  verbs:
  - get
  - list
  - create
  - update
  - patch
`),
		createTestForResource("clusterrolebinding", `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: user-authz:test:scale
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: user-authz:scale
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: test@flant.com
`),
		createTestForResource("job", `
apiVersion: batch/v1
kind: Job
metadata:
  name: test
  namespace: default
spec:
  backoffLimit: 0
  completionMode: NonIndexed
  completions: 1
  manualSelector: false
  parallelism: 1
  podReplacementPolicy: TerminatingOrFailed
  selector:
    matchLabels:
      controller-uid: c70e2a0d-99e0-4e81-b105-57fe53241865
  suspend: false
  template:
    metadata:
      labels:
        controller-uid: c70e2a0d-99e0-4e81-b105-57fe53241865
    spec:
      containers:
      - command:
        - /bin/ash
        image: alpine:3.13
        name: installer
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      schedulerName: default-scheduler
status:
  completionTime: "2021-05-18T16:37:48Z"
  conditions:
  - lastProbeTime: "2021-05-18T16:37:48Z"
    lastTransitionTime: "2021-05-18T16:37:48Z"
    status: "True"
    type: Complete
  startTime: "2021-05-18T16:37:37Z"
  succeeded: 1
`),
	}

	assertAllCheckersTest(t, tests)
}

func TestByPhaseChecker(t *testing.T) {
	checkerType := &ByPhaseChecker{}

	createTestForPodPhase := func(phase string, isReady bool) testChecker {
		return testChecker{
			testName: fmt.Sprintf("pod with phase %s", phase),
			content: fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: test
  namespace: default
spec:
  containers:
  - command:
    - python3
    image: image:latest
    imagePullPolicy: IfNotPresent
    name: app
  dnsPolicy: ClusterFirst
  priority: 1000
  priorityClassName: develop
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: default
  serviceAccountName: default
  terminationGracePeriodSeconds: 30
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-08-20T11:23:50Z"
    reason: PodCompleted
    status: "False"
  phase: %s
`, phase),
			ready:       isReady,
			hasError:    false,
			checkerType: checkerType,
		}
	}

	createTestForPVPhase := func(phase string, isReady bool) testChecker {
		return testChecker{
			testName: fmt.Sprintf("pv with phase %s", phase),
			content: fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolume
metadata:
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: www-web-removed-0
    namespace: default
    resourceVersion: "866989489"
    uid: 282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
  hostPath:
    path: /opt/local-path-provisioner/removed/pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1_default_www-web-removed-0
    type: DirectoryOrCreate
  persistentVolumeReclaimPolicy: Delete
  storageClassName: localpath-dev-removed
  volumeMode: Filesystem
status:
  lastPhaseTransitionTime: "2025-05-04T16:07:47Z"
  phase: %s
`, phase),
			ready:       isReady,
			hasError:    false,
			checkerType: checkerType,
		}
	}

	createTestForPVCPhase := func(phase string, isReady bool) testChecker {
		return testChecker{
			testName: fmt.Sprintf("pvc with phase %s", phase),
			content: fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  finalizers:
  - kubernetes.io/pvc-protection
  name: test
  namespace: default
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
`, phase),
			ready:       isReady,
			hasError:    false,
			checkerType: checkerType,
		}
	}

	tests := []testChecker{
		{
			testName: "pod without status",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: test
  namespace: default
spec:
  containers:
  - command:
    - python3
    image: image:latest
    imagePullPolicy: IfNotPresent
    name: app
  dnsPolicy: ClusterFirst
  priority: 1000
  priorityClassName: develop
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: default
  serviceAccountName: default
  terminationGracePeriodSeconds: 30
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "pod without phase",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: test
  namespace: default
spec:
  containers:
  - command:
    - python3
    image: image:latest
    imagePullPolicy: IfNotPresent
    name: app
  dnsPolicy: ClusterFirst
  priority: 1000
  priorityClassName: develop
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: default
  serviceAccountName: default
  terminationGracePeriodSeconds: 30
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-08-20T11:23:50Z"
    reason: PodCompleted
    status: "True"
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		createTestForPodPhase("Pending", false),
		createTestForPodPhase("Failed", false),
		createTestForPodPhase("Incorrect", false),
		createTestForPodPhase("Succeeded", true),
		createTestForPodPhase("Running", true),
		{
			testName: "pod with incorrect status",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: test
  namespace: default
spec:
  containers:
  - command:
    - python3
    image: image:latest
    imagePullPolicy: IfNotPresent
    name: app
  dnsPolicy: ClusterFirst
  priority: 1000
  priorityClassName: develop
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: default
  serviceAccountName: default
  terminationGracePeriodSeconds: 30
status: "string"
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "pod with incorrect phase",
			content: `
apiVersion: v1
kind: Pod
metadata:
  name: test
  namespace: default
spec:
  containers:
  - command:
    - python3
    image: image:latest
    imagePullPolicy: IfNotPresent
    name: app
  dnsPolicy: ClusterFirst
  priority: 1000
  priorityClassName: develop
  restartPolicy: Always
  schedulerName: default-scheduler
  securityContext: {}
  serviceAccount: default
  serviceAccountName: default
  terminationGracePeriodSeconds: 30
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-08-20T11:23:50Z"
    reason: PodCompleted
    status: "False"
  phase: {}
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "pv without status",
			content: `
apiVersion: v1
kind: PersistentVolume
metadata:
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: www-web-removed-0
    namespace: default
    resourceVersion: "866989489"
    uid: 282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
  hostPath:
    path: /opt/local-path-provisioner/removed/pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1_default_www-web-removed-0
    type: DirectoryOrCreate
  persistentVolumeReclaimPolicy: Delete
  storageClassName: localpath-dev-removed
  volumeMode: Filesystem
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "pv with empty status",
			content: `
apiVersion: v1
kind: PersistentVolume
metadata:
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: www-web-removed-0
    namespace: default
    resourceVersion: "866989489"
    uid: 282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
  hostPath:
    path: /opt/local-path-provisioner/removed/pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1_default_www-web-removed-0
    type: DirectoryOrCreate
  persistentVolumeReclaimPolicy: Delete
  storageClassName: localpath-dev-removed
  volumeMode: Filesystem
status: {}
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "pv without phase",
			content: `
apiVersion: v1
kind: PersistentVolume
metadata:
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: www-web-removed-0
    namespace: default
    resourceVersion: "866989489"
    uid: 282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
  hostPath:
    path: /opt/local-path-provisioner/removed/pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1_default_www-web-removed-0
    type: DirectoryOrCreate
  persistentVolumeReclaimPolicy: Delete
  storageClassName: localpath-dev-removed
  volumeMode: Filesystem
status:
  lastPhaseTransitionTime: "2025-05-04T16:07:47Z"
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		createTestForPVPhase("Released", false),
		createTestForPVPhase("Failed", false),
		createTestForPVPhase("Incorrect", false),
		createTestForPVPhase("Available", true),
		createTestForPVPhase("Bound", true),
		{
			testName: "pv with incorrect status",
			content: `
apiVersion: v1
kind: PersistentVolume
metadata:
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: www-web-removed-0
    namespace: default
    resourceVersion: "866989489"
    uid: 282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
  hostPath:
    path: /opt/local-path-provisioner/removed/pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1_default_www-web-removed-0
    type: DirectoryOrCreate
  persistentVolumeReclaimPolicy: Delete
  storageClassName: localpath-dev-removed
  volumeMode: Filesystem
status: 123
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "pv with incorrect phase",
			content: `
apiVersion: v1
kind: PersistentVolume
metadata:
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: www-web-removed-0
    namespace: default
    resourceVersion: "866989489"
    uid: 282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1
  hostPath:
    path: /opt/local-path-provisioner/removed/pvc-282bcfa7-2d5a-4dcc-92dd-7bbe8c74d9c1_default_www-web-removed-0
    type: DirectoryOrCreate
  persistentVolumeReclaimPolicy: Delete
  storageClassName: localpath-dev-removed
  volumeMode: Filesystem
status:
  lastPhaseTransitionTime: "2025-05-04T16:07:47Z"
  phase: []
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "pvc without status",
			content: `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  finalizers:
  - kubernetes.io/pvc-protection
  name: test
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ceph-ssd
  volumeMode: Filesystem
  volumeName: pvc-88dd79ea-55c9-42ab-a528-2741d5c550cd
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "pvc with empty status",
			content: `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  finalizers:
  - kubernetes.io/pvc-protection
  name: test
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ceph-ssd
  volumeMode: Filesystem
  volumeName: pvc-88dd79ea-55c9-42ab-a528-2741d5c550cd
status: {}
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "pvc without status",
			content: `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  finalizers:
  - kubernetes.io/pvc-protection
  name: test
  namespace: default
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
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		createTestForPVCPhase("Pending", false),
		createTestForPVCPhase("Lost", false),
		createTestForPVCPhase("Incorrect", false),
		createTestForPVCPhase("Bound", true),
		{
			testName: "pvc with incorrect status",
			content: `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  finalizers:
  - kubernetes.io/pvc-protection
  name: test
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ceph-ssd
  volumeMode: Filesystem
  volumeName: pvc-88dd79ea-55c9-42ab-a528-2741d5c550cd
status: []
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},

		{
			testName: "pvc with incorrect phase",
			content: `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  finalizers:
  - kubernetes.io/pvc-protection
  name: test
  namespace: default
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
  phase: {}
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
	}

	assertAllCheckersTest(t, tests)
}

func TestByConditionsKnownResourcesChecker(t *testing.T) {
	checkerType := &ByConditionsChecker{}

	tests := []testChecker{
		{
			testName: "deployment without status",
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "deployment with empty status",
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
status: {}
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "deployment without conditions",
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
status:
  replicas: 1
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			//testName: "deployment without available condition",
			testName: "a",

			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
status:
  conditions:
  - lastTransitionTime: "2025-10-10T13:38:45Z"
    lastUpdateTime: "2025-10-17T10:07:17Z"
    message: ReplicaSet "test-786cdf6646" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "deployment with false status of available condition",
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
status:
  conditions:
  - lastTransitionTime: "2025-10-10T13:38:45Z"
    lastUpdateTime: "2025-10-17T10:07:17Z"
    message: ReplicaSet "test-786cdf6646" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing
  - lastTransitionTime: "2025-08-03T15:26:40Z"
    lastUpdateTime: "2025-08-03T15:26:40Z"
    message: Deployment does not have minimum availability.
    reason: MinimumReplicasUnavailable
    status: "False"
    type: Available
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "deployment with true status of available condition",
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
status:
  conditions:
  - lastTransitionTime: "2025-10-10T13:38:45Z"
    lastUpdateTime: "2025-10-17T10:07:17Z"
    message: ReplicaSet "test-786cdf6646" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing
  - lastTransitionTime: "2025-08-03T15:26:40Z"
    lastUpdateTime: "2025-08-03T15:26:40Z"
    message: Deployment does not have minimum availability.
    reason: MinimumReplicasUnavailable
    status: "True"
    type: Available
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "deployment incorrect status",
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
status: 134
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "deployment with incorrect conditions",
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
status:
  conditions: {}
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "deployment with incorrect type condition",
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
status:
  conditions:
  - lastTransitionTime: "2025-08-03T15:26:40Z"
    lastUpdateTime: "2025-08-03T15:26:40Z"
    message: Deployment does not have minimum availability.
    reason: MinimumReplicasUnavailable
    status: "True"
    type: []
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "deployment incorrect status of available condition",
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-as
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test-as
    spec:
      containers:
      - command:
        - python3
        image: image:latest
        imagePullPolicy: IfNotPresent
        name: app
      restartPolicy: Always
      schedulerName: default-scheduler
status:
  conditions:
  - lastTransitionTime: "2025-08-03T15:26:40Z"
    lastUpdateTime: "2025-08-03T15:26:40Z"
    message: Deployment does not have minimum availability.
    reason: MinimumReplicasUnavailable
    status: []
    type: Available
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		// api service
		{
			testName: "APIService without status",
			content: `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.deckhouse.io
spec:
  group: deckhouse.io
  groupPriorityMinimum: 1000
  version: v1
  versionPriority: 100
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "APIService with empty status",
			content: `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.deckhouse.io
spec:
  group: deckhouse.io
  groupPriorityMinimum: 1000
  version: v1
  versionPriority: 100
status: {}
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "APIService with false of available condition",
			content: `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.deckhouse.io
spec:
  group: deckhouse.io
  groupPriorityMinimum: 1000
  version: v1
  versionPriority: 100
status:
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: Local APIServices are always available
    reason: Local
    status: "False"
    type: Available
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "APIService with true of available condition",
			content: `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.deckhouse.io
spec:
  group: deckhouse.io
  groupPriorityMinimum: 1000
  version: v1
  versionPriority: 100
status:
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: Local APIServices are always available
    reason: Local
    status: "True"
    type: Available
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "APIService with incorrect status",
			content: `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.deckhouse.io
spec:
  group: deckhouse.io
  groupPriorityMinimum: 1000
  version: v1
  versionPriority: 100
status: []
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "APIService with incorrect conditions",
			content: `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.deckhouse.io
spec:
  group: deckhouse.io
  groupPriorityMinimum: 1000
  version: v1
  versionPriority: 100
status:
  conditions: "string"
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "APIService with incorrect type of condition",
			content: `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.deckhouse.io
spec:
  group: deckhouse.io
  groupPriorityMinimum: 1000
  version: v1
  versionPriority: 100
status:
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: Local APIServices are always available
    reason: Local
    status: "True"
    type: {}
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "APIService with incorrect status of available condition",
			content: `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.deckhouse.io
spec:
  group: deckhouse.io
  groupPriorityMinimum: 1000
  version: v1
  versionPriority: 100
status:
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: Local APIServices are always available
    reason: Local
    status: {}
    type: Available
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},

		// node group
		{
			testName: "Node group without status",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static
spec:
  nodeType: Static
  staticInstances:
    count: 0
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "Node group with empty status",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static
spec:
  nodeType: Static
  staticInstances:
    count: 0
status: {}
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},

		{
			testName: "Node group without ready condition",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static
spec:
  nodeType: Static
  staticInstances:
    count: 1
status:
  conditions:
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Updating
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Error
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},

		{
			testName: "Node group with false status of ready condition",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static
spec:
  nodeType: Static
  staticInstances:
    count: 1
status:
  conditions:
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Ready
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Updating
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "Node group with true status of ready condition",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ephemeral
spec:
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker-big
    maxPerZone: 0
    maxSurgePerZone: 0
    maxUnavailablePerZone: 0
    minPerZone: 0
  nodeType: CloudEphemeral
status:
  conditions:
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "True"
    type: Ready
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Updating
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
	}

	assertAllCheckersTest(t, tests)
}

func TestByConditionsNotKnownResourcesChecker(t *testing.T) {
	checkerType := &ByConditionsChecker{}

	tests := []testChecker{
		{
			testName: "without status",
			content: `
apiVersion: scheduling.k8s.io/v1
description: |
  Cluster components that are non-essential to the cluster'.
kind: PriorityClass
metadata:
  name: cluster-low
preemptionPolicy: PreemptLowerPriority
value: 2000
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with empty status",
			content: `
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: test
spec:
  email: test@flant.com
  groups:
  - test
  password: "2a"
status: {}
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with status without conditions",
			content: `
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: test
spec:
  email: test@flant.com
  groups:
  - test
  password: "2a"
status:
  groups:
  - test
  lock:
    state: false
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions without Available and Ready condition",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: tes
spec:
  isSudoer: false
  nodeGroups:
  - '*'
  passwordHash: "6"
  sshPublicKey: ssh-rsa AAA
  uid: 1001
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Updating
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Error
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions with false status of Available condition",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: tes
spec:
  isSudoer: false
  nodeGroups:
  - '*'
  passwordHash: "6"
  sshPublicKey: ssh-rsa AAA
  uid: 1001
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "False"
    type: Available
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions with true status of Available condition",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: tes
spec:
  isSudoer: false
  nodeGroups:
  - '*'
  passwordHash: "6"
  sshPublicKey: ssh-rsa AAA
  uid: 1001
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Updating
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "True"
    type: Available
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions with false status of Ready condition",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: tes
spec:
  isSudoer: false
  nodeGroups:
  - '*'
  passwordHash: "6"
  sshPublicKey: ssh-rsa AAA
  uid: 1001
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "False"
    type: Ready
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions with true status of Ready condition",
			content: `
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: tes
spec:
  isSudoer: false
  nodeGroups:
  - '*'
  passwordHash: "6"
  sshPublicKey: ssh-rsa AAA
  uid: 1001
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Updating
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "True"
    type: Ready
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions Ready and Available with false status of all",
			content: `
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  accessLevel: PrivilegedUser
  allowScale: true
  portForwarding: true
  subjects:
  - kind: User
    name: test@flant.com
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "False"
    type: Available
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Creating
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "False"
    type: Ready
`,
			ready:       false,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions false Available and true Ready",
			content: `
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  accessLevel: PrivilegedUser
  allowScale: true
  portForwarding: true
  subjects:
  - kind: User
    name: test@flant.com
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "False"
    type: Available
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Creating
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "True"
    type: Ready
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions true Available and false Ready",
			content: `
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  accessLevel: PrivilegedUser
  allowScale: true
  portForwarding: true
  subjects:
  - kind: User
    name: test@flant.com
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "True"
    type: Available
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Creating
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "False"
    type: Ready
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions true Available and true Ready",
			content: `
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  accessLevel: PrivilegedUser
  allowScale: true
  portForwarding: true
  subjects:
  - kind: User
    name: test@flant.com
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "True"
    type: Available
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Creating
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "True"
    type: Ready
`,
			ready:       true,
			hasError:    false,
			checkerType: checkerType,
		},
		{
			testName: "with conditions but type incorrect and true Ready",
			content: `
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  accessLevel: PrivilegedUser
  allowScale: true
  portForwarding: true
  subjects:
  - kind: User
    name: test@flant.com
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "True"
    type: {}
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Creating
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "True"
    type: Ready
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "with conditions but incorrect status of Available and true Ready",
			content: `
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  accessLevel: PrivilegedUser
  allowScale: true
  portForwarding: true
  subjects:
  - kind: User
    name: test@flant.com
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: []
    type: Available
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Creating
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: "True"
    type: Ready
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
		{
			testName: "with conditions but incorrect status of Available and Ready",
			content: `
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  accessLevel: PrivilegedUser
  allowScale: true
  portForwarding: true
  subjects:
  - kind: User
    name: test@flant.com
status:
  errors: {}
  conditions:
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: []
    type: Available
  - lastTransitionTime: "2024-08-23T17:04:48Z"
    status: "False"
    type: Creating
  - lastTransitionTime: "2023-07-25T11:24:56Z"
    message: "Some message"
    reason: NotReason
    status: {}
    type: Ready
`,
			ready:       false,
			hasError:    true,
			checkerType: checkerType,
		},
	}

	assertAllCheckersTest(t, tests)
}

type testChecker struct {
	content  string
	testName string
	ready    bool
	hasError bool

	checkerType any
}

func assertAllCheckersTest(t *testing.T, tests []testChecker) {
	for _, tst := range tests {
		t.Run(tst.testName, func(tt *testing.T) {
			assertChecker(t, tst)
		})
	}
}

func assertChecker(t *testing.T, tst testChecker) {
	require.NotEmpty(t, tst.testName)
	require.NotEmpty(t, tst.content, tst.testName)
	require.False(t, govalue.IsNil(tst.checkerType), tst.testName)

	obj, resourceName := testYAMLToUnstructured(t, tst.content)
	gvk := obj.GroupVersionKind()
	require.False(t, gvk.Empty(), tst.testName)

	logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())
	params := GetCheckerParams{
		LoggerProvider: log.SimpleLoggerProvider(logger),
	}

	checker, err := GetCheckerByGvk(&gvk, params)
	require.NoError(t, err, tst.testName)
	require.False(t, govalue.IsNil(checker), tst.testName)
	require.IsType(t, tst.checkerType, checker, tst.testName)

	ready, err := checker.IsReady(context.TODO(), obj, resourceName)
	if tst.hasError {
		require.Error(t, err, tst.testName)
	} else {
		require.NoError(t, err, tst.testName)
	}

	require.Equal(t, tst.ready, ready, tst.testName)
}

func testYAMLToUnstructured(t *testing.T, content string) (*unstructured.Unstructured, string) {
	t.Helper()

	jsonData, err := yaml.YAMLToJSON([]byte(content))
	require.NoError(t, err, content)

	obj := &unstructured.Unstructured{}
	err = obj.UnmarshalJSON(jsonData)
	require.NoError(t, err, content)

	name := fmt.Sprintf("%s/%s %s/%s", obj.GetAPIVersion(), obj.GetKind(), obj.GetNamespace(), obj.GetName())

	return obj, name
}
