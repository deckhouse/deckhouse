---
title: "Управление узлами"
permalink: ru/admin/configuration/platform-scaling/node-management.html
lang: ru
---

## Общее описание

Управление узлами в Deckhouse Kubernetes Platform (DKP) происходит через модуль `node-manager`. Он позволяет группировать узлы (NodeGroup), автоматизировать установку и обновление системного ПО (containerd, kubelet, ОС), подключать их к кластеру, масштабировать, а также настраивать балансировку и мониторинг.

Основные функции:

1. Управление несколькими узлами как связанной **группой (NodeGroup)**:  
   - Передача единого набора метаданных всем узлам группы (лейблы, тейнты, аннотации и т.д.).  
   - Мониторинг группы узлов как одной сущности, группировка алертов, возможность Chaos Monkey для проверки отказоустойчивости.
1. Установка, обновление и настройка ПО узла (containerd, kubelet и т.д.) с учётом особенностей ОС и окружения.
1. **Управление обновлениями узлов и их простоем (disruptions)**:  
   - Разрешается или запрещается обновлять узлы раньше Control Plane.  
   - Поддерживаются «обычные» обновления (всегда автоматически) и «требующие disruption» (нужен drain или ручное подтверждение).  
   - Ведётся контроль прогресса с помощью встроенных метрик.  
1. **Масштабирование кластера**:  
   - Автоматическое (при поддерживаемых облаках) или поддержание фиксированного числа узлов.  
   - Автоматическое масштабирование возможно благодаря  machine-controller-manager (MCM), что позволяет при дефиците ресурсов добавлять узлы, а при простое — удалять.
1. Управление Linux-пользователями на узлах.

## Включение node-manager

Включается/выключается через CR `ModuleConfig/node-manager` или командой:

```shell
kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable node-manager
# или disable
```

Пример включения модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: node-manager
spec:
  version: 2
  enabled: true
  settings:
    earlyOomEnabled: true
    instancePrefix: kube
    mcmEmergencyBrake: false
```

### Типы узлов

В одной группе (NodeGroup) могут быть:

- CloudEphemeral (автоматически заказываются/удаляются в облаке),
- CloudPermanent (настраиваются из <Provider>ClusterConfiguration и требуют `dhctl converge`),
- CloudStatic (статический узел в облачном окружении, управляется cloud-controller-manager),
- Static (узел на bare metal или ВМ, не управляемый облаком). Это позволяет строить гибридные кластеры, часть узлов в облаке, часть — на физическом «железе».

### Автоматическое развёртывание и обновление.

Deckhouse (node-manager) выполняет операции:

- Настройка/оптимизация ОС (ставит нужные пакеты, параметры ядра).
- Установка containerd, kubelet, добавление узла в кластер.
- Настройка Nginx для балансировки запросов узла к kube-apiserver.
- Поддержка актуального состояния (обычные или disruptive-обновления).
- Disruptions позволяют при необходимости вручную подтверждать обновление узла, если требуется drain и перезагрузка.

Узлы в группе получают одинаковые параметры. NodeGroup можно использовать для любых сценариев — master, приложения, сервисные узлы, и т. д. Возможно сочетание «гибридных» кластеров (часть узлов cloud, часть static). Для автоматизации работы со статикой поддерживается Cluster API Provider Static (CAPS).

## Добавление узлов в bare-metal-кластер

Перед добавлением необходимо:

1. Установить ОС.
1. Убедиться в сетевой связности с master-узлами.
1. В настройках кластера (StaticClusterConfiguration) прописать подсети.

### Ручной способ

1. Создайте NodeGroup с `nodeType: Static`, например:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
   ``` 

1. В Deckhouse автоматически появится скрипт bootstrap.sh (сохраняется в секрете manual-bootstrap-for-<NodeGroup>).

1. Загрузите этот скрипт на целевой сервер и выполните от root. В результате узел добавится в кластер.

### Автоматический способ

