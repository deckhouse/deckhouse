---
title: "Управление узлами"
permalink: ru/admin/configuration/platform-scaling/node.html
lang: ru
---

## Общее описание

Deckhouse Kubernetes Platform (DKP) поддерживает полный цикл управления узлами:

- Автоматическое масштабирование количества узлов в зависимости от нагрузки;
- Обновление узлов и поддержание их в актуальном состоянии;
- Централизованное управление конфигурацией групп узлов через CRD NodeGroup;
- Использование различных типов узлов: постоянные, временные, облачные или bare-metal.

> DKP может работать как с bare-metal, так и с облачными кластерами, обеспечивая гибкость и расширяемость.

Группы узлов позволяют логически сегментировать инфраструктуру кластера. В Deckhouse часто используются следующие типы NodeGroup по назначению:

- `master` — управляющие узлы (Control Plane);
- `front` — узлы для маршрутизации HTTP(S)-трафика;
- `monitoring` — узлы для размещения компонентов мониторинга;
- `worker` — узлы для пользовательских приложений;
- `system` — выделенные узлы для системных компонентов.

В каждой группе можно централизованно задавать настройки узлов, включая версию Kubernetes, ресурсы, taint'ы, лейблы, параметры kubelet и прочее.

## Включение механизма управления узлами

Управление узлами реализовано с помощью модуля `node-manager`, который можно включить или выключить несколькими способами:

1. Через ресурс ModuleConfig/node-manager:

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

1. Командой:

   ```console
   d8 platform module enable node-manager
   # или disable
   ```

