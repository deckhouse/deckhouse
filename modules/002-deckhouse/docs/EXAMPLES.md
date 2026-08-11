---
title: "The deckhouse module: examples"
description: "Practical examples of creating and using PriorityClass, the pod preemption mechanism, and related troubleshooting."
---

## PriorityClass creation and usage examples

PriorityClass maps a priority class name to an integer value. The higher the value, the higher the pod priority. The scheduler uses this mechanism when placing pods and preempting them under node resource pressure.

When a node is short on resources, the scheduler preempts pods with a lower priority to free capacity for pods with a higher priority.

Deckhouse Kubernetes Platform (DKP) ships with a predefined set of priority classes. For the full list, recommended use cases, and usage restrictions, see [Priority Classes](./#priority-classes).

{% alert level="info" %}
Do not create custom classes with values above 1 000 000. Doing so can disrupt critically important system components.
{% endalert %}

### Creating a PriorityClass

Create the `my-priority.yaml` file with the following contents — a PriorityClass named `my-app-critical` with priority `8000`:

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: my-app-critical
value: 8000
globalDefault: false
description: "Priority for critical applications"
```

Apply the manifest in the cluster:

```shell
d8 k apply -f my-priority.yaml
```

Verify that the resource was created:

```shell
d8 k get priorityclass my-app-critical
```

Example output:

```console
NAME              VALUE   GLOBAL-DEFAULT   AGE   PREEMPTIONPOLICY
my-app-critical   8000    false            7s    PreemptLowerPriority
```

A PriorityClass manifest uses the following fields:

- `metadata.name` — the class name. Specified on pods in the `priorityClassName` parameter;
- `value` — the numeric priority. The higher the value, the higher the priority;
- `globalDefault` — whether this class is used by default for pods without an explicit `priorityClassName`;
- `description` — the class description.

{% alert level="warning" %}
Be careful with `globalDefault: true`. If you set it on a custom class, all pods in the cluster without an explicit priority receive that value, which can lead to unpredictable preemption of system components.
{% endalert %}

### Using PriorityClass in a pod

Create the `test-pod.yaml` file to run a pod with the `my-app-critical` class from [Creating a PriorityClass](#creating-a-priorityclass):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-priority-pod
spec:
  priorityClassName: my-app-critical
  containers:
  - name: nginx
    image: nginx
```

Apply the manifest in the cluster:

```shell
d8 k apply -f test-pod.yaml
```

Verify that the pod received the expected priority:

```shell
d8 k describe pod test-priority-pod | grep Priority
```

Example output:

```console
Priority:             8000
Priority Class Name:  my-app-critical
```

{% alert level="warning" %}
You cannot change `priorityClassName` on a running pod. The field is immutable. The only way to change the priority is to delete the pod and create it again with a new class.
{% endalert %}

### Using PriorityClass in a Deployment

You can set PriorityClass in a Deployment template. In that case, all pods created by the Deployment inherit the specified priority class.

Create the `deployment-with-priority.yaml` file to deploy an application with the DKP built-in `production-high` class (value `9000`, see [Priority Classes](./#priority-classes)):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      priorityClassName: production-high
      containers:
      - name: app
        image: nginx:latest
        ports:
        - containerPort: 80
```

Apply the manifest in the cluster:

```shell
d8 k apply -f deployment-with-priority.yaml
```

Check the priorities of the created pods:

```shell
d8 k get pods -l app=my-app -o custom-columns=NAME:.metadata.name,CLASS:.spec.priorityClassName,PRIORITY:.spec.priority
```

Example output:

```console
NAME                      CLASS             PRIORITY
my-app-7d8f9b5c4f-2xq9t   production-high   9000
my-app-7d8f9b5c4f-4p7k2   production-high   9000
my-app-7d8f9b5c4f-9w5r8   production-high   9000
```

## Preemption mechanism

### How it works

The scheduler bases preemption decisions solely on the numeric priority value. The workload type (Deployment, StatefulSet, DaemonSet, or a plain pod) does not matter — the scheduler compares only numeric values: a pod with a higher priority value can preempt a pod with a lower value to free resources on a node.

Equal priorities do not trigger preemption. If the new pod has the same priority as existing pods on the node, the scheduler does not preempt them — the new pod stays `Pending` until free resources appear.

### Step-by-step preemption demo

In the examples below, replace `worker-0` with the name of your worker node.

{% alert level="info" %}
The behavior depends on the number of worker nodes and may differ from the example below: the demo assumes a cluster with a single worker node. If there are multiple worker nodes, pods may be spread across them, and preemption on one node may not always occur.
{% endalert %}

List the nodes in the cluster:

```shell
d8 k get nodes
```

Example output:

```console
NAME        STATUS   ROLES                  AGE   VERSION
master-0    Ready    control-plane,master   14d   v1.34.9
worker-0    Ready    worker                 14d   v1.34.9
```

Check resources on the worker node:

```shell
d8 k describe node worker-0 | grep -E "Capacity|Allocatable|Allocated" -A 5
```

Example output:

```console
Capacity:
  cpu:                4
  memory:             8174932Ki
Allocatable:
  cpu:                3800m
  memory:             7174932Ki
Allocated resources:
  cpu:                1200m (31%)
  memory:             2Gi (28%)
```

In this example, the node has roughly 2600m CPU and 5Gi of memory free. Create a low-priority pod that takes a large share of those resources, then a high-priority pod that does not fit in what remains.

{% alert level="info" %}
The `requests` values in this example (2 CPU and 4Gi of memory) are tuned for a node with 4 CPU and 8Gi of memory where about 30% of resources are already allocated. If your nodes are larger or smaller, adjust the values so the low-priority pod uses roughly 70–80% of the node's free resources.
{% endalert %}

Create the file `low-priority-pod.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: low-priority-pod
spec:
  priorityClassName: develop
  containers:
  - name: app
    image: nginx
    resources:
      requests:
        cpu: "2"
        memory: "4Gi"
```

Apply the manifest:

```shell
d8 k apply -f low-priority-pod.yaml
```

Wait for the pod to reach the `Running` state:

```shell
d8 k get pods low-priority-pod
```

Example output:

```console
NAME               READY   STATUS    RESTARTS   AGE
low-priority-pod   1/1     Running   0          10s
```

Create the `high-priority-pod.yaml` file to run a high-priority pod that requests more resources than remain free on the node:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: high-priority-pod
spec:
  priorityClassName: production-high
  containers:
  - name: app
    image: nginx
    resources:
      requests:
        cpu: "2"
        memory: "4Gi"
```

Apply the manifest:

```shell
d8 k apply -f high-priority-pod.yaml
```

Check pod status:

```shell
d8 k get pods | grep priority
```

Example output:

```console
NAME                READY   STATUS    RESTARTS   AGE
high-priority-pod   1/1     Running   0          5s
```

The `high-priority-pod` pod is `Running`. The `low-priority-pod` pod may be missing from the output because the scheduler preempted it within seconds. In some cases you may still see `low-priority-pod` in `Pending`.

Check preemption events:

```shell
d8 k get events --field-selector reason=Preempted
```

Example output:

```console
LAST SEEN   TYPE     REASON      OBJECT                 MESSAGE
68s         Normal   Preempted   pod/low-priority-pod   Preempted by pod d9d25b95-4a7d-4214-8a30-8ce1fd616f67 on node worker-0
```

## Protection against preemption

The only way to protect a pod from preemption is to assign it a sufficiently high priority. Even then, a pod with an even higher priority can still preempt it.

To reduce preemption risk, combine three mechanisms. They are generic and apply to any critical application:

1. A high `priorityClassName` — makes the pod a less likely candidate for preemption.
1. PodDisruptionBudget (PDB) — limits how many replicas can be removed at once. During preemption, PDB acts as a recommendation (unlike planned operations such as `d8 k drain`, where PDB is a hard constraint).
1. `terminationGracePeriodSeconds` — gives the application time to flush data to disk and close connections before forced deletion. This is the last line of defense that helps keep data consistent even if the PDB is violated.

{% alert level="info" %}
Without `terminationGracePeriodSeconds`, an immediate stop can cause loss of unsaved data, database corruption, and a lengthy filesystem check on the next start. A graceful shutdown lets new pods remount the PVC and recover from a consistent state.
{% endalert %}

For a practical application of these mechanisms to Stateful workloads, see [Protecting Stateful applications](#protecting-stateful-applications).

## Architectural usage scenarios

### Separating environments by priority

Separating environments by namespace alone does not protect critical services from cluster-wide resource pressure. Priority classes let you build a clear hierarchy of resource consumption.

Suppose a cluster node is fully loaded by a background job from a development environment. At the same time, a critical service in the production environment needs to scale. Because of the priority gap (`9000` for `production-high` versus `1000` for `develop`), the scheduler automatically preempts the development job to free resources for the critical production service.

{% alert level="info" %}
Without priority changes, the reverse situation — preemption of a production service by development jobs — is impossible.
{% endalert %}

In the development environment, create the `dev-data-processor.yaml` file to run a background job with the built-in `develop` class:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: dev-data-processor
spec:
  schedule: "0 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          priorityClassName: develop
          containers:
          - name: processor
            image: busybox:latest
            command: ["sleep", "infinity"]
            resources:
              requests:
                cpu: "2"
                memory: "4Gi"
              limits:
                cpu: "2"
                memory: "4Gi"
          restartPolicy: OnFailure
```

Apply the manifest:

```shell
d8 k apply -f dev-data-processor.yaml
```

{% alert level="info" %}
The `command: ["sleep", "infinity"]` line is a placeholder. It keeps the container running indefinitely and holds the requested resources, which is required for a reliable demonstration of node resource pressure without deploying a real application.
{% endalert %}

Create a Job manually for an immediate run:

```shell
d8 k create job --from=cronjob/dev-data-processor manual-test
```

Verify that the pod started:

```shell
d8 k get pods | grep manual-test
```

Example output:

```console
manual-test-mh9f4                   1/1     Running   0          15h
```

In the production environment, create the `prod-api.yaml` file and start a critical high-priority service using the built-in `production-high` class:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prod-api
spec:
  replicas: 2
  selector:
    matchLabels:
      app: prod-api
  template:
    metadata:
      labels:
        app: prod-api
    spec:
      priorityClassName: production-high
      containers:
      - name: api
        image: busybox:latest
        command: ["sleep", "infinity"]
        resources:
          requests:
            cpu: "2"
            memory: "4Gi"
          limits:
            cpu: "2"
            memory: "4Gi"
```

Apply the manifest:

```shell
d8 k apply -f prod-api.yaml
```

Check events to confirm that preemption occurred:

```shell
d8 k get events --field-selector reason=Preempted --sort-by='.lastTimestamp'
```

Example output:

```console
LAST SEEN   TYPE     REASON      OBJECT                  MESSAGE
15s         Normal   Preempted   pod/manual-test-tnksn   Preempted by pod 3205b347-705d-49b0-9a20-a1e56f51cb7e on node worker-0
```

## Protecting Stateful applications

Stateful applications (for example databases and message queues) keep data in memory or in persistent volumes (PVC). Protecting them requires a special approach: abruptly destroying a pod without a graceful shutdown can corrupt data, and mass preemption of replicas can take the service down.

This example protects Stateful applications with the three mechanisms from [Protection against preemption](#protection-against-preemption):

- A high `priorityClassName` (prefer `production-high` with value `9000` or higher; see [Priority Classes](./#priority-classes)) makes pods less likely candidates for preemption.
- PodDisruptionBudget keeps a minimum number of healthy replicas (for example, `minAvailable: 5`).
- `terminationGracePeriodSeconds` gives time to flush data to disk and close transactions before the pod exits (30–60 seconds is a common recommendation).

### Demonstrating Stateful application protection

#### Step 1. Create a protected StatefulSet with a PDB

Create the file `stateful-protect.yaml`:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mock-stateful
spec:
  serviceName: mock-stateful
  replicas: 7
  selector:
    matchLabels:
      app: mock-stateful
  template:
    metadata:
      labels:
        app: mock-stateful
    spec:
      priorityClassName: production-high
      terminationGracePeriodSeconds: 30
      containers:
      - name: app
        image: busybox
        command:
        - sh
        - -c
        - |
          trap 'echo ">>> START: Saving data to disk..."; sleep 10; echo ">>> END: Data saved, exiting."' TERM
          echo "Application started and running..."
          while true; do sleep 1; done
        resources:
          requests:
            cpu: "100m"
            memory: "256Mi"
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: mock-stateful-pdb
spec:
  minAvailable: 5
  selector:
    matchLabels:
      app: mock-stateful
```

Apply the configuration:

```shell
d8 k apply -f stateful-protect.yaml
```

Wait until all 7 pods are `Running`:

```shell
d8 k get pods -l app=mock-stateful -w
```

#### Step 2. Create a priority class higher than the application

Create the file `super-critical-pc.yaml`:

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: super-critical
value: 10000
description: "Maximum priority for emergency tasks"
```

Apply it:

```shell
d8 k apply -f super-critical-pc.yaml
```

#### Step 3. Simulate resource pressure

Create the file `emergency-task.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: emergency-task
spec:
  priorityClassName: super-critical
  containers:
  - name: task
    image: busybox
    command: ["sleep", "infinity"]
    resources:
      requests:
        cpu: "1"
        memory: "5Gi"
```

Apply the manifest:

```shell
d8 k apply -f emergency-task.yaml
```

#### Step 4. Observe the protection mechanisms

Check pod status:

```shell
d8 k get pods -l app=mock-stateful
```

Example expected output (during preemption):

```console
NAME                      READY   STATUS        RESTARTS   AGE
emergency-task            0/1     Pending       0          5s
mock-stateful-0           1/1     Terminating   0          55s
mock-stateful-1           1/1     Terminating   0          53s
mock-stateful-2           1/1     Running       0          51s
mock-stateful-3           1/1     Running       0          49s
mock-stateful-4           1/1     Running       0          47s
mock-stateful-5           1/1     Running       0          45s
mock-stateful-6           1/1     Terminating   0          43s
```

{% alert level="warning" %}
Logs of preempted pods are available only while the pod is in `Terminating`. To observe a graceful shutdown, run `d8 k logs` before the pod is fully deleted.
{% endalert %}

Check logs of a terminating pod:

```shell
d8 k logs mock-stateful-0 --tail=20
```

Example expected output:

```console
Application started and running...
>>> START: Saving data to disk...
>>> END: Data saved, exiting.
```

### How protection mechanisms work in a critical situation

Because the `emergency-task` priority (`10000`) is higher than the Stateful application (`9000`) and there are no other preemption candidates, the scheduler must select the Stateful application's pods for preemption. The protection mechanisms from [Protection against preemption](#protection-against-preemption) then apply:

1. In this case, PodDisruptionBudget attempts to limit the blast radius, but because the request is extremely large, the scheduler violates the PDB while still trying to minimize the number of deleted pods.
1. `terminationGracePeriodSeconds` helps keep data consistent even if the workload is fully preempted.

## Differences from other resource management mechanisms

PriorityClass is often confused with other resource management mechanisms. Their differences matter:

| Mechanism | Purpose | Difference from PriorityClass |
|----------|------------|--------------------------|
| Resource Quotas | Limits total resource consumption in a namespace | Caps aggregate usage; does not affect pod preemption |
| Limit Ranges | Default limits and requests for pods | Sets minimum and maximum values; does not affect preemption |

Unlike those mechanisms, PriorityClass affects the preemption order under resource pressure, not consumption caps.

## Operations and troubleshooting

### A pod does not start because of insufficient resources

If the cluster has no free resources and preemption is not possible, the pod stays `Pending`.

Check the pod status:

```shell
d8 k get pod high-priority-pod
```

Example output:

```console
NAME                READY   STATUS    RESTARTS   AGE
high-priority-pod   0/1     Pending   0          2m
```

Check the pod events:

```shell
d8 k describe pod high-priority-pod | grep -A10 "Events:"
```

Look for messages like this in `Events`:

```console
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  2m    default-scheduler  0/3 nodes are available: 2 Insufficient cpu, 1 Insufficient memory. preemption: 0/3 nodes are available: 3 Preemption is not helpful for scheduling.
```

What these messages mean:

- `Insufficient cpu` or `Insufficient memory` — nodes lack the requested CPU or memory.
- `Preemption is not helpful for scheduling` — preempting existing pods would not free enough resources (for example, all pods have equal or higher priority).

Check allocated resources on the nodes:

```shell
d8 k describe nodes | grep -A 5 "Allocated resources"
```

Example output:

```console
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests    Limits
  --------           --------    ------
  cpu                3800m (95%)  4200m (105%)
  memory             7Gi (87%)    8Gi (100%)
```

Possible solutions:

- Reduce `requests` in the pod manifest.
- Add capacity to the cluster (new nodes or larger existing ones).
- Delete unused pods.

### Preemption does not happen

If low-priority pods are not preempted even though resources look free, check events with reason `FailedPreemption`:

```shell
d8 k get events --field-selector reason=FailedPreemption --sort-by='.metadata.creationTimestamp'
```

Example output:

```console
LAST SEEN   TYPE      REASON             OBJECT                    MESSAGE
30s         Warning   FailedPreemption   pod/high-priority-pod     no preemption victims found for pod
```

Possible causes:

- Resource fragmentation. Free resources are spread across nodes, and no single node can fit the new pod even after preemption.
- No suitable victims. Every pod on the node has equal or higher priority than the new pod, so the scheduler cannot preempt them.

#### No lower-priority pods

If the pod does not start and events show `No preemption victims found for incoming pod`, check priorities of all pods on the node:

```shell
d8 k get pods --all-namespaces -o wide --field-selector spec.nodeName=worker-0 -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName,PRIORITY:.spec.priority --sort-by=.spec.priority
```

Example output:

```console
NAME                                                            NODE       PRIORITY
mock-stateful-5                                                 worker-0   9000
mock-stateful-2                                                 worker-0   9000
mock-stateful-3                                                 worker-0   9000
mock-stateful-4                                                 worker-0   9000
emergency-task                                                  worker-0   10000
multitenancy-manager-5968799d76-ktjgl                           worker-0   2000000000
csi-node-s82x4                                                  worker-0   2000001000
agent-wqzxq                                                     worker-0   2000001000
early-oom-6tkzg                                                 worker-0   2000001000
safe-agent-updater-ntrzq                                        worker-0   2000001000
kubernetes-api-proxy-worker-0                                   worker-0   2000001000
node-exporter-cddjm                                             worker-0   2000001000
oom-kills-exporter-pplfk                                        worker-0   2000001000
```

Check the exact reason why `mock-stateful-0` could not preempt other pods:

```shell
d8 k describe pod mock-stateful-0 | grep -A10 "Events:"
```

Example message from the pod events:

```console
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  13s   default-scheduler  0/2 nodes are available: preemption: 0/2 nodes are available: 1 No preemption victims found for incoming pod
```

| Message | Meaning |
|-----------|----------|
| `0/2 nodes are available` | The cluster has 2 nodes, but none is suitable for placing the pod |
| `1 No preemption victims found for incoming pod` | On the memory-constrained node there are no lower-priority pods that can be preempted |

Solution: raise the target pod's priority or free resources on the node manually.

#### Pod count limit on a node

Even after CPU and memory become free, the maximum pod count on a node can still block a high-priority pod.

Check the pod limit on the node:

```shell
d8 k describe node worker-0 | grep pods -A2
```

Example output:

```console
Capacity:
  pods:  120
```

Check the current number of pods on the node:

```shell
d8 k get pods --all-namespaces -o wide | grep worker-0 | wc -l
```

Example output:

```console
64
```

The node currently has 64 of 120 pods. There is still room.

Create the file `pod-filler.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pod-filler
spec:
  replicas: 110
  selector:
    matchLabels:
      app: filler
  template:
    metadata:
      labels:
        app: filler
    spec:
      priorityClassName: develop
      containers:
      - name: filler
        image: busybox
        command: ["sleep", "infinity"]
        resources:
          requests:
            cpu: "1m"
            memory: "5Mi"
```

Apply the manifest:

```shell
d8 k apply -f pod-filler.yaml
```

Wait until the Deployment fills the node:

```shell
d8 k get pods -l app=filler -o wide | grep worker-0 | wc -l
```

Create the `high-priority-pod.yaml` file to run a high-priority pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: high-priority-pod
spec:
  priorityClassName: production-high
  containers:
  - name: app
    image: nginx
    resources:
      requests:
        cpu: "100m"
        memory: "256Mi"
```

Apply the manifest:

```shell
d8 k apply -f high-priority-pod.yaml
```

Check the high-priority pod status:

```shell
d8 k get pod high-priority-pod
```

Example output:

```console
NAME                READY   STATUS    RESTARTS   AGE
high-priority-pod   0/1     Pending   0          11s
```

Inspect the cause in events:

```shell
d8 k describe pod high-priority-pod | grep -A10 "Events:"
```

Example output:

```console
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  11s   default-scheduler  0/2 nodes are available: 1 Too many pods, 1 node(s) had untolerated taint(s).
```

Confirm that preemption did not occur:

```shell
d8 k get events --field-selector reason=Preempted
```

Example output (empty):

```console
No resources found in default namespace.
```

What this means: the node pod limit is already reached, so even a high-priority pod cannot start. Preempting existing pods does not help because the pod count stays the same (the preempted pod is replaced by the new one).

### Useful commands for monitoring priorities

Count pods by priority class:

```shell
d8 k get pods -A -o jsonpath='{range .items[*]}{.spec.priorityClassName}{"\n"}{end}' | sort | uniq -c | sort -rn
```

Example expected output:

```console
     34 cluster-medium
     30 cluster-low
     26 system-cluster-critical
     18 system-node-critical
      6 production-high
```

{% alert level="info" %}
Events are retained for a limited time (usually about an hour). If there have been no recent preemptions, these commands may return nothing — repeat the preemption demo and run the commands immediately afterward.
{% endalert %}

List preemption events across all namespaces:

```shell
d8 k get events -A --field-selector reason=Preempted -o custom-columns=NAMESPACE:.metadata.namespace,POD:.involvedObject.name,MESSAGE:.message
```

Example expected output:

```console
NAMESPACE          POD                                                 MESSAGE
d8-chrony          chrony-master-9wzbl                                 Preempted by pod ac651aed-... on node master-0
d8-console         backend-58f9989c9d-4svjw                            Preempted by pod ac651aed-... on node master-0
d8-monitoring      prometheus-main-0                                   Preempted by pod 91f6e071-... on node worker-0
default            log-collector-dlxpv                                 Preempted by pod 91f6e071-... on node worker-0
```

Find which pods were preempted most often:

```shell
d8 k get events -A --field-selector reason=Preempted -o jsonpath='{range .items[*]}{.involvedObject.name}{"\n"}{end}' | sort | uniq -c | sort -rn | head -10
```

Example expected output:

```console
      2 prometheus-main-0
      2 memcached-0
      1 user-api-77494dc777-jzp7p
      1 upmeter-dex-authenticator-7f54c8dfb4-wwv22
      1 upmeter-dex-authenticator-7f54c8dfb4-wqfn5
      1 upmeter-dex-authenticator-7f54c8dfb4-h6784
      1 upmeter-dex-authenticator-7f54c8dfb4-bxsvk
      1 upmeter-dex-authenticator-7f54c8dfb4-28fgt
      1 upmeter-agent-4chrw
      1 status-dex-authenticator-786c6cc554-mfsdw
```

## Cleaning up resources created in the examples

After you finish the examples, delete the created resources. The commands below match [PriorityClass creation and usage examples](#priorityclass-creation-and-usage-examples), [Preemption mechanism](#preemption-mechanism), [Architectural usage scenarios](#architectural-usage-scenarios), [Protecting Stateful applications](#protecting-stateful-applications), and [Operations and troubleshooting](#operations-and-troubleshooting).

```shell
# Resources from "Creating a PriorityClass".
d8 k delete priorityclass my-app-critical

# Resources from "Using PriorityClass in a pod".
d8 k delete pod test-priority-pod

# Resources from "Using PriorityClass in a Deployment".
d8 k delete deployment my-app

# Resources from "Step-by-step preemption demo".
d8 k delete pod low-priority-pod
d8 k delete pod high-priority-pod

# Resources from "Separating environments by priority".
d8 k delete deployment prod-api
d8 k delete job manual-test
d8 k delete cronjob dev-data-processor

# Resources from "Protecting Stateful applications".
d8 k delete statefulset mock-stateful
d8 k delete pdb mock-stateful-pdb
d8 k delete pod emergency-task
d8 k delete priorityclass super-critical

# Resources from "Pod count limit on a node".
d8 k delete deployment pod-filler
```
