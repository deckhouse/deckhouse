---
title: "Priority classes"
permalink: en/user/configuration/app-scaling/priority-classes.html
description: "Using priority classes in Deckhouse Kubernetes Platform: configuration examples, preemption demonstration, and practical diagnostics."
---

A priority class defines which pods are more important when a node runs out of resources. You can assign a class in a Deployment manifest, verify preemption, and figure out why a pod is stuck in `Pending`.

## Using a priority class in a Deployment

Deckhouse Kubernetes Platform (DKP) already provides a set of priority classes. The following example shows how to use a priority class in a Deployment pod template.

Create a file `deployment-with-priority.yaml` to deploy an application with the DKP predefined class `production-high` (value `9000`, see [Available priority classes](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#available-priority-classes)):

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
      # Specify the priority class name.
      priorityClassName: production-high
      containers:
      - name: app
        image: nginx:latest
        ports:
        - containerPort: 80
```

Apply the manifest to the cluster:

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

{% alert level="warning" %}
You cannot change the `priorityClassName` field of a running pod. This field is immutable. To change the priority, update the template in the Deployment and recreate the pods.
{% endalert %}

## Step-by-step preemption demonstration

This example shows how the scheduler preempts a lower-priority pod when a node lacks sufficient resources for a higher-priority pod. First, a pod with the `develop` class (`1000`) is started on a worker node and consumes most of the free resources. Then a pod with the `production-high` class (`9000`) is created with the same `requests` — there is no room left on the node, and the scheduler preempts the lower-priority pod in favor of the new one.

In the examples, replace `worker-0` with the name of your worker node.

{% alert level="info" %}
The example is designed for a single worker node with 4 CPU and 8 Gi of memory. If there are multiple nodes, pods may be placed on different nodes, and preemption may not occur. Adjust `requests` so that the low-priority pod consumes most of the node's free resources.
{% endalert %}

Check the list of nodes in the cluster:

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

Create a low-priority pod that consumes a significant portion of the free resources, then a high-priority pod that requests more than remains available.

Create a file `low-priority-pod.yaml`:

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

Wait until the pod reaches `Running` status:

```shell
d8 k get pods low-priority-pod
```

Example output:

```console
NAME               READY   STATUS    RESTARTS   AGE
low-priority-pod   1/1     Running   0          10s
```

Create a file `high-priority-pod.yaml` to run a high-priority pod that requests more resources than remain available on the node:

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

Pod `high-priority-pod` is in `Running` status. The `low-priority-pod` pod may no longer appear in the output because the scheduler preempted it within seconds. In some cases, you may see `low-priority-pod` in `Pending` status.

Check preemption events:

```shell
d8 k get events -A --field-selector reason=Preempted
```

Example output:

```console
NAMESPACE   LAST SEEN   TYPE     REASON      OBJECT                 MESSAGE
default     68s         Normal   Preempted   pod/low-priority-pod   Preempted by pod d9d25b95-4a7d-4214-8a30-8ce1fd616f67 on node worker-0
```

## Environment separation by priority

A namespace itself does not affect preemption: the scheduler compares only pod priority classes, regardless of which namespaces the pods are in. To protect production workloads, use a lower priority class in test and develop environments (`develop`, `staging`), and a higher one in production (`production-low`, `production-medium`, `production-high`).

## Protecting stateful applications

Stateful applications (for example, databases and message queues) store data in memory or persistent volumes (PVC). Protecting them requires a special approach: abruptly destroying a pod without a graceful shutdown can corrupt data, and mass preemption of replicas can make the service unavailable.

For more on protection from eviction, see [Protection from eviction](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#protection-from-eviction). The [Demo of stateful application protection](#demo-of-stateful-application-protection) subsection shows a practical scenario where these mechanisms work together.

This example uses a combination of three mechanisms to protect stateful applications:

- A high `priorityClassName` — in the example, the StatefulSet uses `production-medium` (`6000`). For real critical stateful applications, usually use `production-high` (`9000`) or higher; see [Available priority classes](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#available-priority-classes). This reduces the likelihood that the pods will be selected for preemption.
- PodDisruptionBudget (PDB) guarantees a minimum number of running replicas (for example, `minAvailable: 5`).
- The `terminationGracePeriodSeconds` parameter gives the application time to write data to disk and close transactions before the pod terminates (30–60 seconds is recommended).

### Demo of stateful application protection

This example deploys a sample stateful application with the `production-medium` class (`6000`), a PodDisruptionBudget, and `terminationGracePeriodSeconds` to limit the scale of preemption and give pods time to shut down gracefully. Then an `emergency-task` pod is created with the higher `production-high` class (`9000`) and a large memory request — to simulate a critical resource shortage on the node and show how the protection mechanisms work.

#### Step 1. Create a protected StatefulSet with a PDB

Create a file `stateful-protect.yaml`:

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
      priorityClassName: production-medium
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

Wait until all 7 pods are in `Running` status:

```shell
d8 k get pods -l app=mock-stateful -w
```

#### Step 2. Simulate a resource shortage

Create a file `emergency-task.yaml` to run a pod with the predefined `production-high` class (value `9000`), which is higher than the stateful application's class (`production-medium`, value `6000`):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: emergency-task
spec:
  priorityClassName: production-high
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

#### Step 3. Observe the protection mechanisms

Check pod status:

```shell
d8 k get pods | grep -E 'mock-stateful|emergency-task'
```

Example output:

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
Logs of preempted pods are available only while the pod is in `Terminating` status. To see graceful shutdown, run `d8 k logs` before the pod is fully deleted.
{% endalert %}

Check the logs of a terminating pod:

```shell
d8 k logs mock-stateful-0 --tail=20
```

Example output:

```console
Application started and running...
>>> START: Saving data to disk...
>>> END: Data saved, exiting.
```

### How protection mechanisms work in a critical situation

Because the priority of the `emergency-task` pod created in [Demo of stateful application protection](#demo-of-stateful-application-protection) (`9000`, class `production-high`) is higher than that of the stateful application (`6000`, class `production-medium`), and there are no other preemption candidates, the scheduler must select the stateful application for preemption. The protection mechanisms then take effect:

1. In this case, the PodDisruptionBudget tries to limit the impact, but because the request is extremely large, the scheduler violates the PDB while still trying to minimize the number of pods removed.
1. The `terminationGracePeriodSeconds` parameter helps preserve data even if preemption is complete.

## Operations and diagnostics

This section provides practical commands to check pod status and scheduler events. Descriptions of problem causes and possible cluster-level actions are in the [Operations and diagnostics](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#operations-and-diagnostics) admin section.

In the [Step-by-step preemption demonstration](#step-by-step-preemption-demonstration), the `high-priority-pod` reaches `Running`. The following subsections cover two cases where the same pod stays in `Pending`: insufficient CPU or memory, and the scheduler finding no pods to preempt.

### Pod does not start due to insufficient resources

If the cluster has no free resources and preemption is not possible, the pod remains in `Pending`.

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

Look for messages like the following in the `Events` section:

```console
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  2m    default-scheduler  0/3 nodes are available: 2 Insufficient cpu, 1 Insufficient memory. preemption: 0/3 nodes are available: 3 Preemption is not helpful for scheduling.
```

What these messages mean:

- `Insufficient cpu` or `Insufficient memory` — nodes lack the requested CPU or memory resources.
- `Preemption is not helpful for scheduling` — preempting existing pods will not free enough resources (for example, all pods have equal or higher priority).

Check available resources on the nodes:

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
- Add resources to the cluster (new nodes or larger existing ones).
- Delete unused pods.

### Practical diagnostics when preemption is impossible

If lower-priority pods are not preempted even though resources appear free, check events with reason `FailedPreemption`:

```shell
d8 k get events -A --field-selector reason=FailedPreemption --sort-by='.metadata.creationTimestamp'
```

Example output:

```console
NAMESPACE   LAST SEEN   TYPE      REASON             OBJECT                    MESSAGE
default     30s         Warning   FailedPreemption   pod/high-priority-pod     no preemption victims found for pod
```

The message `no preemption victims found for pod` usually means there are no lower-priority pods on the node that can be preempted for the new pod. For a walkthrough of this case, see [Practical check when no suitable pods are available](#practical-check-when-no-suitable-pods-are-available).

A pod may also fail to start for reasons unrelated to the priority class: for example, there are no suitable nodes because of taints and tolerations. In that case, events show `FailedScheduling` with messages such as `untolerated taint(s)`, not `FailedPreemption`.

For possible cluster-level actions, see [Operations and diagnostics](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#operations-and-diagnostics).

### Practical check when no suitable pods are available

This example uses the cluster state [after Demo of stateful application protection](#demo-of-stateful-application-protection). Pod `mock-stateful-0` has priority `6000`. On the target node, the other `mock-stateful` pods have the same priority, `emergency-task` has a higher priority (`9000`), and system pods have an even higher priority. Therefore, the scheduler cannot preempt any of them to schedule `mock-stateful-0`: there are no lower-priority pods on the node.

If events contain the message `No preemption victims found for incoming pod`, check pod priorities on the node:

```shell
d8 k get pods --all-namespaces -o wide --field-selector spec.nodeName=worker-0 -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName,PRIORITY:.spec.priority --sort-by=.spec.priority
```

Example output:

```console
NAME                                                            NODE       PRIORITY
mock-stateful-5                                                 worker-0   6000
mock-stateful-2                                                 worker-0   6000
mock-stateful-3                                                 worker-0   6000
mock-stateful-4                                                 worker-0   6000
emergency-task                                                  worker-0   9000
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
|---------|---------|
| `0/2 nodes are available` | The cluster has 2 nodes, but none can accommodate the pod. |
| `1 No preemption victims found for incoming pod` | On the node with insufficient memory, there are no lower-priority pods that can be preempted. |

For cluster-level actions, see [Operations and diagnostics](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#operations-and-diagnostics).

### Practical example "Pod limit per node"

Even when CPU and memory become available, the maximum number of pods per node can prevent a high-priority pod from starting.

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

Create a file `pod-filler.yaml`:

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

Create a file `high-priority-limit-pod.yaml` to run a high-priority pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: high-priority-limit-pod
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
d8 k apply -f high-priority-limit-pod.yaml
```

Check the status of the high-priority pod:

```shell
d8 k get pod high-priority-limit-pod
```

Example output:

```console
NAME                      READY   STATUS    RESTARTS   AGE
high-priority-limit-pod   0/1     Pending   0          11s
```

Check the reason in the events:

```shell
d8 k describe pod high-priority-limit-pod | grep -A10 "Events:"
```

Example output:

```console
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  11s   default-scheduler  0/2 nodes are available: 1 Too many pods, 1 node(s) had untolerated taint(s).
```

Verify that no preemption occurred:

```shell
d8 k get events -A --field-selector reason=Preempted
```

Example output:

```console
No resources found
```

The issue: the pod limit on the node has already been reached, so even a high-priority pod cannot start. Preempting existing pods does not help because the number of pods stays the same (the preempted pod is replaced by the new one).

### Useful commands for monitoring priorities

Pod count by priority class:

```shell
d8 k get pods -A -o jsonpath='{range .items[*]}{.spec.priorityClassName}{"\n"}{end}' | sort | uniq -c | sort -rn
```

Example output:

```console
     34 cluster-medium
     30 cluster-low
     26 system-cluster-critical
     18 system-node-critical
      6 production-high
```

Preemption events across all namespaces:

```shell
d8 k get events -A --field-selector reason=Preempted -o custom-columns=NAMESPACE:.metadata.namespace,POD:.involvedObject.name,MESSAGE:.message
```

Example output:

```console
NAMESPACE          POD                                                 MESSAGE
d8-chrony          chrony-master-9wzbl                                 Preempted by pod ac651aed-... on node master-0
d8-console         backend-58f9989c9d-4svjw                            Preempted by pod ac651aed-... on node master-0
d8-monitoring      prometheus-main-0                                   Preempted by pod 91f6e071-... on node worker-0
default            log-collector-dlxpv                                 Preempted by pod 91f6e071-... on node worker-0
```

{% alert level="info" %}
Events are retained for a limited time (usually about an hour). If preemption has not occurred recently, these commands may return nothing — repeat the preemption demonstration and run the commands immediately afterward.
{% endalert %}

Most frequently preempted pods:

```shell
d8 k get events -A --field-selector reason=Preempted -o jsonpath='{range .items[*]}{.involvedObject.name}{"\n"}{end}' | sort | uniq -c | sort -rn | head -10
```

Example output:

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
