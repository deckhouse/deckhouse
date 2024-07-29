---
title: "Cluster Autoscaler: Examples"
description: Examples of configuring Cluster Autoscaler in Kubernetes. Annotations for DaemonSet.
---

## Description

<https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-enabledisable-eviction-for-a-specific-daemonset>

You can disable eviction of DaemonSet pods by specifying this annotation:

```console
{{ include "helm_lib_prevent_ds_eviction_annotation" . | nindent 8 }}
```

## Why This Matters

CNI and CSI usually run in DaemonSet pods, as do monitoring agents. When Cluster Autoscaler reduces the number of nodes, it starts by evicting pods. If CNI/CSI pods are evicted before the pods with user workloads, the user pods cannot shut down properly.
Reproducing the Issue

### Launch pods that will cause Cluster Autoscaler to add new nodes and wait until the nodes are ready

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: resource-consumer
  namespace: default
spec:
  replicas: 5
  selector:
    matchLabels:
      app: resource-consumer
  template:
    metadata:
      labels:
        app: resource-consumer
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app
                    operator: In
                    values:
                      - resource-consumer
              topologyKey: "kubernetes.io/hostname"
      containers:
      - name: resource-consumer
        image: busybox
        resources:
          requests:
            cpu: "2"
            memory: "2Gi"
        command: ["/bin/sh"]
        args: ["-c", "while true; do echo 'Consuming resources'; sleep 3600; done"]
```

### Launch pods that take a long time to terminate

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: long-terminating-script
  namespace: default
data:
  app.py: |
    import signal
    import time

    def handler(signum, frame):
        print('Signal received, ignoring for 5 minutes...')
        time.sleep(300)
        print('Exiting...')
        exit(0)

    signal.signal(signal.SIGTERM, handler)
    signal.signal(signal.SIGINT, handler)

    while True:
        time.sleep(3600)
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: long-terminating
  namespace: default
spec:
  replicas: 5
  selector:
    matchLabels:
      app: long-terminating
  template:
    metadata:
      labels:
        app: long-terminating
    spec:
      terminationGracePeriodSeconds: 600
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app
                      operator: In
                      values:
                        - long-terminating
                topologyKey: "kubernetes.io/hostname"
      containers:
      - name: long-terminating
        image: python:3.11-slim
        command: ["python", "/app/app.py"]
        volumeMounts:
        - name: script-volume
          mountPath: /app
        resources:
          requests:
            cpu: "0.1"
            memory: "100Mi"
      volumes:
      - name: script-volume
        configMap:
          name: long-terminating-script
```

### Launch dummy pods to simulate user workloads

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dummy-pods
  namespace: default
spec:
  replicas: 30
  selector:
    matchLabels:
      app: dummy-pod
  template:
    metadata:
      labels:
        app: dummy-pod
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app
                      operator: In
                      values:
                        - dummy-pod
                topologyKey: "kubernetes.io/hostname"
      containers:
      - name: dummy-container
        image: busybox
        resources:
          requests:
            cpu: "5m"
            memory: "16Mi"
        command: ["/bin/sh"]
        args: ["-c", "while true; do echo 'Dummy pod'; sleep 3600; done"]
```

### Scale down the resource-consumer deployment

```bash
kubectl scale deployment resource-consumer --replicas 0
```

### Outcome

You will observe that the dummy-pod and DaemonSet resources (including CNI/CSI, Prometheus exporter, and log-shipper) are evicted while the long-terminating pods are still trying to finish.
