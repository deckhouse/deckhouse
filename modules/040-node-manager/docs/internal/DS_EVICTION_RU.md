---
title: "Cluster Autoscaler: примеры"
description: Примеры настройки Cluster Autoscaler в Kubernetes. Аннотации для DaemonSet.
---

## Описание

<https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-enabledisable-eviction-for-a-specific-daemonset>

Cluster Autoscaler выселяет DaemonSets по аннотации:

`"cluster-autoscaler.kubernetes.io/enable-ds-eviction": "true"`

Можно отключить выселение:

`"cluster-autoscaler.kubernetes.io/enable-ds-eviction": "false"`

Аннотацию нужно указать на подах DaemonSet.

Чтобы не назначать аннотацию на каждый d8 DaemonSet мы делаем патч, который исключает выселение ds подов из namespace d8-*.

## Почему это важно

Как правило, CNI и CSI запускаются в DaemonSet подах. DaemonSet также используется для агентов мониторинга. Когда Cluster Autoscaler начинает процесс уменьшения количества узлов, он сначала выселяет поды. Если поды с CNI/CSI будут выселены раньше, чем поды с пользовательской нагрузкой, последние не смогут корректно завершиться.

## Как воспроизвести проблему

Проблема возникает, если патч не работает или DaemonSet поды не имеют аннотации cluster-autoscaler.kubernetes.io/enable-ds-eviction.

1. Запустите поды, которые заставят Cluster Autoscaler добавить новые узлы, и дождитесь готовности узлов.

```
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

2. Запустите поды, которые долго завершаются.

```
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

3. Запустите пустые поды для симуляции пользовательской нагрузки.

```
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

4. Уменьшите количество реплик для resource-consumer.

```
kubectl scale deployment resource-consumer --replicas 0
```

5. Итог

В результате вы увидите, что поды dummy-pod и ресурсы DaemonSet (включая CNI/CSI, Prometheus exporter и log-shipper) удалены с ноды, в то время как long-terminating поды всё ещё пытаются завершить работу.