1. Через [веб-интерфейс Deckhouse](https://deckhouse.ru/products/kubernetes-platform/modules/console/stable/):

   - Перейдите в раздел «Deckhouse - «Модули»;
   - Найдите модуль `node-manager` и нажмите на него;
   - Включите тумблер «Модуль включен».

## Автоматическое развёртывание и обновление

В Deckhouse Kubernetes Platform (DKP) реализован автоматизированный механизм управления жизненным циклом узлов на основе объектов NodeGroup. DKP обеспечивает как начальное развёртывание узлов, так и их обновление при изменении конфигурации, поддерживая как облачные, так и bare-metal кластеры (при наличии интеграции с модулем `node-manager`).

Как это работает:

1. NodeGroup — основной объект управления группами узлов. Он определяет тип узлов, их количество, шаблоны ресурсов и ключевые параметры (например, настройки kubelet, taint'ов и др.).
1. При создании или изменении NodeGroup, модуль `node-manager` автоматически приводит состояние узлов в соответствие с заданной конфигурацией.
1. Обновление происходит без вмешательства пользователя — устаревшие узлы удаляются, новые создаются автоматически.

Пример: автоматическое обновление версии kubelet.

1. Пользователь изменяет параметры в секции kubelet объекта NodeGroup.
1. DKP определяет, что текущие узлы не соответствуют новой конфигурации.
1. Последовательно создаются новые узлы с обновлёнными настройками.
1. Старые узлы постепенно удаляются из кластера.

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker-cloud
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: AnotherCloudInstanceClass
         name: my-class
   ```

## Базовая настройка узлов и операционной системы

При создании и подключении узлов Deckhouse автоматически выполняет ряд действий, необходимых для корректной работы кластера:

- установка и настройка поддерживаемой операционной системы;
- отключение автоматических обновлений пакетов;
- настройка журналирования и системных параметров;
- установка необходимых пакетов и утилит;
- настройка компонента `nginx` для балансировки трафика от `kubelet` к API-серверам;
- установка и конфигурация компонентов container runtime (`containerd`) и `kubelet`;
- включение узла в состав кластера Kubernetes.

Эти операции выполняются автоматически при использовании `bootstrap.sh` или при подключении узлов через ресурсы StaticInstance и SSHCredentials.

### Обновления, требующие прерывания работы узла

Некоторые обновления, например, обновление версии `containerd` или обновление kubelet на несколько версий вперед,
требуют прерывания работы узла
и могут привести к кратковременному простою системных компонентов (*disruptive-обновления*).
Режим применения таких обновлений настраивается с помощью [параметра `disruptions.approvalMode`](../../../reference/cr/nodegroup/#nodegroup-v1-spec-disruptions-approvalmode):

- `Manual` — режим ручного подтверждения disruptive-обновлений.
  При появлении доступного disruptive-обновления отображается специальный алерт.
  
  Чтобы подтвердить disruptive-обновление,
  установите аннотацию `update.node.deckhouse.io/disruption-approved=` на каждый узел в группе, следуя примеру:

  ```shell
  sudo -i d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
  ```

  > **Важно**. В этом режиме не выполняется автоматический drain узла.
  > При необходимости выполните drain вручную перед установкой аннотации.
  >
  > Чтобы избежать проблем при выполнении drain,
  > всегда устанавливайте режим `Manual` для группы master-узлов.

- `Automatic` — режим автоматического разрешения disruptive-обновлений.
  
  В этом режиме по умолчанию выполняется автоматический drain узла перед применением обновления.
  Поведение можно изменить с помощью [параметра `disruptions.automatic.drainBeforeApproval`](../../../reference/cr/nodegroup/#nodegroup-v1-spec-disruptions-automatic-drainbeforeapproval) в настройках узла.

- `RollingUpdate` — режим, при котором будет создан новый узел с обновлёнными настройками, а старый будет удалён.
  Применим только к облачным узлам.

  В этом режиме на время обновления в кластере создаётся дополнительный узел.
  Это может быть удобно, если в кластере нет ресурсов для временного размещения нагрузки с обновляемого узла.

## Типы узлов и механика добавления

В Deckhouse узлы разделяются на следующие типы:

- Static — управляются вручную, `node-manager` их не масштабирует и не пересоздаёт;
- CloudStatic — создаются вручную или любыми внешними инструментами, размещается в том же облаке, с которым настроена интеграция у одного из облачных провайдеров:
  - Узлы типа CloudStatic обладают рядом особенностей, связанных с интеграцией с облачным провайдером. Такие узлы управляются компонентом cloud-controller-manager, в результате чего:
    - в объект Node автоматически добавляются метаданные о зоне и регионе размещения;
    - при удалении виртуальной машины из облака, соответствующий объект Node также будет удалён из кластера;
    - доступна работа CSI-драйвера для подключения облачных дисков.
- CloudPermanent — постоянные узлы, создаваемые и обновляемые `node-manager`;
- CloudEphemeral — временные узлы, создаваемые и масштабируемые в зависимости от нагрузки.

Узлы добавляются в кластер путём создания объекта NodeGroup, который описывает тип, параметры и конфигурацию группы узлов. В случае CloudEphemeral-групп DKP интерпретирует этот объект и автоматически создаёт соответствующие узлы, регистрируя их в Kubernetes-кластере. Для других типов NodeGroup (например, CloudPermanent или Static) создание и регистрация узлов должны быть выполнены вручную или внешними инструментами.

Также поддерживается сценарий гибридных групп, где одна NodeGroup может включать как облачные, так и статические узлы. Например, основную нагрузку могут нести серверы bare-metal, а облачные инстансы использоваться как масштабируемое дополнение при пиковых нагрузках.

## Добавление узлов в bare-metal-кластер

### Ручной способ

1. Включите модуль `node-manager`.

1. Создайте объект NodeGroup с типом `Static`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
   ```

   В спецификации этого ресурса укажем тип узлов `Static`. Для всех объектов NodeGroup в кластере автоматически будет создан скрипт `bootstrap.sh`, с помощью которого узлы добавляются в группы. Когда узлы добавляются вручную, необходимо скопировать этот скрипт на сервер и выполнить.

   Скрипт можно получить в веб-интерфейсе Deckhouse на вкладке «Группы узлов → Скрипты» или командой kubectl:

   ```console
   kubectl -n d8-cloud-instance-manager get secrets manual-bootstrap-for-worker -ojsonpath="{.data.bootstrap\.sh}"
   ```

   Скрипт нужно раскодировать из Base64, а затем выполнить от `root`.

1. Когда скрипт выполнится, сервер добавится в кластер в качестве узла той группы, для которой был использован скрипт.

### Автоматический способ

В DKP возможно автоматическое добавление физических (bare-metal) серверов в кластер без ручного запуска установочного скрипта на каждом узле. Для этого необходимо:

1. Подготовить сервер (ОС, сеть):
   - Установить поддерживаемую ОС;
   - Настроить сеть и убедиться, что сервер доступен по SSH;
   - Создать системного пользователя (например, ubuntu), от имени которого будет выполняться подключение по SSH;
   - Убедиться, что пользователь может выполнять команды через `sudo`.

1. Создать объект `SSHCredentials` с доступом к серверу. DKP использует объект `SSHCredentials` для подключения к серверам по SSH. В нём указывается:
   - Приватный ключ;
   - Пользователь ОС;
   - Порт SSH;
   - (опционально) пароль для `sudo`, если требуется.

   Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: SSHCredentials
   metadata:
     name: static-nodes
   spec:
     privateSSHKey: |
       -----BEGIN OPENSSH PRIVATE KEY-----
       LS0tLS1CRUdJlhrdG...................VZLS0tLS0K
       -----END OPENSSH PRIVATE KEY-----
     sshPort: 22
     sudoPassword: password
     user: ubuntu
   ```

   > **Важно**. Приватный ключ должен соответствовать открытому ключу, добавленному в `~/.ssh/authorized_keys` на сервере.

1. Создать объект StaticInstance для каждого сервера:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-0
     labels:
       static-node: auto
   spec:
     address: 192.168.1.10
     credentialsRef:
       apiVersion: deckhouse.io/v1alpha1
       kind: SSHCredentials
       name: static-nodes
   ```

   Под каждый сервер необходимо создавать отдельный ресурс StaticInstance, но можно использовать одни и те же `SSHCredentials` для доступа на разные серверы.

   Возможные состояния ресурсов StaticInstance:

   - `Pending` — сервер ещё не настроен, в кластере отсутствует соответствующий узел.
   - `Bootstrapping` — выполняется настройка сервера и подключение узла в кластер.
   - `Running` — сервер успешно настроен, узел подключён к кластеру.
   - `Cleaning` — выполняется очистка сервера и удаление узла из кластера.

     Эти состояния отображают текущий этап управления узлом. CAPS автоматически переводит StaticInstance между этими состояниями в зависимости от необходимости добавить или удалить узел из группы.

1. Создать NodeGroup с описанием, как DKP будет использовать эти серверы:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 3
       labelSelector:
         matchLabels:
           static-node: auto
     nodeTemplate:
       labels:
         node-role.deckhouse.io/worker: ""
   ```

   Здесь добавляются параметры, которые описывают использование StaticInstances: `count` указывает, сколько узлов будет добавлено в эту группу; в `labelSelector` прописываются правила для создания выборки узлов.

   При использовании Cluster API Provider Static (CAPS) важно правильно задать параметры `nodeType`: `Static` и секцию `staticInstances` в объекте NodeGroup:
   - Если параметр labelSelector не задан, CAPS будет использовать любые ресурсы StaticInstance, доступные в кластере.
   - Один и тот же StaticInstance может быть использован в разных группах узлов, если соответствует фильтрам.
   - CAPS будет автоматически поддерживать количество узлов в группе, заданное в параметре count.
   - При удалении узла CAPS выполнит его очистку и отключение, а соответствующий StaticInstance перейдёт в статус Pending и может быть использован повторно.

После того как группа узлов будет создана, появится скрипт для добавления серверов в эту группу. DKP будет ждать, пока в кластере появится необходимое количество объектов StaticInstance, которые подходят под выборку по лейблам. Как только такой объект появится, DKP получит из созданных ранее манифестов IP-адрес сервера и параметры для подключения по SSH, подключится к серверу и выполнит на нём скрипт `bootstrap.sh`. После этого сервер добавится в заданную группу в качестве узла.

## Добавление узлов в облачном кластере

### Добавление CloudPermanent-узлов в облачном кластере

Чтобы добавить узлы типа `CloudPermanent` в облачный кластер DKP:

1. Убедитесь, что включён модуль облачного провайдера. Например, `cloud-provider-aws`, `cloud-provider-openstack`, `cloud-provider-yandex` и др.

   Это можно проверить с помощью команды:

   ```console
   kubectl -n d8-system get modules
   ```

   Или посмотреть в веб-интерфейсе Deckhouse.

1. Создайте объект NodeGroup с типом `CloudPermanent`. Узлы типа `CloudPermanent` управляются через Terraform, встроенный в DKP. Конфигурация таких узлов находится в объекте `(Provider)ClusterConfiguration`. Редактировать его нужно с помощью утилиты `dhctl` в установочном контейнере. Пример:

   ```yaml
   nodeGroups:
   - name: cloud-permanent
     replicas: 2
     instanceClass:
       flavorName: m1.large
       imageName: ubuntu-22-04-cloud-amd64
       rootDiskSize: 20
       mainNetwork: default
     volumeTypeMap:
       nova: ceph-ssd
   ```

1. Укажите параметры шаблона инстанса. Поля внутри `instanceClass` зависят от конкретного облачного провайдера. Ниже приведён пример для OpenStack:
   - `flavorName` — тип инстанса (ресурсы: CPU, RAM);
   - `imageName` — образ ОС;
   - `rootDiskSize` — размер системного диска (в ГБ);
   - `mainNetwork` — имя сети;
   - при необходимости: диск ETCD, зоны, volume types и т.д.

     Для других облаков названия и структура параметров могут отличаться. Актуальные поля можно посмотреть в описании CRD или в документации по соответствующему облачному провайдеру.

1. Примените конфигурацию с помощью `dhctl converge`. После редактирования `(Provider)ClusterConfiguration` выполните:

   ```console
   dhctl converge \
     --ssh-host <IP мастер-узла> \
     --ssh-user <имя пользователя> \
     --ssh-agent-private-keys /tmp/.ssh/<ключ>
   ```

   Эта команда:
   - запустит Terraform,
   - создаст нужные виртуальные машины,
   - выполнит на них установку DKP (через `bootstrap.sh`),
   - зарегистрирует узлы в кластере.

1. Готово — новые узлы появятся в кластере автоматически. Их можно увидеть выполнив команду:

   ```console
   kubectl get nodes
   ```

   Либо в веб-интерфейсе Deckhouse.

Deckhouse Kubernetes Platform может работать поверх сервисов Managed Kubernetes (например, GKE и EKS). При этом модуль `node-manager` обеспечивает управление конфигурацией и автоматизацию действий с узлами, но возможности могут быть ограничены API соответствующего облачного провайдера.

### Добавление master-узлов в облачном кластере

Чтобы добавить master-узлы в облачном кластере:

1. Убедитесь, что включён модуль `control-plane-manager`.

1. Откройте файл `ClusterConfiguration` (например, `OpenStackClusterConfiguration`).

1. Добавьте или отредактируйте секцию `masterNodeGroup`:

   ```yaml
   masterNodeGroup:
     replicas: 3
     instanceClass:
       flavorName: m1.medium
       imageName: ubuntu-22-04-cloud-amd64
       rootDiskSize: 20
       mainNetwork: default
   ```

1. Примените изменения с помощью `dhctl converge`:

   ```console
   dhctl converge \
     --ssh-host <IP мастер-узла> \
     --ssh-user <пользователь> \
     --ssh-agent-private-keys /tmp/.ssh/<ключ>
   ```

### Добавление CloudEphemeral-узлов в облачном кластере

CloudEphemeral-узлы автоматически создаются и управляются в кластере с помощью Machine Controller Manager (MCM) или Cluster API (в зависимости от конфигурации) — оба компонента входят в состав модуля `node-manager` в DKP.

Для добавления узлов:

1. Убедитесь, что включён модуль облачного провайдера. Например: cloud-provider-yandex, cloud-provider-openstack, cloud-provider-aws.

1. Создайте объект `InstanceClass` с конфигурацией машин. Этот объект описывает параметры виртуальных машин, создаваемых в облаке:

   Пример (для OpenStack):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: OpenStackInstanceClass
   metadata:
     name: worker-instance
   spec:
     flavorName: m1.medium
     imageName: ubuntu-22-04-cloud-amd64
     rootDiskSize: 20
     mainNetwork: default
   ```

   Здесь задаются:
   - `flavorName` — тип инстанса (CPU/RAM);
   - `imageName` — образ ОС;
   - `rootDiskSize` — размер системного диска;
   - `mainNetwork`— облачная сеть для инстанса.

1. Создайте NodeGroup с типом CloudEphemeral. Пример манифеста:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: workers
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: OpenStackInstanceClass
         name: worker-instance
       minPerZone: 1
       maxPerZone: 3
       zones:
         - nova
     nodeTemplate:
       labels:
         node-role.deckhouse.io/worker: ""
       taints: []
   ```

1. Дождитесь запуска и автоматического добавления узлов.

## Конфигурация группы узлов

В Deckhouse (DKP) каждая группа узлов настраивается через объект NodeGroup. Этот объект описывает параметры узлов, которые DKP будет создавать или подключать к кластеру.

Значения по умолчанию для многих полей (например, nodeTemplate, kubelet, disruptions, taints) можно задать через объект NodeGroupConfiguration. Это особенно полезно, когда в кластере используется несколько NodeGroup с одинаковыми параметрами.

Это позволяет:

- централизованно управлять настройками для всех групп узлов;
- задавать единообразные значения без дублирования в каждой NodeGroup;
- изменять параметры узлов в кластере без ручного редактирования всех объектов NodeGroup.

Создание и настройка пользователей на узлах может быть автоматизирована через объекты NodeGroupConfiguration. С помощью объектов централизованно добавлять пользователей, задавать SSH-ключи, пароли и выполнять другие действия, связанные с безопасностью и управлением доступом.

### Общие настройки

Независимо от типа инфраструктуры (cloud или bare metal), объект NodeGroup содержит ряд параметров, определяющих поведение и характеристики узлов. Пример структуры:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    zones:
      - eu-west-1a
      - eu-west-1b
    minPerZone: 1
    maxPerZone: 2
    classReference:
      kind: AWSInstanceClass
      name: test
  nodeTemplate:
    labels:
      tier: test
```

### Настройки для групп с узлами Static и CloudStatic

Группы узлов с типами Static и CloudStatic предназначены для управления вручную созданными узлами — как физическими (bare-metal), так и виртуальными (в облаке, но без участия автоматических контроллеров DKP). Эти узлы подключаются вручную или через StaticInstance и не поддерживают автоматическое обновление и масштабирование.

Особенности конфигурации:

- Все действия по обновлению (обновление kubelet, перезапуск, замена узлов) выполняются вручную или через внешние автоматизации вне DKP.

- Рекомендуется явно указывать желаемую версию kubelet, чтобы обеспечить единообразие между узлами, особенно если они подключаются с разными версиями вручную:
  
  ```yaml
  nodeTemplate:
     kubelet:
       version: "1.28"
  ```

- Подключение узлов к кластеру может выполняться вручную или автоматически, в зависимости от конфигурации:
  - Вручную — пользователь скачивает bootstrap-скрипт, настраивает сервер, запускает скрипт вручную.
  - Автоматически (CAPS) — при использовании StaticInstance и SSHCredentials, DKP автоматически подключает и настраивает узлы.
  - Смешанный подход — вручную добавленный узел можно передать под управление CAPS, используя аннотацию `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

Если включён Cluster API Provider Static (CAPS), в NodeGroup можно использовать секцию staticInstances. Это позволяет DKP автоматически подключать, настраивать и, при необходимости, отключать статические узлы на основе ресурсов StaticInstance и SSHCredentials.

### Настройки для групп с узлами CloudEphemeral

Группы узлов с типом CloudEphemeral предназначены для автоматического масштабирования за счёт создания и удаления виртуальных машин в облаке с помощью Machine Controller Manager (MCM). Этот тип групп широко применяется в облачных кластерах DKP.

Конфигурация узлов задаётся в секции `cloudInstances` и включает параметры для масштабирования, зонирования, резервирования и приоритизации.

Пример базовой конфигурации:

```yaml
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 1
    maxPerZone: 5
    maxUnavailablePerZone: 1
    zones:
    - ru-central1-a
    - ru-central1-b
```

## Автомасштабирование группы узлов

В Deckhouse Kubernetes Platform (DKP) автомасштабирование группы узлов реализовано для групп с типом CloudEphemeral. Масштабирование происходит на основе потребностей в ресурсах (CPU и память) и выполняется компонентом `Cluster Autoscaler`, входящим в модуль `node-manager`.

Автоматическое масштабирование происходит только при наличии Pending-подов, которые не могут быть запущены на существующих узлах из-за нехватки ресурсов (например, CPU или памяти). В этом случае `Cluster Autoscaler` пытается добавить узлы, основываясь на конфигурации NodeGroup.

Основные параметры масштабирования задаются в секции `cloudInstances` ресурса NodeGroup:

- `minPerZone` — минимальное количество виртуальных машин в каждой зоне. Это число всегда поддерживается даже при отсутствии нагрузки.
- `maxPerZone` — максимальное количество узлов, которые можно создать в каждой зоне. Определяет верхнюю границу масштабирования.
- `maxUnavailablePerZone` — ограничивает количество недоступных узлов в процессе обновлений, удаления или создания.
- `standby` — опциональный параметр, позволяющий заранее запускать дополнительные узлы в режиме ожидания.
- `priority` — целочисленный приоритет группы. При масштабировании `Cluster Autoscaler` сначала выбирает группы с большим значением `priority`.Используется для задания порядка масштабирования между несколькими группами узлов.

Пример конфигурации группы узлов с автомасштабированием:

```yaml
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 1         # Минимальное количество узлов на зону.
    maxPerZone: 5         # Максимальное количество узлов на зону.
    maxUnavailablePerZone: 1  # Сколько узлов можно одновременно обновлять/удалять.
    zones:
      - nova
      - supernova
      - hypernova
```

### Пример сценария автомасштабирования

Имеется следующая группа узлов:

```yaml
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: m4.large
    minPerZone: 1
    maxPerZone: 5
    zones:
      - nova
      - supernova
      - hypernova
```

Также есть Deployment с конфигурацией:

```yaml
kind: Deployment
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        resources:
          requests:
            cpu: 1500m
            memory: 5Gi
```

Каждая VM может вместить максимум один такой под. Следовательно, для 3 реплик потребуются 3 узла — по одному в каждую зону.

Теперь увеличим количество реплик до 5. Два пода окажутся в статусе `Pending`. Cluster Autoscaler:

- Отследит эту ситуацию.
- Просчитает, сколько ресурсов не хватает.
- Решит создать ещё два узла.
- Передаст задание Machine Controller Manager.
- В облаке появятся 2 новые VM, которые автоматически подключатся к кластеру.
- Поды будут размещены на новых узлах.

## Перемещение узла между NodeGroup

{% alert level="warning" %}
В процессе переноса узлов между NodeGroup будет выполнена очистка и повторный бутстрап узла, объект `Node` будет пересоздан.
{% endalert %}

1. Создайте новый ресурс NodeGroup, например, с именем `front`, который будет управлять статическим узлом с лейблом `role: front`:

   ```yaml
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: front
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: front
   ```

1. Измените лейбл `role` у существующего StaticInstance с `worker` на `front`. Это позволит новой NodeGroup `front` начать управлять этим узлом:

   ```console
   kubectl label staticinstance static-worker-1 role=front --overwrite
   ```

1. Обновите ресурс NodeGroup `worker`, уменьшив значение параметра `count` с `1` до `0`:

   ```console
   kubectl patch nodegroup worker -p '{"spec": {"staticInstances": {"count": 0}}}' --type=merge
   ```

### Ручная очистка статического узла

Для отключения узла кластера и очистки сервера (виртуальной машины) нужно выполнить скрипт `/var/lib/bashible/cleanup_static_node.sh`, который уже находится на каждом статическом узле.

Пример отключения узла кластера и очистки сервера:

```console
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

{% alert level="info" %}
Инструкция справедлива как для узла, настроенного вручную (с помощью бутстрап-скрипта), так и для узла, настроенного с помощью CAPS.
{% endalert %}

## Примеры описания NodeGroup

### Облачные узлы

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    zones:
      - eu-west-1a
      - eu-west-1b
    minPerZone: 1
    maxPerZone: 2
    classReference:
      kind: AWSInstanceClass
      name: test
  nodeTemplate:
    labels:
      tier: test
```

### Статические узлы

Для виртуальных машин на гипервизорах или физических серверов используйте статические узлы, указав `nodeType: Static` в NodeGroup.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

Узлы в такую группу добавляются вручную с помощью подготовленных скриптов или автоматически с помощью Cluster API Provider Static (CAPS).

### Пример системной NodeGroup

Системные узлы — это узлы, предназначенные для запуска системных компонентов. Обычно они выделяются с помощью меток и taint'ов, чтобы туда не попадали пользовательские поды. Системные узлы могут быть как статическими, так и облачными.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: Static
```

## Примеры описания NodeGroupConfiguration

### Установка плагина cert-manager для kubectl на master-узлах

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-cert-manager-plugin.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "master"
  content: |
    if [ -x /usr/local/bin/kubectl-cert_manager ]; then
      exit 0
    fi
    curl -L https://github.com/cert-manager/cert-manager/releases/download/v1.7.1/kubectl-cert_manager-linux-amd64.tar.gz -o - | tar -zxvf - kubectl-cert_manager
    mv kubectl-cert_manager /usr/local/bin
```

### Задание параметра sysctl

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: sysctl-tune.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "*"
  content: |
    sysctl -w vm.max_map_count=262144
```

### Добавление корневого сертификата в хост

{% alert level="warning" %}
Данный пример приведен для ОС Ubuntu.  
Способ добавления сертификатов в хранилище может отличаться в зависимости от ОС.
При адаптации скрипта под другую ОС измените параметры `bundles` и `content`.
{% endalert %}

{% alert level="warning" %}
Для использования сертификата в `containerd` после добавления сертификата требуется произвести рестарт сервиса.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca.sh
spec:
  weight: 31
  nodeGroups:
  - '*'  
  bundles:
  - 'ubuntu-lts'
  content: |-
    CERT_FILE_NAME=example_ca
    CERTS_FOLDER="/usr/local/share/ca-certificates"
    CERT_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    JTEjMCEGA1UEAxMabmV4dXMuNTEuMjUwLjQxLjIuc3NsaXAuaW8wHhcNMjQwODAx
    MTAzMjA4WhcNMjQxMDMwMTAzMjA4WjAlMSMwIQYDVQQDExpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL1p
    WLPr2c4SZX/i4IS59Ly1USPjRE21G4pMYewUjkSXnYv7hUkHvbNL/P9dmGBm2Jsl
    WFlRZbzCv7+5/J+9mPVL2TdTbWuAcTUyaG5GZ/1w64AmAWxqGMFx4eyD1zo9eSmN
    G2jis8VofL9dWDfUYhRzJ90qKxgK6k7tfhL0pv7IHDbqf28fCEnkvxsA98lGkq3H
    fUfvHV6Oi8pcyPZ/c8ayIf4+JOnf7oW/TgWqI7x6R1CkdzwepJ8oU7PGc0ySUWaP
    G5bH3ofBavL0bNEsyScz4TFCJ9b4aO5GFAOmgjFMMUi9qXDH72sBSrgi08Dxmimg
    Hfs198SZr3br5GTJoAkCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB
    /wQCMAAwUwYDVR0RBEwwSoIPbmV4dXMuc3ZjLmxvY2FsghpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pb4IbZG9ja2VyLjUxLjI1MC40MS4yLnNzbGlwLmlvMA0GCSqG
    SIb3DQEBCwUAA4IBAQBvTjTTXWeWtfaUDrcp1YW1pKgZ7lTb27f3QCxukXpbC+wL
    dcb4EP/vDf+UqCogKl6rCEA0i23Dtn85KAE9PQZFfI5hLulptdOgUhO3Udluoy36
    D4WvUoCfgPgx12FrdanQBBja+oDsT1QeOpKwQJuwjpZcGfB2YZqhO0UcJpC8kxtU
    by3uoxJoveHPRlbM2+ACPBPlHu/yH7st24sr1CodJHNt6P8ugIBAZxi3/Hq0wj4K
    aaQzdGXeFckWaxIny7F1M3cIWEXWzhAFnoTgrwlklf7N7VWHPIvlIh1EYASsVYKn
    iATq8C7qhUOGsknDh3QSpOJeJmpcBwln11/9BGRP
    -----END CERTIFICATE-----
    EOF
    )

    # bb-event           - Creating subscription for event function. More information: http://www.bashbooster.net/#event
    ## ca-file-updated   - Event name
    ## update-certs      - The function name that the event will call
    
    bb-event-on "ca-file-updated" "update-certs"
    
    update-certs() {          # Function with commands for adding a certificate to the store
      update-ca-certificates
    }

    # bb-tmp-file - Creating temp file function. More information: http://www.bashbooster.net/#tmp
    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    # bb-sync-file                                - File synchronization function. More information: http://www.bashbooster.net/#sync
    ## "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"    - Destination file
    ##  ${CERT_TMP_FILE}                          - Source file
    ##  ca-file-updated                           - Name of event that will be called if the file changes.

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} \
      ca-file-updated   
