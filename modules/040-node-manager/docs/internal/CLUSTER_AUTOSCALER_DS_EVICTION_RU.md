---
title: "Cluster Autoscaler: Примеры"
description: Примеры настройки Cluster Autoscaler в Kubernetes. Аннотации для DaemonSet.
---

## Описание

<https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-enabledisable-eviction-for-a-specific-daemonset>

Вы можете отключить выселение (evict) подов DaemonSet указав следующую аннотацию:

```console
{{ include "helm_lib_prevent_ds_eviction_annotation" . | nindent 8 }}
```

## Почему это важно

CNI и CSI обычно работают в подах DaemonSet, как и агенты мониторинга. Когда Cluster Autoscaler уменьшает количество узлов, он начинает с выселения подов. Если поды CNI/CSI выселяются до подов с пользовательскими рабочими нагрузками, то пользовательские поды не могут корректно завершить работу.

### Запуск подов, которые вызовут добавление новых узлов Cluster Autoscaler и ожидание готовности узлов

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

### Запуск подов, которые требуют много времени для завершения

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

### Запуск фиктивных подов для имитации пользовательских рабочих нагрузок

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

### Масштабирование Deployment resource-consumer

```bash
kubectl scale deployment resource-consumer --replicas 0
```

### Результат

Вы заметите, что ресурсы dummy-pod и DaemonSet (включая CNI/CSI, Prometheus exporter и log-shipper) выселяются, пока поды long-terminating все еще пытаются завершить работу.
