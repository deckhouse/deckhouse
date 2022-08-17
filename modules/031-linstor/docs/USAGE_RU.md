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
