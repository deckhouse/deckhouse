---
title: "Обновление Kubernetes и управление версиями"
permalink: ru/admin/configuration/platform-scaling/control-plane/updating-and-versioning.html
lang: ru
---

## Обновление и управление версиями

Процесс обновления control plane в DKP полностью автоматизирован.

- В DKP поддерживаются последние пять версий Kubernetes.
- Control plane можно откатывать на одну минорную версию назад и обновлять на несколько версий вперёд — шаг за шагом, по одной версии за раз.
- Patch-версии (например, `1.27.3` → `1.27.5`) обновляются автоматически вместе с версией Deckhouse, и управлять этим процессом нельзя.
- Minor-версии задаются вручную в [параметре `kubernetesVersion`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) в ресурсе ClusterConfiguration.

### Изменение версии Kubernetes

1. Откройте редактирование [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration):

   ```shell
   d8 system edit cluster-configuration
   ```

1. Установите желаемую версию Kubernetes (`kubernetesVersion`):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterConfiguration
   cloud:
     prefix: demo-stand
     provider: Yandex
   clusterDomain: cloud.education
   clusterType: Cloud
   defaultCRI: Containerd
   kubernetesVersion: "1.30"
   podSubnetCIDR: 10.111.0.0/16
   podSubnetNodeCIDRPrefix: "24"
   serviceSubnetCIDR: 10.222.0.0/16
   ```

1. Сохраните изменения.
1. Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `d8 k get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

## Мониторинг хода обновления

Модуль [control-plane-manager](/modules/control-plane-manager/) включает компонент update-observer, который даёт актуальную информацию о процессе обновления версии Kubernetes в кластере.

Update-observer:

- читает конфигурацию кластера из Secret `d8-cluster-configuration`;
- отслеживает версии kubelet на всех узлах через `nodeInfo.kubeletVersion`;
- собирает версии со всех экземпляров control plane по аннотации `control-plane-manager.deckhouse.io/kubernetes-version`;
- создаёт и поддерживает ConfigMap **`d8-cluster-kubernetes`** в пространстве имён `kube-system` с детальным статусом обновления.

В ConfigMap `d8-cluster-kubernetes` отображаются:

- **Статус по компонентам** — версии компонентов control plane (kube-apiserver, kube-scheduler, kube-controller-manager) на каждом master-узле;
- **Прогресс по узлам** — сколько узлов уже обновлено и сколько всего;
- **Целевая и текущая версия** — желаемая версия из конфигурации и фактическое состояние во время обновления;
- **Расхождение версий** — если какие-то компоненты работают на версии, отличной от целевой (в том числе новее желаемой).

Таким образом, вы можете в реальном времени видеть, какие компоненты обновляются, на каком этапе процесс и не «застряло» ли обновление на каком-либо узле или компоненте.

Для просмотра статуса обновления выполните команду:

```shell
kubectl get configmap d8-cluster-kubernetes -n kube-system -o yaml
```

### Примеры содержимого ConfigMap

В `data.spec` и `data.status` хранится YAML с полем `spec` (целевая версия и режим обновления) и полем `status` (текущее состояние). Ниже представлены примеры содержимого для различных ситуаций.

#### Кластер в актуальном состоянии (3 master-узла, 3 worker-узла)

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: v1.32
    updateMode: Manual
  status: |
    currentVersion: v1.32
    phase: UpToDate
    controlPlane:
    - name: mazin-master-1
      phase: UpToDate
      components:
        kube-apiserver: v1.32
        kube-controller-manager: v1.32
        kube-scheduler: v1.32
    - name: mazin-master-2
      phase: UpToDate
      components:
        kube-apiserver: v1.32
        kube-controller-manager: v1.32
        kube-scheduler: v1.32
    - name: mazin-master-0
      phase: UpToDate
      components:
        kube-apiserver: v1.32
        kube-controller-manager: v1.32
        kube-scheduler: v1.32
    nodes:
      desiredCount: 6
      upToDateCount: 6
kind: ConfigMap
metadata:
  annotations:
    cause: idle
    lastReconciliationTime: "2026-02-02T01:13:05Z"
    lastUpToDateTime: "2026-01-30T16:26:36Z"
  creationTimestamp: "2026-01-16T16:48:45Z"
  labels:
    heritage: deckhouse
    k8s-version: v1.32
    max-k8s-version: v1.33
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "20837731"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Начало обновления (например, понижение версии Kubernetes)

