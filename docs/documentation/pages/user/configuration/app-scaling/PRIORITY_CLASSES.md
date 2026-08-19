---
title: "Pod priorities"
permalink: en/user/configuration/app-scaling/priority-classes.html
description: "Using PriorityClass in Deckhouse Kubernetes Platform: pod priority configuration examples, preemption demonstrations, and practical diagnostics."
---

## Using PriorityClass in a pod

In addition to the priority classes predefined in Deckhouse Kubernetes Platform (DKP), a cluster administrator can create a custom PriorityClass. This example uses the `my-app-critical` class — see [Creating custom priority classes](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#creating-custom-priority-classes) for how to create it.

Create a file `test-pod.yaml` to run a pod with the `my-app-critical` class:

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

Apply the manifest to the cluster:

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
You cannot change the `priorityClassName` field of an existing pod. This field is immutable. To change the priority class, delete the pod and recreate it with a different class.
{% endalert %}

## Using PriorityClass in a Deployment

You can specify a PriorityClass in a Deployment's pod template. In this case, all pods created by the Deployment use the specified priority class.

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

## Step-by-step preemption demonstration

This example shows how the scheduler preempts a lower-priority pod when a node lacks sufficient resources to schedule a higher-priority pod. First, a pod with the `develop` class (`1000`) is started on a worker node and consumes most of the free resources. Then a pod with the `production-high` class (`9000`) is created with the same `requests` — the node does not have enough resources to accommodate both pods, and the scheduler preempts the lower-priority pod to make room for the higher-priority pod.

In the examples, replace `worker-0` with the name of your worker node.

{% alert level="info" %}
Behavior depends on the number of worker nodes and may differ from this example: the demonstration is designed for a cluster with a single worker node. If there are multiple worker nodes, pods may be distributed across nodes, and preemption on one node may not occur.
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

In this example, the node has approximately 2600m of CPU and 5Gi of memory available. Create a low-priority pod that consumes a significant portion of these resources, then a high-priority pod that requests more resources than remain available.

{% alert level="info" %}
The `requests` values in this example (2 CPU and 4Gi memory) are chosen for a node with 4 CPU and 8Gi memory where about 30% of resources are already used. If your cluster nodes are more or less powerful, adjust these values so that the low-priority pod consumes approximately 70–80% of the node's free resources.
{% endalert %}

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

Pod `high-priority-pod` is in `Running` status. The `low-priority-pod` pod may no longer appear in the output because it may have been preempted and removed within seconds. In some cases, you may see `low-priority-pod` in `Pending` status.

Check preemption events:

```shell
d8 k get events --field-selector reason=Preempted
```

Example output:

```console
LAST SEEN   TYPE     REASON      OBJECT                 MESSAGE
68s         Normal   Preempted   pod/low-priority-pod   Preempted by pod d9d25b95-4a7d-4214-8a30-8ce1fd616f67 on node worker-0
```

## Environment separation by priority

This scenario shows how priorities help protect production workloads at the cluster level — not only within a single namespace. Unlike the [Step-by-step preemption demonstration](#step-by-step-preemption-demonstration), which compares two standalone pods, this example models a more typical scenario: a background task from a development environment occupies a node, and then a critical production service with higher priority must be started.

Separating environments by namespace alone does not protect critical services from resource shortages at the cluster level. Priority classes establish which workloads should take precedence when cluster resources are insufficient.

Suppose a cluster node is fully loaded by a background task from the development environment. A critical production service then needs to scale up. Thanks to the priority difference (`9000` for `production-high` versus `1000` for `develop`), the scheduler automatically preempts the development task to free resources for the critical production service.

{% alert level="info" %}
With these priority classes, development workloads cannot preempt production workloads.
{% endalert %}

In the development environment, create a file `dev-data-processor.yaml` to run a background task with the predefined `develop` class:

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
`command: ["sleep", "infinity"]` is used for demonstration purposes. It keeps the container running indefinitely and holding the requested resources, making it possible to demonstrate resource shortage without deploying a real application.
{% endalert %}

Create a job manually for immediate execution:

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

In the production environment, create a file `prod-api.yaml` and run a critical service with high priority using the predefined `production-high` class:

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

## Protecting stateful applications

Stateful applications, such as databases and message queues, maintain persistent state in memory or on persistent volumes (PVCs). Protecting them requires special care because sudden pod termination without graceful shutdown can corrupt data, and mass preemption of replicas leads to service unavailability.

For more on protection from eviction mechanisms, see [Protection from eviction](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#protection-from-eviction). The [Demo of stateful application protection](#demo-of-stateful-application-protection) subsection shows a practical scenario in which these mechanisms work together.

To protect stateful applications in this example, use a combination of three mechanisms:

- A high `priorityClassName` (recommended: `production-high` with value `9000` or higher, see [Available priority classes](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#available-priority-classes)) makes pods less likely to be selected for preemption.
- PodDisruptionBudget (PDB) helps maintain a minimum number of available replicas (for example, `minAvailable: 5`).
- The `terminationGracePeriodSeconds` field gives the application time to flush data to disk and close transactions before the pod is terminated (recommended: 30–60 seconds).

### Demo of stateful application protection

This example deploys a sample stateful application: it is assigned the `production-medium` class (`6000`) together with PodDisruptionBudget and `terminationGracePeriodSeconds` to limit the scope of preemption and give pods time to shut down gracefully. Then pod `emergency-task` is created with the higher `production-high` class (`9000`) and a large memory request — to simulate a critical resource shortage on the node and verify how protection mechanisms behave.

#### Step 1. Create a protected StatefulSet with PDB

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

Wait until all 7 pods reach `Running` status:

```shell
d8 k get pods -l app=mock-stateful -w
```

#### Step 2. Simulate resource shortage

Create a file `emergency-task.yaml` to run a pod with the predefined `production-high` class (value `9000`), which is higher than the stateful application (`production-medium`, value `6000`):

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

#### Step 3. Observe protection mechanisms

Check pod status:

```shell
d8 k get pods -l app=mock-stateful
```

Example output while preemption is in progress:

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
Logs of terminating pods are available only while the pod is in `Terminating` status. To see graceful shutdown, run `d8 k logs` before the pod is fully deleted.
{% endalert %}

Check logs of the terminating pod:

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

Since the priority of pod `emergency-task` created in the [Demo of stateful application protection](#demo-of-stateful-application-protection) (`9000`, class `production-high`) is higher than that of the stateful application (`6000`, class `production-medium`), and there are no other preemption candidates, the scheduler is forced to preempt pods of the stateful application. At the same time, protection mechanisms are triggered:

1. In this case, PodDisruptionBudget tries to limit the scope of damage, but because the request is extremely large, the scheduler cannot honor the PDB while trying to minimize the number of pods removed.
1. `terminationGracePeriodSeconds` gives affected pods time to shut down gracefully, reducing the risk of data loss during preemption.

## Operations and diagnostics

This section provides practical commands to check pod status and scheduler events. Descriptions of problem causes and possible cluster-level actions are provided in the [Operations and diagnostics](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#operations-and-diagnostics) admin section.

In the [Step-by-step preemption demonstration](#step-by-step-preemption-demonstration), pod `high-priority-pod` reaches `Running` status. The following subsections examine why the same pod might remain `Pending`, including insufficient resources and the absence of suitable preemption candidates.

### Pod does not start due to insufficient resources

If the cluster has no free resources and preemption is impossible, the pod remains in `Pending` status.

Check pod status:

```shell
d8 k get pod high-priority-pod
```

Example output:

```console
NAME                READY   STATUS    RESTARTS   AGE
high-priority-pod   0/1     Pending   0          2m
```

Check pod events:

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

- `Insufficient cpu` or `Insufficient memory` — nodes lack the requested resources (CPU and memory, respectively).
- `Preemption is not helpful for scheduling` — preempting existing pods will not free enough resources (for example, all pods have equal or higher priority).

Check available resources on nodes:

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
- Increase cluster capacity by adding nodes or increasing the resources available to existing nodes.
- Delete unused pods.

### Practical diagnostics when preemption is impossible

If a higher-priority pod remains `Pending` and lower-priority pods are not preempted, check events with reason `FailedPreemption`:

```shell
d8 k get events --field-selector reason=FailedPreemption --sort-by='.metadata.creationTimestamp'
```

Example output:

```console
LAST SEEN   TYPE      REASON             OBJECT                    MESSAGE
30s         Warning   FailedPreemption   pod/high-priority-pod     no preemption victims found for pod
```

For causes and cluster-level actions, see [Preemption does not occur](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#preemption-does-not-occur).

### Practical check when no suitable pods are available

This example uses the cluster state [after the Demo of stateful application protection](#demo-of-stateful-application-protection). Pod `mock-stateful-0` has priority `6000`. On the target node, other `mock-stateful` pods have the same priority, `emergency-task` has a higher priority (`9000`), and system pods have an even higher priority. Therefore, the scheduler cannot preempt any of them to run `mock-stateful-0`: there are no lower-priority pods on the node.

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

Example message from pod events:

```console
Events:
  Type     Reason            Age   From               Message
  ----     ------            ----  ----               -------
  Warning  FailedScheduling  13s   default-scheduler  0/2 nodes are available: preemption: 0/2 nodes are available: 1 No preemption victims found for incoming pod
```

| Message | Meaning |
|---------|---------|
| `0/2 nodes are available` | The cluster has 2 nodes, but neither can accommodate the pod. |
| `1 No preemption victims found for incoming pod` | On the node with insufficient memory, there are no lower-priority pods that can be preempted. |

For cluster-level actions, see [No lower-priority pods available](/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/pod-eviction/priority-classes.html#no-lower-priority-pods-available).

### Practical example "Pod limit per node"

Even if CPU and memory are freed, the limit on the maximum number of pods per node can prevent a high-priority pod from starting.

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

The node currently has 64 pods out of 120. There is still free capacity.

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

Check the high-priority pod status:

```shell
d8 k get pod high-priority-limit-pod
```

Example output:

```console
NAME                      READY   STATUS    RESTARTS   AGE
high-priority-limit-pod   0/1     Pending   0          11s
```

Check the reason in events:

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

Verify that preemption did not occur:

```shell
d8 k get events --field-selector reason=Preempted
```

Example output (empty):

```console
No resources found in default namespace.
```

The pod cannot start because the node has reached its pod limit. Preempting an existing pod does not help because replacing one pod with another does not reduce the total number of pods on the node.

### Useful commands for monitoring priorities

Count pods by priority class:

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

{% alert level="info" %}
Events are stored for a limited time (usually about an hour). If there has been no preemption for a long time, these commands may return nothing — repeat the preemption demonstration and run the commands immediately afterward.
{% endalert %}

View preemption events across all namespaces:

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

Check which pods were preempted most often:

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
