---
title: Вспомогательные компоненты
permalink: ru/architecture/virtualization/auxiliary.html
lang: ru
search: virtualization-audit, virtualization-dra, dra
description: Архитектура вспомогательных компонентов модуля virtualization в Deckhouse Kubernetes Platform.
---

В модуле [`virtualization`](/modules/virtualization/) используются компоненты, реализующие следующие вспомогательные функции:

- аудит событий безопасности;
- проброс USB-устройств в виртуальные машины;
- обновление сетевых маршрутов;
- удаление ресурсов перед деактивацией модуля [`virtualization`](/modules/virtualization/).

## Аудит событий безопасности

С инструкцией по активации аудита событий безопасности модуля [`virtualization`](/modules/virtualization/) можно ознакомиться в [документации модуля](/modules/virtualization/stable/admin_guide.html#%D0%BE%D0%BF%D0%B8%D1%81%D0%B0%D0%BD%D0%B8%D0%B5-%D0%BF%D0%B0%D1%80%D0%B0%D0%BC%D0%B5%D1%82%D1%80%D0%BE%D0%B2).

### Архитектура

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

- На схеме контейнеры разных подов показаны как взаимодействующие напрямую. Фактически обмен выполняется через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса приводится над стрелкой.
- Поды могут быть запущены в нескольких репликах, однако на схеме каждый под показан в единственном экземпляре.
{% endalert %}

Архитектура компонентов, реализующих аудит событий безопасности модуля [`virtualization`](/modules/virtualization/) на уровне 2 модели C4 изображена на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура компонента virtualization-audit модуля virtualization](../../../images/architecture/virtualization/c4-l2-virtualization-audit.ru.png)

### Компоненты

Аудит событий безопасности реализован одним компонентом:

- **Virtualization-audit** — компонент, состоящий из одного контейнера и принимающий поток событий безопасности модуля [`virtualization`](/modules/virtualization/). Отправка событий реализована с использованием модуля [`log-shipper`](/modules/log-shipper/). Агент логирования vector согласно настройкам в кастомных ресурсах ClusterLoggingConfig  отбирает из аудит-лога кластера события, связанные с кастомными ресурсами модуля [`virtualization`](/modules/virtualization/) и отправляет их на эндпойнт сервиса virtualization-audit. Virtualization-audit обрабатывает полученные audit-события, обогащает их данными из Kubernetes API и сохраняет обработанные события в собственный лог.

Можно перенаправить события безопасности в систему логирования кластера (например, Loki). В этом случае аналогичным образом используются ресурсы ClusterLoggingConfig и агент vector модуля [`log-shipper`](/modules/log-shipper/).

### Взаимодействия

Virtualization-audit взаимодействует со следующими компонентами:

1. **Kube-apiserver** — cледит за изменениями кастомных ресурсов модуля [`virtualization`](/modules/virtualization/).

С virtualization-audit взаимодействуют следующие внешние компоненты:

1. **Log-shipper-agent**:

   - отправляет события безопасности модуля [`virtualization`](/modules/virtualization/);
   - cобирает обработанные аудит-логи.

## Virtualization-DRA и прочие компоненты

### Архитектура

Архитектура прочих вспомогательных компонентов модуля [`virtualization`](/modules/virtualization/) на уровне 2 модели C4 изображена на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура прочих вспомогательных компонентов модуля virtualization](../../../images/architecture/virtualization/c4-l2-virtualization-misc.ru.png)

### Компоненты

