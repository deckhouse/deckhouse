---
title: CSI-драйвер (VMware vSphere)
permalink: ru/architecture/cluster-and-infrastructure/infrastructure/csi-vsphere.html
lang: ru
search: csi vsphere, csi-vsphere, container storage interface, vmware vsphere
description: Описание архитектуры CSI-драйвера для VMware vSphere в Deckhouse Kubernetes Platform.
---

Для управления постоянными томами хранения в Deckhouse Kubernetes Platform (DKP) используется CSI-драйвер (плагин).

[Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/master/spec.md) — это стандартный интерфейс, который унифицирует доступ к хранилищам и упрощает интеграцию различных систем хранения в кластеры.

В модуле [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) используется [`csi-vsphere`](/modules/csi-vsphere/) (основан на [vSphere CSI Driver](https://github.com/kubernetes-sigs/vsphere-csi-driver)), который отличается от драйверов, применяемых в других модулях `cloud-provider-*` DKP, наличием компонента syncer, специфичного для VMware vSphere.

## Архитектура драйвера

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура CSI-драйвера [`csi-vsphere`](/modules/csi-vsphere/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура CSI-драйвера csi-vsphere](../../../../images/architecture/cluster-and-infrastructure/c4-l2-csi-driver-vsphere.ru.png)

## Компоненты драйвера

CSI-драйвер состоит из следующих компонентов:

1. **Csi-controller** (Deployment) — Controller Plugin, отвечающий за глобальные операции с томами: создание и удаление, подключение и отключение от узлов, а также управление снимками.

   Состоит из следующих контейнеров:

   * **controller** — основной контейнер, реализующий функциональность CSI-драйвера (capabilities) в виде gRPC-сервисов Identity Service и Controller Service согласно [спецификации CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md#rpc-interface);

   * **сайдкар-контейнеры контроллера** — поддерживаемые сообществом Kubernetes внешние контроллеры (external controllers).

     Они необходимы, поскольку persistent volume controller, запущенный в kube-controller-manager (компонент [control plane кластера DKP](../../kubernetes-and-scheduling/control-plane.html)), не имеет интерфейса взаимодействия с CSI-драйверами. Внешние контроллеры следят за ресурсами PersistentVolumeClaim и вызывают соответствующие функции CSI-драйвера в контейнере controller. Они также выполняют служебные функции, такие как получение информации о плагине и его capabilities или проверка состояния драйвера (liveness probe).

     Внешние контроллеры взаимодействуют c контейнером controller по gRPC через Unix-сокеты.

     В csi-controller входят следующие внешние контроллеры:

     * **provisioner** ([external-provisioner](https://github.com/kubernetes-csi/external-provisioner)) — отслеживает ресурсы PersistentVolumeClaim и вызывает RPC `CreateVolume` или `DeleteVolume`. Также использует RPC `ValidateVolumeCapabilities` для проверки совместимости;

     * **attacher** ([external-attacher](https://github.com/kubernetes-csi/external-attacher)) — отслеживает ресурсы VolumeAttachment после того, как под запланирован на узел, а также подключает и отключает тома через RPC `ControllerPublishVolume` и `ControllerUnpublishVolume`;

     * **resizer** ([external-resizer](https://github.com/kubernetes-csi/external-resizer)) — отслеживает обновления ресурсов PersistentVolumeClaim, расширяет тома с помощью RPC `ControllerExpandVolume`, если пользователь запросил больше дискового пространства для PVC и драйвер поддерживает capability `EXPAND_VOLUME`;

     * **snapshotter** ([external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter)) — работает совместно с модулем [`snapshot-controller`](/modules/snapshot-controller/), следит за ресурсами VolumeSnapshotContent, а также управляет снимками томов через RPC `CreateSnapshot`, `DeleteSnapshot` и `ListSnapshots` (если драйвер это поддерживает);

     * [**livenessprobe**](https://github.com/kubernetes-csi/livenessprobe) — отслеживает состояние CSI-драйвера через RPC `Probe` из Identity Service и предоставляет HTTP-эндпоинт `/healthz`, за которым следит [kubelet](../../kubernetes-and-scheduling/kubelet.html). При неуспешной *livenessProbe* kubelet перезапускает под csi-controller.

     * **syncer** — специфичный для VMware vSphere компонент, который синхронизирует метаданные PersistentVolumes, PersistentVolumeClaims и Pods с данными в компоненте VMware vSphere Cloud Native Storage (CNS). [CNS](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/container-storage-plugin/3-0/getting-started-with-vmware-vsphere-container-storage-plug-in-3-0/vsphere-container-storage-plug-in-concepts.html) — это control plane, который управляет томами различных типов в системе хранения VMware vSphere.

2. **Csi-node** (DaemonSet) — Node Plugin, работающий на всех узлах кластера и отвечающий за локальное монтирование и размонтирование томов.

   > **Внимание.** У плагина есть привилегированный доступ к файловой системе каждого узла. В Linux для этого требуется capability `CAP_SYS_ADMIN`. Это необходимо для выполнения операций монтирования и работы с блочными устройствами.

   Состоит из следующих контейнеров:

   * **node** — основной контейнер, реализующий функции CSI-драйвера в виде gRPC-сервисов Identity Service и Node Service согласно [спецификации CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md#rpc-interface);

   * **node-driver-registrar** — сайдкар-контейнер, регистрирующий Node Plugin в [kubelet](../../kubernetes-and-scheduling/kubelet.html). Вызывает в контейнере node RPC `GetPluginInfo` и `NodeGetInfo`, чтобы получить информацию о плагине и узле. Взаимодействуют c контейнером **node** по gRPC через Unix-сокет.

## Взаимодействия драйвера

Драйвер взаимодействует со следующими компонентами:

1. **Kube-apiserver** — мониторинг ресурсов PersistentVolumeClaim, VolumeAttachment и VolumeSnapshotContent.

2. **VMware vSphere**:

   * создание и удаление томов;
   * подключение и отключение томов от узлов;
   * управление снимками;
   * отправка и синхронизация метаданных PersistentVolumes и PersistentVolumeClaims в CNS.

С драйвером взаимодействуют следующие внешние компоненты:

1. [Kubelet](../../kubernetes-and-scheduling/kubelet.html):

   * проверяет livenessProbe CSI-драйвера;
   * регистрирует Node Plugin;
   * вызывает RPC `NodeStageVolume`, `NodeUnstageVolume`, `NodePublishVolume`, `NodeUnpublishVolume` и `NodeExpandVolume` в Node Plugin.

   [Kubelet](../../kubernetes-and-scheduling/kubelet.html) взаимодействует с Node Plugin по gRPC через Unix-сокет.