```

### Добавление сертификата в ОС и containerd

{% alert level="warning" %}
Данный пример приведен для ОС Ubuntu.  
Способ добавления сертификатов в хранилище может отличаться в зависимости от ОС.
При адаптации скрипта под другую ОС измените параметры `bundles` и `content`.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca-containerd..sh
spec:
  weight: 31
  nodeGroups:
  - '*'  
  bundles:
  - 'ubuntu-lts'
  content: |-
    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/usr/local/share/ca-certificates"
    CERT_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    JTEjMCEGA1UEAxMabmV4dXMuNTEuMjUwLjQxLjIuc3NsaXAuaW8wHhcNMjQwODAx
    MTAzMjA4WhcNMjQxMDMwMTAzMjA4WjAlMSMwIQYDVQQDExpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL1p
    WLPr2c4SZX/i4IS59Ly1USPjRE21G4pMYewUjkSXnYv7hUkHvbNL/P9dmGBm2Jsl
    WFlRZbzCv7+5/J+9mPVL2TdTbWuAcTUyaG5GZ/1w64AmAWxqGMFx4eyD1zo9eSmN
    G2jis8VofL9dWDfUYhRzJ90qKxgK6k7tfhL0pv7IHDbqf28fCEnkvxsA98lGkq3H
    fUfvHV6Oi8pcyPZ/c8ayIf4+JOnf7oW/TgWqI7x6R1CkdzwepJ8oU7PGc0ySUWaP
    G5bH3ofBavL0bNEsyScz4TFCJ9b4aO5GFAOmgjFMMUi9qXDH72sBSrgi08Dxmimg
    Hfs198SZr3br5GTJoAkCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB
    /wQCMAAwUwYDVR0RBEwwSoIPbmV4dXMuc3ZjLmxvY2FsghpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pb4IbZG9ja2VyLjUxLjI1MC40MS4yLnNzbGlwLmlvMA0GCSqG
    SIb3DQEBCwUAA4IBAQBvTjTTXWeWtfaUDrcp1YW1pKgZ7lTb27f3QCxukXpbC+wL
    dcb4EP/vDf+UqCogKl6rCEA0i23Dtn85KAE9PQZFfI5hLulptdOgUhO3Udluoy36
    D4WvUoCfgPgx12FrdanQBBja+oDsT1QeOpKwQJuwjpZcGfB2YZqhO0UcJpC8kxtU
    by3uoxJoveHPRlbM2+ACPBPlHu/yH7st24sr1CodJHNt6P8ugIBAZxi3/Hq0wj4K
    aaQzdGXeFckWaxIny7F1M3cIWEXWzhAFnoTgrwlklf7N7VWHPIvlIh1EYASsVYKn
    iATq8C7qhUOGsknDh3QSpOJeJmpcBwln11/9BGRP
    -----END CERTIFICATE-----
    EOF
    )
    CONFIG_CONTENT=$(cat <<EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
        ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
    )
    
    mkdir -p /etc/containerd/conf.d

    # bb-tmp-file - Create temp file function. More information: http://www.bashbooster.net/#tmp

    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    CONFIG_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CONFIG_CONTENT}" > "${CONFIG_TMP_FILE}"  

    # bb-event           - Creating subscription for event function. More information: http://www.bashbooster.net/#event
    ## ca-file-updated   - Event name
    ## update-certs      - The function name that the event will call
    
    bb-event-on "ca-file-updated" "update-certs"
    
    update-certs() {          # Function with commands for adding a certificate to the store
      update-ca-certificates  # Restarting the containerd service is not required as this is done automatically in the script 032_configure_containerd.sh
    }

    # bb-sync-file                                - File synchronization function. More information: http://www.bashbooster.net/#sync
    ## "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"    - Destination file
    ##  ${CERT_TMP_FILE}                          - Source file
    ##  ca-file-updated                           - Name of event that will be called if the file changes.

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} \
      ca-file-updated   
      
    bb-sync-file \
      "/etc/containerd/conf.d/${REGISTRY_URL}.toml" \
      ${CONFIG_TMP_FILE} 
```

