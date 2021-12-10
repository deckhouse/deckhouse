---
title: "Модуль local-path-provisioner: примеры конфигурации"
---

## Пример CR `LocalPathProvisioner`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
```

Примечания:

- Этот пример создаст `localpath-system` класс (`storage class`) который **должен** быть использован в pod'ах что бы все заработало
- Все создаваемые хранилища будут иметь политику очистки `Delete` ([issue](https://github.com/deckhouse/deckhouse/issues/360))
- Если этот объект будет удален раньше чем объекты его использующие, папки с сервера удалены не будут
- Обратите внимание - в примере предпологается создание папок на системных нодах, которые скорее всего имеют ряд ограничителей (taints), а как следствие pod'ы **должны** иметь соотв. tolerations

### StatefulSet распределенный между системными нодами

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: demo
spec:
  nodeGroups:
  # мы собираемся хранить данные на системных нодах, а как следствие pod'ы ДОЛЖНЫ иметь соотв. tolerations
  - system
  # путь на ноде, где будут храниться наши данные, фактический путь будет похожим на "/mnt/kubernetes/demo/pvc-{guid}_{namespace}_{volumeclaimtemplates_name}-{statefulset_metadata_name}-{number}"
  path: /mnt/kubernetes/demo/
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  namespace: demo
  name: demo
  labels:
    app: demo
spec:
  serviceName: demo
  replicas: 2
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      # stage и prod ноды могут иметь разный набор taint
      tolerations:
      # stage
      - key: dedicated.deckhouse.io
        operator: Equal
        value: system
        effect: NoSchedule
      # prod
      - key: dedicated.deckhouse.io
        operator: Equal
        value: system
        effect: NoExecute
      # следующие настройки, вынудят кластер создать pod'ы на разных нодах, а как следствие local path provisioner так же создаст папки на разных нодах
      affinity:
        podAntiAffinity:
          # должно работать для prod (2 ноды) и stage (1 нода), для более жесткого ограничения используйте "requiredDuringSchedulingIgnoredDuringExecution"
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 1
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - demo
              topologyKey: kubernetes.io/hostname
      containers:
      - name: demo
        image: nginx:alpine
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 80
        volumeMounts:
        - name: demo
          mountPath: /usr/share/nginx/html
  volumeClaimTemplates:
  - metadata:
      name: demo
    spec:
      accessModes:
      - ReadWriteOnce
      # storage class созданный local path provisioner ДОЛЖЕН быть использован здесь
      storageClassName: demo
      resources:
        requests:
          storage: 128Mi
```