1. **Virtualization-dra** (DaemonSet) — драйвер DRA, с помощью которого реализуется проброс USB-устройств в виртуальные машины. Для проброса USB-устройств используется технология [DRA (Dynamic Resource Allocation)](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/). DRA — это механизм Kubernetes API, scheduler и kubelet для описания, планирования и подготовки динамически выделяемых ресурсов через внешние драйверы. Драйвер DRA выполняет следующие операции:

   - автоматически обнаруживает USB-устройства на узлах кластера, публикует их в Kubernetes DRA API как ResourceSlice и создаёт ресурсы NodeUSBDevice. Администратор назначает пространство имен ресурсу NodeUSBDevice, установив поле `.spec.assignedNamespace`. Это делает устройство доступным в этом пространстве имен. Компонент [virtualization-controller](controller.html) модуля [`virtualization`](/modules/virtualization/) автоматически создаёт соответствующий ресурс USBDevice в этом пространстве имен, который дальше используется для настройки проброса USB-устройств. Подробнее с настройкой проброса USB-устройств можно ознакомиться в [документации модуля](/modules/virtualization/stable/user_guide.html#usb-%D1%83%D1%81%D1%82%D1%80%D0%BE%D0%B9%D1%81%D1%82%D0%B2%D0%B0);

   - автоматически обнаруживает USB-устройства на узлах кластера, публикует их в Kubernetes DRA API как ResourceSlice. Virtualization-controller синхронизирует эти данные в кастомные ресурсы NodeUSBDevice, которые дальше используются для настройки проброса USB-устройств. Подробнее с настройкой проброса USB-устройств можно ознакомиться в [документации модуля](/modules/virtualization/stable/user_guide.html#usb-%D1%83%D1%81%D1%82%D1%80%D0%BE%D0%B9%D1%81%D1%82%D0%B2%D0%B0);

   - регистрируется в [kubelet](../kubernetes-and-scheduling/kubelet.html) как DRA kubelet plugin. DRA kubelet plugin подготавливает и освобождает выделенные ресурсы для подов через операции PrepareResourceClaims и UnprepareResourceClaims. Метод PrepareResourceClaims возвращает ID устройств CDI (Container Device Interface), которые kubelet передает в containerd. Данные о доступных USB-устройствах публикуются через ResourceSlice, а выбор устройств выполняется механизмом Kubernetes DRA на основе ResourceClaim/ResourceClaimTemplate и DeviceClass.
   
     DRA-драйвер взаимодействует с kubelet по протоколу gRPC через Unix-сокеты.

   - реализует USBIP-сервер, благодаря чему USB-устройство автоматически по сети пробрасывается на узел, где запущена виртуальная машина. Нет необходимости вручную размещать ВМ на том же узле, где находится устройство.

   Cостоит из следующих контейнеров:

   - **init-load** — init-контейнер, загружающий модули ядра Linux, необходимые для работы DRA-драйвера;
   - **virtualization-dra** — основной контейнер.

1. **Vm-route-forge** — контроллер, следящий за кастомными ресурсами VirtualMachines из `virtualization.deckhouse.io` API group и обновляющий сетевые маршруты на узле через Linux netlink/eBPF в таблицах маршрутизации, используемых [CNI Cilium](/modules/cni-cilium/) для маршрутизации трафика между ВМ.

1. **Pre-delete-hook** (Job) — задание, запускаемое контроллером Deckhouse перед удалением модуля [`virtualization`](/modules/virtualization/), которое удаляет кастомные ресурсы InternalVirtualizationKubeVirt и InternalVirtualizationCDI с именем `config`.

### Взаимодействия

Virtualization-dra взаимодействует со следующими компонентами:

1. **Kubelet** — регистрируется в kubelet как DRA kubelet plugin.

Vm-route-forge взаимодействует со следующими компонентами:

1. **Kube-apiserver** — получает события по ресурсам VirtualMachines, CiliumNode и Node.
1. **Сеть хоста/ядро Linux** — обновляет маршруты и правила маршрутизации на узле.
1. **Cilium data plane** — использует данные CiliumNode и таблиц маршрутизации Cilium для маршрутизации трафика ВМ.

Pre-delete-hook взаимодействует со следующими компонентами:

1. **Kube-apiserver** — удаляет ресурсы InternalVirtualizationKubeVirt и InternalVirtualizationCDI с именем `config`.

С Virtualization-dra взаимодействуют следующие внешние компоненты:

1. **Kubelet** — вызывает gRPC-методы PrepareResourceClaims и UnprepareResourceClaims для подготовки и освобождения ресурсов, связанных с USB-устройствами.