1. Создайте SSHCredentials c приватным ключом, пользователем и паролем/портом для sudo.
1. Создайте объекты StaticInstance для каждого сервера (указывая IP и ссылаясь на SSHCredentials).
1. В NodeGroup секции `spec.staticInstances` укажите `count` и `labelSelector`, который будет совпадать с лейблами нужных StaticInstance.
1. Как только появляется подходящий StaticInstance, Deckhouse автоматически по SSH «забирает» сервер, выполняет на нём `bootstrap.sh` и добавляет узел в группу.

## Добавление узлов в cloud-кластер

В cloud-кластер возможно добавить три типа узлов:

- Static / CloudStatic (вручную),
- CloudPermanent (через <PROVIDER>ClusterConfiguration + dhctl),
- CloudEphemeral (через Machine Controller Manager / NodeGroup).

### Механика добавления узлов в кластер

При добавлении узлов любого типа используется один и тот же подход — на целевой машине выполняется скрипт `bootstrap.sh`. Способы его доставки:

- CloudEphemeral / CloudPermanent: добавляется в cloud-init.
- Static: вручную (ssh + script) или автоматически (CAPS).

Скрипт определяет ОС, контактирует с bashible API, получает скрипты настройки, и устанавливает systemd-сервис bashible, который проверяет апдейты конфигурации каждые 4 часа (или по контрольной сумме).

### Добавление CloudPermanent-узлов в cloud-кластер

- <PROVIDER>ClusterConfiguration через dhctl config edit provider-cluster-configuration.
- Укажите нужные параметры (flavor, imageName, replicas, т. д.).
- Примените dhctl converge — DKP закажет/удалит машины, настроит bootstrap. Узлы добавятся в кластер. 

### Добавление CloudEphemeral-узлов в cloud-кластер

1. Создайте NodeGroup с nodeType: CloudEphemeral, укажите `cloudInstances` (`classReference` на `InstanceClass`, `zones`, `minPerZone`, `maxPerZone` и т. д.).
1. MCM будет заказывать нужное число узлов, выполнять bootstrap, и при уменьшении нагрузки — удалять их.

## Конфигурация группы узлов

Ниже описаны базовые параметры NodeGroup, разделённые на категории: общие (CRI, kubelet, disruptions, update, nodeTemplate) и специфические для static/cloud.

### Общие настройки

1. CRI: можно указать cri.type: Containerd (настраивает встроенный containerd) или NotManaged (CRI устанавливается отдельно).
1. kubelet: параметры вроде maxPods, containerLogMaxSize, rootDir.
1. disruptions: управляет «простаями» узлов. Можно включить автоматический drain при обновлениях или задать окно обслуживания.
1. update: указывает, сколько узлов одновременно можно обновлять (maxConcurrent).
1. nodeTemplate: лейблы, аннотации, taints для всех узлов группы.

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: example
   spec:
     cri:
       type: Containerd
       containerd:
         maxConcurrentDownloads: 15
     kubelet:
       maxPods: 110
     disruptions:
       approvalMode: RollingUpdate
     update:
       maxConcurrent: 1
     nodeTemplate:
       labels:
         role: example
   ```

### Настройки для групп с узлами Static и CloudStatic

В `spec.staticInstances` указывают `count` и `labelSelector`, чтобы автоматизировать работу с CAPS (Cluster API Provider Static). Если узлы добавляются вручную, эта секция может отсутствовать.

### Настройки для групп с узлами CloudEphemeral

В spec.cloudInstances указываются:

1. classReference (InstanceClass),
1. zones (список зон),
1. minPerZone, maxPerZone для автоматического масштабирования,
1. standby / standbyHolder (резервные узлы),
1. priority (когда несколько групп для автомасштабирования), quickShutdown и т. д.

## Автомасштабирование группы узлов

Рассмотрим типичный пример:

- Группа CloudEphemeral: minPerZone: 1, maxPerZone: 5.
- Deployment запрашивает ~1500m CPU, 5Gi RAM.
Cluster Autoscaler при Pending-подах добавит узлы. При снижении нагрузки (лишние узлы) — удалит узлы, если в течение 15 минут они не понадобятся.

Обратите внимание:

- Автомасштабирование учитывает только запрос ресурсов (resources.requests), а не текущую утилизацию.
- Приоритет NodeGroup (priority) определяет, в какую группу Cluster Autoscaler будет масштабироваться сначала.