## Пользовательские настройки на узлах

Для автоматизации действий на узлах группы предусмотрен ресурс [NodeGroupConfiguration](cr.html#nodegroupconfiguration). Ресурс позволяет выполнять на узлах bash-скрипты, в которых можно пользоваться набором команд [bashbooster](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bashbooster), а также позволяет использовать шаблонизатор [Go Template](https://pkg.go.dev/text/template). Это удобно для автоматизации таких операций, как:

- Установка и настройки дополнительных пакетов ОС.  

  Примеры:  
  - [установка kubectl-плагина](examples.html#установка-плагина-cert-manager-для-kubectl-на-master-узлах);
  - [настройка containerd с поддержкой Nvidia GPU](faq.html#как-использовать-containerd-с-поддержкой-nvidia-gpu).

- Обновление ядра ОС на конкретную версию.

  Примеры:
  - [обновление ядра Debian](faq.html#для-дистрибутивов-основанных-на-debian);
  - [обновление ядра CentOS](faq.html#для-дистрибутивов-основанных-на-centos).

- Изменение параметров ОС.

  Примеры:  
  - [настройка параметра sysctl](examples.html#задание-параметра-sysctl);
  - [добавление корневого сертификата](examples.html#добавление-корневого-сертификата-в-хост).

- Сбор информации на узле и выполнение других подобных действий.

Ресурс `NodeGroupConfiguration` позволяет указывать [приоритет](cr.html#nodegroupconfiguration-v1alpha1-spec-weight) выполняемым скриптам, ограничивать их выполнение определенными [группами узлов](cr.html#nodegroupconfiguration-v1alpha1-spec-nodegroups) и [типами ОС](cr.html#nodegroupconfiguration-v1alpha1-spec-bundles).

Код скрипта указывается в параметре [content](cr.html#nodegroupconfiguration-v1alpha1-spec-content) ресурса. При создании скрипта на узле содержимое параметра `content` проходит через шаблонизатор [Go Template](https://pkg.go.dev/text/template), который позволят встроить дополнительный уровень логики при генерации скрипта. При прохождении через шаблонизатор становится доступным контекст с набором динамических переменных.

Переменные, которые доступны для использования в шаблонизаторе:
<ul>
<li><code>.cloudProvider</code> (для групп узлов с nodeType <code>CloudEphemeral</code> или <code>CloudPermanent</code>) — массив данных облачного провайдера.
{% offtopic title="Пример данных..." %}
```yaml
cloudProvider:
  instanceClassKind: OpenStackInstanceClass
  machineClassKind: OpenStackMachineClass
  openstack:
    connection:
      authURL: https://cloud.provider.com/v3/
      domainName: Default
      password: p@ssw0rd
      region: region2
      tenantName: mytenantname
      username: mytenantusername
    externalNetworkNames:
    - public
    instances:
      imageName: ubuntu-22-04-cloud-amd64
      mainNetwork: kube
      securityGroups:
      - kube
      sshKeyPairName: kube
    internalNetworkNames:
    - kube
    podNetworkMode: DirectRoutingWithPortSecurityEnabled
  region: region2
  type: openstack
  zones:
  - nova
```
{% endofftopic %}</li>
<li><code>.cri</code> — используемый CRI (с версии Deckhouse 1.49 используется только <code>Containerd</code>).</li>
<li><code>.kubernetesVersion</code> — используемая версия Kubernetes.</li>
<li><code>.nodeUsers</code> — массив данных о пользователях узла, добавленных через ресурс <a href="cr.html#nodeuser">NodeUser</a>.
{% offtopic title="Пример данных..." %}
```yaml
nodeUsers:
- name: user1
  spec:
    isSudoer: true
    nodeGroups:
    - '*'
    passwordHash: PASSWORD_HASH
    sshPublicKey: SSH_PUBLIC_KEY
    uid: 1050
```
{% endofftopic %}
</li>
<li><code>.nodeGroup</code> — массив данных группы узлов.
{% offtopic title="Пример данных..." %}
```yaml
nodeGroup:
  cri:
    type: Containerd
  disruptions:
    approvalMode: Automatic
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: "Off"
  kubernetesVersion: "1.29"
  manualRolloutID: ""
  name: master
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  nodeType: CloudPermanent
  updateEpoch: "1699879470"
```
{% endofftopic %}</li>
</ul>

{% raw %}
Пример использования переменных в шаблонизаторе:

```shell
{{- range .nodeUsers }}
echo 'Tuning environment for user {{ .name }}'
# Some code for tuning user environment
{{- end }}
```

Пример использования команд bashbooster:

```shell
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
```

{% endraw %}

Ход выполнения скриптов можно увидеть на узле в журнале сервиса bashible c помощью команды:

```bash
journalctl -u bashible.service
```  

Сами скрипты находятся на узле в директории `/var/lib/bashible/bundle_steps/`.  

Сервис принимает решение о повторном запуске скриптов путем сравнения единой контрольной суммы всех файлов, расположенной по пути `/var/lib/bashible/configuration_checksum` с контрольной суммой размещенной в кластере `kubernetes` в секрете `configuration-checksums` namespace `d8-cloud-instance-manager`.
Проверить контрольную сумму можно следующей командой:  

```bash
kubectl -n d8-cloud-instance-manager get secret configuration-checksums -o yaml
```  

Сравнение контрольных суммы сервис совершает каждую минуту.  

Контрольная сумма в кластере изменяется раз в 4 часа, тем самым повторно запуская скрипты на всех узлах.  
Принудительный вызов исполнения bashible на узле можно произвести путем удаления файла с контрольной суммой скриптов с помощью следующей команды:  

```bash
rm /var/lib/bashible/configuration_checksum
```

## Часто задаваемые вопросы по работе с узлами

### Можно ли удалить StaticInstance

StaticInstance, находящийся в состоянии `Pending` можно удалять без каких-либо проблем.

Чтобы удалить StaticInstance находящийся в любом состоянии, отличном от `Pending` (`Running`, `Cleaning`, `Bootstrapping`), выполните следующие шаги:

1. Добавьте метку `"node.deckhouse.io/allow-bootstrap": "false"` в `StaticInstance`.
1. Дождитесь, пока StaticInstance перейдет в статус `Pending`.
1. Удалите StaticInstance.
1. Уменьшите значение параметра `NodeGroup.spec.staticInstances.count` на 1.

### Как изменить IP-адрес StaticInstance

Изменить IP-адрес в ресурсе StaticInstance нельзя. Если в `StaticInstance` указан ошибочный адрес, то нужно [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

### Как изменить NodeGroup у статического узла

Если узел находится под управлением [CAPS](./#cluster-api-provider-static), то изменить принадлежность к `NodeGroup` у такого узла **нельзя**. Единственный вариант — [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

Чтобы перенести существующий статический узел созданный [вручную](./#работа-со-статическими-узлами) из одной `NodeGroup` в другую, необходимо изменить у узла лейбл группы:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Применение изменений потребует некоторого времени.

### Как понять, что что-то пошло не так

Если узел в NodeGroup не обновляется (значение `UPTODATE` при выполнении команды `kubectl get nodegroup` меньше значения `NODES`) или вы предполагаете какие-то другие проблемы, которые могут быть связаны с модулем `node-manager`, нужно проверить логи сервиса `bashible`. Сервис `bashible` запускается на каждом узле, управляемом модулем `node-manager`.

Чтобы проверить логи сервиса `bashible`, выполните на узле следующую команду:

```shell
journalctl -fu bashible
```

Пример вывода, когда все необходимые действия выполнены:

```console
May 25 04:39:16 kube-master-0 systemd[1]: Started Bashible service.
May 25 04:39:16 kube-master-0 bashible.sh[1976339]: Configuration is in sync, nothing to do.
May 25 04:39:16 kube-master-0 systemd[1]: bashible.service: Succeeded.
```

### Как посмотреть, что в данный момент выполняется на узле при его создании

Если необходимо узнать, что происходит на узле (например, узел долго создается), можно проверить логи `cloud-init`. Для этого выполните следующие шаги:

1. Найдите узел, который находится в стадии бутстрапа:

   ```shell
   kubectl get instances | grep Pending
   ```

   Пример:

   ```shell
   $ kubectl get instances | grep Pending
   dev-worker-2a6158ff-6764d-nrtbj   Pending   46s
   ```

1. Получите информацию о параметрах подключения для просмотра логов:

   ```shell
   kubectl get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   ```

   Пример:

   ```shell
   $ kubectl get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   bootstrapStatus:
     description: Use 'nc 192.168.199.178 8000' to get bootstrap logs.
     logsEndpoint: 192.168.199.178:8000
   ```

1. Выполните полученную команду (в примере выше — `nc 192.168.199.178 8000`), чтобы просмотреть логи `cloud-init` и определить, на каком этапе остановилась настройка узла.

Логи первоначальной настройки узла находятся в `/var/log/cloud-init-output.log`.

### Как обновить ядро на узлах

#### Для дистрибутивов, основанных на Debian

Создайте ресурс NodeGroupConfiguration, указав в переменной `desired_version` shell-скрипта (параметр `spec.content` ресурса) желаемую версию ядра:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="5.15.0-53-generic"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-apt-install "linux-image-${desired_version}"
```

#### Для дистрибутивов, основанных на CentOS

Создайте ресурс NodeGroupConfiguration, указав в переменной `desired_version` shell-скрипта (параметр `spec.content` ресурса) желаемую версию ядра:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="3.10.0-1160.42.2.el7.x86_64"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-dnf-install "kernel-${desired_version}"
```

### Какие параметры NodeGroup к чему приводят

| Параметр NG                           | Disruption update          | Перезаказ узлов   | Рестарт kubelet |
|---------------------------------------|----------------------------|-------------------|-----------------|
| chaos                                 | -                          | -                 | -               |
| cloudInstances.classReference         | -                          | +                 | -               |
| cloudInstances.maxSurgePerZone        | -                          | -                 | -               |
| cri.containerd.maxConcurrentDownloads | -                          | -                 | +               |
| cri.type                              | - (NotManaged) / + (other) | -                 | -               |
| disruptions                           | -                          | -                 | -               |
| kubelet.maxPods                       | -                          | -                 | +               |
| kubelet.rootDir                       | -                          | -                 | +               |
| kubernetesVersion                     | -                          | -                 | +               |
| nodeTemplate                          | -                          | -                 | -               |
| static                                | -                          | -                 | +               |
| update.maxConcurrent                  | -                          | -                 | -               |

В случае изменения параметров `InstanceClass` или `instancePrefix` в конфигурации Deckhouse не будет происходить `RollingUpdate`. Deckhouse создаст новые `MachineDeployment`, а старые удалит. Количество заказываемых одновременно `MachineDeployment` определяется параметром `cloudInstances.maxSurgePerZone`.

При обновлении, которое требует прерывания работы узла (disruption update), выполняется процесс вытеснения подов с узла. Если какой-либо под не может быть вытеснен, попытка повторяется каждые 20 секунд до достижения глобального таймаута в 5 минут. После истечения этого времени, поды, которые не удалось вытеснить, удаляются принудительно.

### Как пересоздать эфемерные машины в облаке с новой конфигурацией

При изменении конфигурации Deckhouse (как в модуле `node-manager`, так и в любом из облачных провайдеров) виртуальные машины не будут перезаказаны. Пересоздание происходит только после изменения ресурсов `InstanceClass` или `NodeGroup`.

Чтобы принудительно пересоздать все узлы, связанные с ресурсом `Machines`, следует добавить/изменить аннотацию `manual-rollout-id` в `NodeGroup`: `kubectl annotate NodeGroup имя_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## Как выделить узлы под специфические нагрузки#

{% alert level="warning" %}
Запрещено использование домена `deckhouse.io` в ключах `labels` и `taints` у `NodeGroup`. Он зарезервирован для компонентов Deckhouse. Следует отдавать предпочтение в пользу ключей `dedicated` или `dedicated.client.com`.
{% endalert %}

Для решений данной задачи существуют два механизма:

1. Установка меток в `NodeGroup` `spec.nodeTemplate.labels` для последующего использования их в `Pod` [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) или [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity). Указывает, какие именно узлы будут выбраны планировщиком для запуска целевого приложения.
1. Установка ограничений в `NodeGroup` `spec.nodeTemplate.taints` с дальнейшим снятием их в `Pod` [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Запрещает исполнение не разрешенных явно приложений на этих узлах.

{% alert level="info" %}
Deckhouse по умолчанию поддерживает использование taint'а с ключом `dedicated`, поэтому рекомендуется применять этот ключ с любым значением для taints на ваших выделенных узлах.

Если требуется использовать другие ключи для taints (например, `dedicated.client.com`), необходимо добавить соответствующее значение ключа в параметр [modules.placement.customTolerationKeys](../../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys). Это обеспечит разрешение системным компонентам, таким как `cni-flannel`, использовать эти узлы.
{% endalert %}

Подробнее [в статье на Habr](https://habr.com/ru/company/flant/blog/432748/).

### Как выделить узлы под системные компоненты

#### Фронтенд

Для Ingress-контроллеров используйте `NodeGroup` со следующей конфигурацией:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/frontend: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

#### Системные

Для компонентов подсистем Deckhouse параметр `NodeGroup` будет настроен с параметрами:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/system: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: system
```

### Как ускорить заказ узлов в облаке при горизонтальном масштабировании приложений

Самое действенное — держать в кластере некоторое количество предварительно подготовленных узлов, которые позволят новым репликам ваших приложений запускаться мгновенно. Очевидным минусом данного решения будут дополнительные расходы на содержание этих узлов.

Необходимые настройки целевой `NodeGroup` будут следующие:

1. Указать абсолютное количество предварительно подготовленных узлов (или процент от максимального количества узлов в этой группе) в параметре `cloudInstances.standby`.
1. При наличии на узлах дополнительных служебных компонентов, не обслуживаемых Deckhouse (например, DaemonSet `filebeat`), задать их процентное потребление ресурсов узла можно в параметре `standbyHolder.overprovisioningRate`.
1. Для работы этой функции требуется, чтобы как минимум один узел из группы уже был запущен в кластере. Иными словами, либо должна быть доступна одна реплика приложения, либо количество узлов для этой группы `cloudInstances.minPerZone` должно быть `1`.

Пример:

```yaml
cloudInstances:
  maxPerZone: 10
  minPerZone: 1
  standby: 10%
  standbyHolder:
    overprovisioningRate: 30%
```

### Как выключить machine-controller-manager/CAPI в случае выполнения потенциально деструктивных изменений в кластере

{% alert level="danger" %}
Использовать эту настройку допустимо только тогда, когда вы четко понимаете, зачем это необходимо.
{% endalert %}

Для того чтобы временно отключить machine-controller-manager (MCM) и предотвратить его автоматические действия, которые могут повлиять на инфраструктуру кластера (например, удаление или пересоздание узлов), установите следующий параметр в конфигурации:

```yaml
mcmEmergencyBrake: true
```

Для отключения CAPI установите следующий параметр в конфигурации:

```yaml
capiEmergencyBrake: true
```

### Как восстановить master-узел, если kubelet не может загрузить компоненты control plane?

Подобная ситуация может возникнуть, если в кластере с одним master-узлом на нем были удалены образы компонентов control plane (например, удалена директория `/var/lib/containerd`).
В этом случае kubelet при рестарте не сможет скачать образы компонентов `control plane`, поскольку на master-узле нет параметров авторизации в `registry.deckhouse.io`.

Далее приведена инструкция по восстановлению master-узла.

#### containerd

Для восстановления работоспособности master-узла нужно в любом рабочем кластере под управлением Deckhouse выполнить команду:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.io".auth'
```

Вывод команды нужно скопировать и присвоить переменной `AUTH` на поврежденном master-узле.

Далее на поврежденном master-узле нужно загрузить образы компонентов `control-plane`:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

После загрузки образов необходимо перезапустить `kubelet`.

### Как изменить CRI для NodeGroup

{% alert level="warning" %}
Смена CRI возможна только между `Containerd` на `NotManaged` и обратно (параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type)).
{% endalert %}

Для изменения CRI для NodeGroup, установите параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type) в `Containerd` или в `NotManaged`.

Пример YAML-манифеста NodeGroup:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  cri:
    type: Containerd
```

Также эту операцию можно выполнить с помощью патча:

* Для `Containerd`:

  ```shell
  kubectl patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* Для `NotManaged`:

  ```shell
  kubectl patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"NotManaged"}}}'
  ```

{% alert level="warning" %}
 При изменении `cri.type` для NodeGroup, созданных с помощью `dhctl`, необходимо обновить это значение в `dhctl config edit provider-cluster-configuration` и настройках объекта NodeGroup.
{% endalert %}

После изменения CRI для NodeGroup модуль `node-manager` будет поочередно перезагружать узлы, применяя новый CRI.  Обновление узла сопровождается простоем (disruption). В зависимости от настройки `disruption` для NodeGroup, модуль `node-manager` либо автоматически выполнит обновление узлов, либо потребует подтверждения вручную.

### Как изменить CRI для всего кластера

{% alert level="warning" %}
Смена CRI возможна только между `Containerd` на `NotManaged` и обратно (параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type)).
{% endalert %}

Для изменения CRI для всего кластера, необходимо с помощью утилиты `dhctl` отредактировать параметр `defaultCRI` в конфигурационном файле `cluster-configuration`.

Также возможно выполнить эту операцию с помощью `kubectl patch`.

* Для `Containerd`:

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/NotManaged/Containerd/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

* Для `NotManaged`:

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/NotManaged/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

Если необходимо, чтобы отдельные NodeGroup использовали другой CRI, перед изменением `defaultCRI` необходимо установить CRI для этой NodeGroup,
как описано [в документации](#как-изменить-cri-для-nodegroup).

{% alert level="danger" %}
Изменение `defaultCRI` влечет за собой изменение CRI на всех узлах, включая master-узлы.
Если master-узел один, данная операция является опасной и может привести к полной неработоспособности кластера.
Рекомендуется использовать multimaster-конфигурацию и менять тип CRI только после этого.
{% endalert %}

При изменении CRI в кластере для master-узлов необходимо выполнить дополнительные шаги:

1. Чтобы определить, какой узел в текущий момент обновляется в master NodeGroup, используйте следующую команду:

   ```shell
   kubectl get nodes -l node-role.kubernetes.io/control-plane="" -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
   ```

1. Подтвердите остановку (disruption) для master-узла, полученного на предыдущем шаге:

   ```shell
   kubectl annotate node <имя master-узла> update.node.deckhouse.io/disruption-approved=
   ```

1. Дождитесь перехода обновленного master-узла в `Ready`. Выполните итерацию для следующего master-узла.

### Как добавить шаг для конфигурации узлов

Дополнительные шаги для конфигурации узлов задаются с помощью кастомного ресурса [NodeGroupConfiguration](cr.html#nodegroupconfiguration).

### Как автоматически проставить на узел кастомные лейблы

1. На узле создайте каталог `/var/lib/node_labels`.

1. Создайте в нём файл или файлы, содержащие необходимые лейблы. Количество файлов может быть любым, как и вложенность подкаталогов, их содержащих.

1. Добавьте в файлы нужные лейблы в формате `key=value`. Например:

   ```console
   example-label=test
   ```

1. Сохраните файлы.

При добавлении узла в кластер указанные в файлах лейблы будут автоматически проставлены на узел.

{% alert level="warning" %}
Обратите внимание, что добавить таким образом лейблы, использующиеся в DKP, невозможно. Работать такой метод будет только с кастомными лейблами, не пересекающимися с зарезервированными для Deckhouse.
{% endalert %}

### Как использовать containerd с поддержкой Nvidia GPU

Необходимо создать отдельную NodeGroup для GPU-узлов:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: gpu
spec:
  chaos:
    mode: Disabled
  disruptions:
    approvalMode: Automatic
  nodeType: CloudStatic
```

Далее создайте NodeGroupConfiguration для NodeGroup `gpu` для конфигурации containerd:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
  - '*'
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/nvidia_gpu.toml - << "EOF"
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".containerd]
          default_runtime_name = "nvidia"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
            [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
                privileged_without_host_devices = false
                runtime_engine = ""
                runtime_root = ""
                runtime_type = "io.containerd.runc.v2"
                [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                  BinaryName = "/usr/bin/nvidia-container-runtime"
                  SystemdCgroup = false
    EOF
  nodeGroups:
  - gpu
  weight: 31
```

Добавьте NodeGroupConfiguration для установки драйверов Nvidia для NodeGroup `gpu`.

### Ubuntu

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - ubuntu-lts
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    if [ ! -f "/etc/apt/sources.list.d/nvidia-container-toolkit.list" ]; then
      distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
      curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | sudo apt-key add -
      curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
    fi
    bb-apt-install nvidia-container-toolkit nvidia-driver-535-server
    nvidia-ctk config --set nvidia-container-runtime.log-level=error --in-place
  nodeGroups:
  - gpu
  weight: 30
```

### Centos

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - centos
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    if [ ! -f "/etc/yum.repos.d/nvidia-container-toolkit.repo" ]; then
      distribution=$(. /etc/os-release;echo $ID$VERSION_ID) \
      curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo
    fi
    bb-dnf-install nvidia-container-toolkit nvidia-driver
    nvidia-ctk config --set nvidia-container-runtime.log-level=error --in-place
  nodeGroups:
  - gpu
  weight: 30
```

После того как конфигурации будут применены, необходимо провести бутстрап и перезагрузить узлы, чтобы применить настройки и установить драйвера.

#### Как проверить, что все прошло успешно

Создайте в кластере Job:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: nvidia-cuda-test
  namespace: default
spec:
  completions: 1
  template:
    spec:
      restartPolicy: Never
      nodeSelector:
        node.deckhouse.io/group: gpu
      containers:
        - name: nvidia-cuda-test
          image: nvidia/cuda:11.6.2-base-ubuntu20.04
          imagePullPolicy: "IfNotPresent"
          command:
            - nvidia-smi
```

Проверьте логи командой:

```shell
$ kubectl logs job/nvidia-cuda-test
Tue Jan 24 11:36:18 2023
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 525.60.13    Driver Version: 525.60.13    CUDA Version: 12.0     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|                               |                      |               MIG M. |
|===============================+======================+======================|
|   0  Tesla T4            Off  | 00000000:8B:00.0 Off |                    0 |
| N/A   45C    P0    25W /  70W |      0MiB / 15360MiB |      0%      Default |
|                               |                      |                  N/A |
+-------------------------------+----------------------+----------------------+

+-----------------------------------------------------------------------------+
| Processes:                                                                  |
|  GPU   GI   CI        PID   Type   Process name                  GPU Memory |
|        ID   ID                                                   Usage      |
|=============================================================================|
|  No running processes found                                                 |
+-----------------------------------------------------------------------------+
```

Создайте в кластере Job:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: gpu-operator-test
  namespace: default
spec:
  completions: 1
  template:
    spec:
      restartPolicy: Never
      nodeSelector:
        node.deckhouse.io/group: gpu
      containers:
        - name: gpu-operator-test
          image: nvidia/samples:vectoradd-cuda10.2
          imagePullPolicy: "IfNotPresent"
```

Проверьте логи командой:

```shell
$ kubectl logs job/gpu-operator-test
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

### Как развернуть кастомный конфигурационный файл containerd

{% alert level="danger" %}
Добавление кастомных настроек вызывает перезапуск сервиса `containerd`.
{% endalert %}

Bashible на узлах объединяет конфигурацию containerd для Deckhouse с конфигурацией из файла `/etc/containerd/conf.d/*.toml`.

{% alert level="warning" %}
Вы можете переопределять значения параметров, которые заданы в файле `/etc/containerd/deckhouse.toml`, но их работу придётся обеспечивать самостоятельно. Также, лучше изменением конфигурации не затрагивать master-узлы (nodeGroup `master`).
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-option-config.sh
spec:
  bundles:
    - '*'
  content: |
    # Copyright 2024 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_option.toml - << EOF
    oom_score = 500
    [metrics]
    address = "127.0.0.1"
    grpc_histogram = true
    EOF
  nodeGroups:
    - "worker"
  weight: 31
```

#### Как добавить авторизацию в дополнительный registry

Разверните скрипт `NodeGroupConfiguration`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
    - '*'
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.
    
    REGISTRY_URL=private.registry.example

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry-1.docker.io"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
              endpoint = ["https://${REGISTRY_URL}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
              auth = "AAAABBBCCCDDD=="
    EOF
  nodeGroups:
    - "*"
  weight: 31
```

#### Как настроить сертификат для дополнительного registry

{% alert level="info" %}
Помимо containerd, сертификат можно [одновременно добавить](#добавление-сертификата-в-ос-и-containerd) и в операционной системе.
{% endalert %}

Пример `NodeGroupConfiguration` для настройки сертификата для дополнительного registry:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: configure-cert-containerd.sh
spec:
  bundles:
  - '*'
  content: |-
    # Copyright 2024 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/var/lib/containerd/certs/"
    CERT_CONTENT=$(cat <<"EOF"
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    JTEjMCEGA1UEAxMabmV4dXMuNTEuMjUwLjQxLjIuc3NsaXAuaW8wHhcNMjQwODAx
    MTAzMjA4WhcNMjQxMDMwMTAzMjA4WjAlMSMwIQYDVQQDExpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL1p
    WLPr2c4SZX/i4IS59Ly1USPjRE21G4pMYewUjkSXnYv7hUkHvbNL/P9dmGBm2Jsl
    WFlRZbzCv7+5/J+9mPVL2TdTbWuAcTUyaG5GZ/1w64AmAWxqGMFx4eyD1zo9eSmN
    G2jis8VofL9dWDfUYhRzJ90qKxgK6k7tfhL0pv7IHDbqf28fCEnkvxsA98lGkq3H
    fUfvHV6Oi8pcyPZ/c8ayIf4+JOnf7oW/TgWqI7x6R1CkdzwepJ8oU7PGc0ySUWaP
    G5bH3ofBavL0bNEsyScz4TFCJ9b4aO5GFAOmgjFMMUi9qXDH72sBSrgi08Dxmimg
    Hfs198SZr3br5GTJoAkCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB
    /wQCMAAwUwYDVR0RBEwwSoIPbmV4dXMuc3ZjLmxvY2FsghpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pb4IbZG9ja2VyLjUxLjI1MC40MS4yLnNzbGlwLmlvMA0GCSqG
    SIb3DQEBCwUAA4IBAQBvTjTTXWeWtfaUDrcp1YW1pKgZ7lTb27f3QCxukXpbC+wL
    dcb4EP/vDf+UqCogKl6rCEA0i23Dtn85KAE9PQZFfI5hLulptdOgUhO3Udluoy36
    D4WvUoCfgPgx12FrdanQBBja+oDsT1QeOpKwQJuwjpZcGfB2YZqhO0UcJpC8kxtU
    by3uoxJoveHPRlbM2+ACPBPlHu/yH7st24sr1CodJHNt6P8ugIBAZxi3/Hq0wj4K
    aaQzdGXeFckWaxIny7F1M3cIWEXWzhAFnoTgrwlklf7N7VWHPIvlIh1EYASsVYKn
    iATq8C7qhUOGsknDh3QSpOJeJmpcBwln11/9BGRP
    -----END CERTIFICATE-----
    EOF
    )

    CONFIG_CONTENT=$(cat <<EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
        ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
    )

    mkdir -p ${CERTS_FOLDER}
    mkdir -p /etc/containerd/conf.d

    # bb-tmp-file - Create temp file function. More information: http://www.bashbooster.net/#tmp

    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    CONFIG_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CONFIG_CONTENT}" > "${CONFIG_TMP_FILE}"  

    # bb-sync-file                                - File synchronization function. More information: http://www.bashbooster.net/#sync
    ## "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"    - Destination file
    ##  ${CERT_TMP_FILE}                          - Source file

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} 

    bb-sync-file \
      "/etc/containerd/conf.d/${REGISTRY_URL}.toml" \
      ${CONFIG_TMP_FILE} 
  nodeGroups:
  - '*'  
  weight: 31
```

### Как использовать NodeGroup с приоритетом

С помощью параметра [priority](cr.html#nodegroup-v1-spec-cloudinstances-priority) кастомного ресурса `NodeGroup` можно задавать порядок заказа узлов в кластере.
Например, можно сделать так, чтобы сначала заказывались узлы типа *spot-node*, а если они закончились — обычные узлы. Или чтобы при наличии ресурсов в облаке заказывались узлы большего размера, а при их исчерпании — узлы меньшего размера.

Пример создания двух `NodeGroup` с использованием узлов типа spot-node:

```yaml
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-spot
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker-spot
    maxPerZone: 5
    minPerZone: 0
    priority: 50
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker
    maxPerZone: 5
    minPerZone: 0
    priority: 30
  nodeType: CloudEphemeral
```

В приведенном выше примере, `cluster-autoscaler` сначала попытается заказать узел типа *_spot-node*. Если в течение 15 минут его не получится добавить в кластер, NodeGroup `worker-spot` будет поставлен на паузу (на 20 минут) и `cluster-autoscaler` начнет заказывать узлы из NodeGroup `worker`.
Если через 30 минут в кластере возникнет необходимость развернуть еще один узел, `cluster-autoscaler` сначала попытается заказать узел из NodeGroup `worker-spot` и только потом — из NodeGroup `worker`.

После того как NodeGroup `worker-spot` достигнет своего максимума (5 узлов в примере выше), узлы будут заказываться из NodeGroup `worker`.

Шаблоны узлов (labels/taints) для NodeGroup `worker` и `worker-spot` должны быть одинаковыми, или как минимум подходить для той нагрузки, которая запускает процесс увеличения кластера.

### Как интерпретировать состояние группы узлов

**Ready** — группа узлов содержит минимально необходимое число запланированных узлов с состоянием `Ready` для всех зон.

Пример 1. Группа узлов в состоянии `Ready`:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

Пример 2. Группа узлов в состоянии `Not Ready`:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

**Updating** — группа узлов содержит как минимум один узел, в котором присутствует аннотация с префиксом `update.node.deckhouse.io` (например, `update.node.deckhouse.io/waiting-for-approval`).

**WaitingForDisruptiveApproval** — группа узлов содержит как минимум один узел, в котором присутствует аннотация `update.node.deckhouse.io/disruption-required` и
отсутствует аннотация `update.node.deckhouse.io/disruption-approved`.

**Scaling** — рассчитывается только для групп узлов с типом `CloudEphemeral`. Состояние `True` может быть в двух случаях:

1. Когда число узлов меньше *желаемого числа узлов в группе, то есть когда нужно увеличить число узлов в группе*.
1. Когда какой-то узел помечается к удалению или число узлов больше *желаемого числа узлов*, то есть когда нужно уменьшить число узлов в группе.

*Желаемое число узлов* — это сумма всех реплик, входящих в группу узлов.

Пример. Желаемое число узлов равно 2:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
...
  desired: 2
...
```

**Error** — содержит последнюю ошибку, возникшую при создании узла в группе узлов.

### Как заставить werf игнорировать состояние Ready в группе узлов

[werf](https://ru.werf.io) проверяет состояние `Ready` у ресурсов и в случае его наличия дожидается, пока значение станет `True`.

Создание (обновление) ресурса [nodeGroup](cr.html#nodegroup) в кластере может потребовать значительного времени на развертывание необходимого количества узлов. При развертывании такого ресурса в кластере с помощью werf (например, в рамках процесса CI/CD) развертывание может завершиться по превышении времени ожидания готовности ресурса. Чтобы заставить werf игнорировать состояние `nodeGroup`, необходимо добавить к `nodeGroup` следующие аннотации:

```yaml
metadata:
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
```

### Что такое ресурс Instance

Ресурс Instance в Kubernetes представляет собой описание объекта эфемерной виртуальной машины, но без конкретной реализации. Это абстракция, которая используется для управления машинами, созданными с помощью таких инструментов, как MachineControllerManager или Cluster API Provider Static.

Объект не содержит спецификации. Статус содержит:

1. Ссылку на InstanceClas`, если он существует для данной реализации.
1. Ссылку на объект Node Kubernetes.
1. Текущий статус машины.
1. Информацию о том, как проверить [логи создания машины](#как-посмотреть-что-в-данный-момент-выполняется-на-узле-при-его-создании) (появляется на этапе создания машины).

При создании или удалении машины создается или удаляется соответствующий объект Instance.
Самостоятельно ресурс Instance создать нельзя, но можно удалить. В таком случае машина будет удалена из кластера (процесс удаления зависит от деталей реализации).

## Когда требуется перезагрузка узлов?

Некоторые операции по изменению конфигурации узлов могут потребовать перезагрузки.

Перезагрузка узла может потребоваться при изменении некоторых настроек sysctl, например, при изменении параметра `kernel.yama.ptrace_scope` (изменяется при использовании команды `astra-ptrace-lock enable/disable` в Astra Linux).
