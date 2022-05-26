---
title: "Модуль linstor: примеры конфигурации"
---

## Использование планировщика linstor

Планировщик `linstor` учитывает размещение данных в хранилище и старается размещать Pod в первую очередь на тех узлах, где данные доступны локально. Включается добавлением параметра `schedulerName: linstor` в описание Pod'а приложения.

Пример описания Pod'а, использующего планировщик `linstor`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
spec:
  schedulerName: linstor # Использование планировщика linstor
  containers:
  - name: busybox
    image: busybox
    command: ["tail", "-f", "/dev/null"]
    volumeMounts:
    - name: my-first-linstor-volume
      mountPath: /data
    ports:
    - containerPort: 80
  volumes:
  - name: my-first-linstor-volume
    persistentVolumeClaim:
      claimName: "test-volume"
```

## Перенос приложения на другой узел при проблемах с хранилищем (fencing)

При наличии лейбла `linstor.csi.linbit.com/on-storage-lost: remove` у Pod'а, модуль linstor автоматически удалит Pod'ы с узла где возникли проблемы с хранилищем, что вызовет их перезапуск на другом узле. 

Пример описания StatefulSet с лейблом `linstor.csi.linbit.com/on-storage-lost: remove`:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-stateful-app
spec:
  serviceName: my-stateful-app
  selector:
    matchLabels:
      app.kubernetes.io/name: my-stateful-app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: my-stateful-app
        linstor.csi.linbit.com/on-storage-lost: remove # <--
    ...
```
