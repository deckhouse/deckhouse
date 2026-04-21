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

## Мониторинг процесса обновления Kubernetes

Модуль [`control-plane-manager`](/modules/control-plane-manager/) включает компонент `update-observer`, который предоставляет актуальную информацию о процессе обновления Kubernetes в кластере.

Компонент `update-observer`:

- Читает конфигурацию кластера из секрета `d8-cluster-configuration`;
- Отслеживает версии kubelet на всех узлах через `nodeInfo.kubeletVersion`;
- Собирает версии всех экземпляров control plane по аннотации `control-plane-manager.deckhouse.io/kubernetes-version`;
- Создаёт и обновляет ConfigMap **`d8-cluster-kubernetes`** в пространстве имён `kube-system` с подробным статусом обновления.

В ConfigMap `d8-cluster-kubernetes` отображаются:

- **Статус компонентов** — версии компонентов control plane (kube-apiserver, kube-scheduler, kube-controller-manager) на каждом master-узле;
- **Прогресс обновления узлов** — сколько узлов уже обновлено и сколько всего узлов должно быть обновлено;
- **Целевая и текущая версии** — желаемая версия из конфигурации и фактическое состояние кластера во время обновления;
- **Расхождение версий** — информация о компонентах, работающих не на целевой версии (в том числе на версии выше целевой);
- **Списки версий** — `supportedVersions` (минорные версии Kubernetes, поддерживаемые в текущем релизе Deckhouse); `availableVersions` (версии, доступные для выбора при обновлении или понижении в *данном* кластере; набор ограничен максимальной когда-либо установленной минорной версией и правилом одного минорного шага при откате); `automaticVersion` (минорная версия, которая будет использована при режиме обновления Automatic).

В фазе `ControlPlaneUpdating` поле `status.progress` отражает общий прогресс обновления с учётом промежуточных минорных версий. При многошаговом обновлении (например, 1.33 → 1.35) процент растёт по мере завершения каждого шага, а не только когда все компоненты control plane достигнут финальной целевой версии.

Минорные версии в ConfigMap (`spec`, `status`, а также метки `k8s-version` и `max-k8s-version`) задаются в том же формате, что и в ClusterConfiguration — без префикса `v` (например, `"1.33"`).

Это позволяет в реальном времени видеть, какие компоненты обновляются, на каком этапе находится процесс, и не остановилось ли обновление на каком-либо узле или компоненте.

Для просмотра статуса обновления выполните команду:

```shell
d8 k get configmap d8-cluster-kubernetes -n kube-system -o yaml
```

### Примеры содержимого ConfigMap

В `data.spec` и `data.status` хранится YAML с полем `spec` (целевая версия и режим обновления) и полем `status` (текущее состояние). Ниже приведены примеры для разных сценариев.

#### Кластер в актуальном состоянии (3 master-узла, 3 worker-узла)

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.32"
    updateMode: Manual
  status: |
    currentVersion: "1.32"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: UpToDate
    controlPlane:
    - name: mazin-master-1
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    - name: mazin-master-2
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    - name: mazin-master-0
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
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
    k8s-version: "1.32"
    max-k8s-version: "1.33"
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
    desiredVersion: "1.32"
    updateMode: Manual
  status: |
    currentVersion: "1.33"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: ControlPlaneUpdating
    progress: 0%
    controlPlane:
    - name: mazin-master-0
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-1
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-2
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
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
    k8s-version: "1.33"
    max-k8s-version: "1.33"
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "21379847"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Обновление control plane в процессе

Часть master-узлов уже на новой версии, часть ещё обновляется. В `status.progress` отображается прогресс в процентах.

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.32"
    updateMode: Manual
  status: |
    currentVersion: "1.33"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: ControlPlaneUpdating
    progress: 60%
    controlPlane:
    - name: mazin-master-0
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-1
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-2
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
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
    k8s-version: "1.33"
    max-k8s-version: "1.33"
  name: d8-cluster-kubernetes
  namespace: kube-system
  resourceVersion: "21388343"
  uid: ba981996-f737-469c-9ce1-53aa46135994
```

#### Промежуточный шаг многошагового обновления (например, 1.33 → 1.35)

В конфигурации целевая версия может быть на несколько миноров впереди текущей минорной версии кластера. Поле `status.currentVersion` отражает активный минорный шаг, при этом отдельные компоненты на время могут работать на разных минорах внутри шага. Поле `progress` учитывает весь путь, включая промежуточные миноры, поэтому может быть заметно больше 0% до того, как все компоненты достигнут финальной цели.

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.35"
    updateMode: Manual
  status: |
    currentVersion: "1.34"
    supportedVersions:
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    - "1.35"
    availableVersions:
    - "1.33"
    - "1.34"
    - "1.35"
    automaticVersion: "1.33"
    phase: ControlPlaneUpdating
    progress: 60%
    controlPlane:
    - name: cluster-master-0
      phase: Updating
      components:
        kube-apiserver: "1.35"
        kube-controller-manager: "1.34"
        kube-scheduler: "1.34"
    nodes:
      desiredCount: 6
      upToDateCount: 0
kind: ConfigMap
metadata:
  annotations:
    cause: upgradeK8s
  labels:
    heritage: deckhouse
    k8s-version: "1.33"
    max-k8s-version: "1.33"
  name: d8-cluster-kubernetes
  namespace: kube-system
```

#### Кластер в актуальном состоянии (2 master-узла, 1 arbitr-узел и 3 worker-узла)

```yaml
apiVersion: v1
data:
  spec: |
    desiredVersion: "1.33"
    updateMode: Manual
  status: |
    currentVersion: "1.33"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: UpToDate
    controlPlane:
    - name: mazin-master-0
      phase: UpToDate
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-1
      phase: UpToDate
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
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
    k8s-version: "1.33"
    max-k8s-version: "1.33"
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
    desiredVersion: "1.32"
    updateMode: Manual
  status: |
    currentVersion: "1.32"
    supportedVersions:
    - "1.30"
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    availableVersions:
    - "1.31"
    - "1.32"
    - "1.33"
    - "1.34"
    automaticVersion: "1.33"
    phase: ControlPlaneUpdating
    progress: 73%
    controlPlane:
    - name: mazin-master-1
      phase: UpToDate
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    - name: mazin-master-2
      phase: Updating
      components:
        kube-apiserver: "1.33"
        kube-controller-manager: "1.33"
        kube-scheduler: "1.33"
    - name: mazin-master-0
      phase: Failed
      components:
        kube-apiserver: "1.32"
        kube-controller-manager: "1.32"
        kube-scheduler: "1.32"
    nodes:
      desiredCount: 6
      upToDateCount: 6
kind: ConfigMap
metadata:
```