Целевая версия уже задана, control plane или узлы ещё переходят на неё.

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: v1.32
    updateMode: Manual
  status: |
    currentVersion: v1.33
    phase: ControlPlaneUpdating
    progress: 0%
    controlPlane:
    - name: mazin-master-0
      phase: Updating
      components:
        kube-apiserver: v1.33
        kube-controller-manager: v1.33
        kube-scheduler: v1.33
    - name: mazin-master-1
      phase: Updating
      components:
        kube-apiserver: v1.33
        kube-controller-manager: v1.33
        kube-scheduler: v1.33
    - name: mazin-master-2
      phase: Updating
      components:
        kube-apiserver: v1.33
        kube-controller-manager: v1.33
        kube-scheduler: v1.33
    nodes:
      desiredCount: 6
      upToDateCount: 0
kind: ConfigMap
metadata:
  annotations:
    cause: downgradeK8s
    lastReconciliationTime: "2026-02-02T11:34:42Z"
    lastUpToDateTime: "2026-02-02T11:09:59Z"
  creationTimestamp: "2026-01-16T16:48:45Z"
  labels:
    heritage: deckhouse
    k8s-version: v1.33
    max-k8s-version: v1.33
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "21379847"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Обновление control plane в процессе

Часть master-узлов уже на новой версии, часть ещё обновляется, отображается прогресс в процентах:

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: v1.32
    updateMode: Manual
  status: |
    currentVersion: v1.33
    phase: ControlPlaneUpdating
    progress: 60%
    controlPlane:
    - name: mazin-master-0
      phase: Updating
      components:
        kube-apiserver: v1.33
        kube-controller-manager: v1.33
        kube-scheduler: v1.33
    - name: mazin-master-1
      phase: Updating
      components:
        kube-apiserver: v1.33
        kube-controller-manager: v1.33
        kube-scheduler: v1.33
    - name: mazin-master-2
      phase: UpToDate
      components:
        kube-apiserver: v1.32
        kube-controller-manager: v1.32
        kube-scheduler: v1.32
    nodes:
      desiredCount: 6
      upToDateCount: 6
kind: ConfigMap
metadata:
  annotations:
    cause: downgradeK8s
    lastReconciliationTime: "2026-02-02T11:41:55Z"
    lastUpToDateTime: "2026-02-02T11:09:59Z"
    creationTimestamp: "2026-01-16T16:48:45Z"
  labels:
    heritage: deckhouse
    k8s-version: v1.33
    max-k8s-version: v1.33
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "21388343"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Кластер в актуальном состоянии (2 master-узла, 1 arbitr-узел и 3 worker-узла)

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: v1.33
    updateMode: Manual
  status: |
    currentVersion: v1.33
    phase: UpToDate
    controlPlane:
    - name: mazin-master-0
      phase: UpToDate
      components:
        kube-apiserver: v1.33
        kube-controller-manager: v1.33
        kube-scheduler: v1.33
    - name: mazin-master-1
      phase: UpToDate
      components:
        kube-apiserver: v1.33
        kube-controller-manager: v1.33
        kube-scheduler: v1.33
    nodes:
      desiredCount: 6
      upToDateCount: 6
kind: ConfigMap
metadata:
  annotations:
    cause: upgradeK8s
    lastReconciliationTime: "2026-02-02T11:09:59Z"
    lastUpToDateTime: "2026-02-02T11:09:59Z"
    creationTimestamp: "2026-01-16T16:48:45Z"
  labels:
    heritage: deckhouse
    k8s-version: v1.33
    max-k8s-version: v1.33
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "21357074"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Сбой одного или нескольких компонентов control plane

У master-узла `phase: Failed`, в поле `description` — причина (например, под или контейнер не в состоянии `Running`):

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: v1.32
    updateMode: Manual
  status: |
    currentVersion: v1.32
    phase: ControlPlaneUpdating
    progress: 73%
    controlPlane:
    - name: mazin-master-1
      phase: UpToDate
      components:
        kube-apiserver: v1.32
        kube-controller-manager: v1.32
        kube-scheduler: v1.32
    - name: mazin-master-2
      phase: Updating
      components:
        kube-apiserver: v1.33
        kube-controller-manager: v1.33
        kube-scheduler: v1.33
    - name: mazin-master-0
      phase: Failed
      components:
        kube-apiserver: v1.32
        kube-controller-manager: v1.32
        kube-scheduler: v1.32
    nodes:
      desiredCount: 6
      upToDateCount: 6
kind: ConfigMap
metadata:
```


