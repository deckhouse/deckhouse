---
title: "Управление узлами"
permalink: ru/admin/configuration/platform-scaling/node-management.html
lang: ru
---

## Общее описание

Управление узлами в Deckhouse Kubernetes Platform (DKP) реализовано с помощью модуля `node-manager`. Этот модуль позволяет:

- Автоматически масштабировать количество узлов в зависимости от нагрузки (автомасштабирование);
- Обновлять узлы и поддерживать их в актуальном состоянии;
- Упрощать управление конфигурацией групп узлов через CRD NodeGroup;
- Использовать различные типы узлов: постоянные, временные, в облаке или на bare-metal.

> DKP может работать как с bare-metal, так и с облачными кластерами, обеспечивая гибкость и расширяемость.

## Включение node-manager

Модуль можно включить или выключить несколькими способами:

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
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable node-manager
   # или disable
   ```

1. Через [веб-интерфейс Deckhouse](https://deckhouse.ru/products/kubernetes-platform/modules/console/stable/):

   - Перейдите в раздел «Deckhouse - «Модули»;
   - Найдите модуль `node-manager` и нажмите на него;
   - Включите тумблер «Модуль включен».

## Автоматическое развёртывание и обновление

В Deckhouse Kubernetes Platform (DKP) реализован автоматизированный механизм управления жизненным циклом узлов на основе объектов NodeGroup. DKP обеспечивает как начальное развёртывание узлов, так и их обновление при изменении конфигурации, поддерживая как облачные, так и bare-metal кластеры (при наличии интеграции с `node-manager`).

Как это работает:

1. NodeGroup — основной объект управления группами узлов. Он определяет тип узлов, их количество, шаблоны ресурсов и ключевые параметры (например, версия kubelet, настройки taint'ов и т.д.).
1. При создании или изменении NodeGroup, модуль `node-manager` автоматически приводит состояние узлов в соответствие с заданной конфигурацией.
1. Обновление происходит без вмешательства пользователя — устаревшие узлы удаляются, новые создаются автоматически.

Пример: автоматическое обновление версии kubelet.

1. Пользователь обновляет параметр `kubernetesVersion` в объекте NodeGroup.
1. DKP определяет, что узлы работают на устаревшей версии.
1. Последовательно создаются новые узлы с новой версией kubelet.
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
         kind: CloudInstanceClass
         name: my-class
     kubernetesVersion: "1.29"
   ```

## Типы узлов и механика добавления

В Deckhouse узлы разделяются на следующие типы:

- Static — управляются вручную, `node-manager` их не масштабирует и не пересоздаёт;
- CloudStatic — создаются автоматически через cloud-интеграцию, но не масштабируются;
- CloudPermanent — постоянные узлы, создаваемые и обновляемые `node-manager`;
- CloudEphemeral — временные узлы, создаваемые и масштабируемые в зависимости от нагрузки.

Узлы добавляются в кластер путём создания объекта NodeGroup, который описывает тип, параметры и конфигурацию группы узлов. DKP интерпретирует этот объект и создаёт соответствующие узлы, автоматически регистрируя их в Kubernetes-кластере.

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

   Скрипт нужно раскодировать из Base64, а затем выполнить от root.

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

1. Создать объект `StaticInstance` для каждого сервера:

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

   Под каждый сервер необходимо создавать отдельный ресурс `StaticInstance`, но можно использовать одни и те же `SSHCredentials` для доступа на разные серверы.

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

   Здесь добавляются параметры, которые описывают использование `StaticInstances`: `count` указывает, сколько узлов будет добавлено в эту группу; в `labelSelector` прописываются правила для создания выборки узлов.

После того как группа узлов будет создана, появится скрипт для добавления серверов в эту группу. DKP будет ждать, пока в кластере появится необходимое количество объектов `StaticInstance`, которые подходят под выборку по лейблам. Как только такой объект появится, DKP получит из созданных ранее манифестов IP-адрес сервера и параметры для подключения по SSH, подключится к серверу и выполнит на нём скрипт `bootstrap.sh`. После этого сервер добавится в заданную группу в качестве узла.

## Добавление узлов в cloud-кластер

### Добавление CloudPermanent-узлов в cloud-кластер

Чтобы добавить узлы типа `CloudPermanent` в облачный кластер DKP:

1. Убедитесь, что включён модуль облачного провайдера. Например, cloud-provider-aws, cloud-provider-openstack, cloud-provider-yandex и др.

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

1. Укажите параметры шаблона инстанса. Внутри `instanceClass` задаются параметры создаваемых виртуальных машин:
   - `flavorName` — тип инстанса (ресурсы: CPU, RAM);
   - `imageName` — образ ОС;
   - `rootDiskSize` — размер системного диска (в ГБ);
   - `mainNetwork` — имя сети;
   - при необходимости: диск ETCD, зоны, volume types и т.д.

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

### Добавление CloudEphemeral-узлов в cloud-кластер

CloudEphemeral-узлы автоматически создаются и управляются в кластере с помощью Machine Controller Manager (MCM) — компонента, входящего в состав модуля `node-manager` в DKP.

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

### Общие настройки

Независимо от типа инфраструктуры (cloud или bare metal), объект NodeGroup содержит ряд общих параметров, определяющих поведение и характеристики узлов. Пример структуры:

```yaml
spec:
  nodeType: CloudEphemeral | CloudPermanent | Static | CloudStatic
  nodeTemplate:
    labels:
    annotations:
    taints:
    resources:
    kubelet:
      version:
      containerRuntime:
  disruptions:
    approvalMode: Manual | Automatic
```

### Настройки для групп с узлами Static и CloudStatic

Группы узлов с типами Static и CloudStatic предназначены для управления вручную созданными узлами — как физическими (bare-metal), так и виртуальными (в облаке, но без участия автоматических контроллеров DKP). Эти узлы подключаются вручную или через `StaticInstance` и не поддерживают автоматическое обновление и масштабирование.

Особенности конфигурации:

- Все действия по обновлению (обновление kubelet, перезапуск, замена узлов) выполняются вручную или через внешние автоматизации вне DKP.

- Рекомендуется явно указывать желаемую версию kubelet, чтобы обеспечить единообразие между узлами, особенно если они подключаются с разными версиями вручную:
  
  ```yaml
  nodeTemplate:
     kubelet:
       version: "1.28"
  ```

- Требуется ручная подготовка сервера (установка ОС, запуск bootstrap-скрипта и т.д.).

Если включён Cluster API Provider Static (CAPS), для групп с Static и CloudStatic узлами можно использовать секцию `staticInstances`. Это позволяет DKP автоматически добавить уже подготовленные узлы на основе заранее заданных параметров.

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static-workers
spec:
  nodeType: Static
  staticInstances:
    count: 2
    labelSelector:
      matchExpressions: []
      matchLabels:
        static-node: group-a
```

### Настройки для групп с узлами CloudEphemeral

Группы узлов с типом CloudEphemeral предназначены для автоматического масштабирования за счёт создания и удаления виртуальных машин в облаке с помощью Machine Controller Manager (MCM). Этот тип групп широко применяется в cloud-кластерах DKP.

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

Дополнительно можно указать параметры:

- `lifecycleHooks` — для кастомных скриптов при создании/удалении инстансов;
- `cloudInstances.template` — шаблон параметров машины: CPU, RAM, диск, образ.

## Автомасштабирование группы узлов

В Deckhouse Kubernetes Platform (DKP) автомасштабирование группы узлов реализовано для групп с типом CloudEphemeral. Масштабирование происходит на основе потребностей в ресурсах (CPU и память) и выполняется компонентом `Cluster Autoscaler`, входящим в модуль `node-manager`.

Основные параметры масштабирования в NodeGroup:

```yaml
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 1         # Минимальное количество узлов на зону
    maxPerZone: 5         # Максимальное количество узлов на зону
    maxUnavailablePerZone: 1  # Сколько узлов можно одновременно обновлять/удалять
    zones:
      - nova
      - supernova
      - hypernova
```

- `minPerZone` — минимальное количество виртуальных машин в каждой зоне. Это число всегда поддерживается даже при отсутствии нагрузки.
- `maxPerZone` — максимальное количество узлов, которые можно создать в каждой зоне. Определяет верхнюю границу масштабирования.
- `maxUnavailablePerZone` — ограничивает количество недоступных узлов в процессе обновлений, удаления или создания.

DKP отслеживает ресурсы (CPU, память) и автоматически масштабирует группы с типом CloudEphemeral. Основные параметры:

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

1. Измените лейбл `role` у существующего `StaticInstance` с `worker` на `front`. Это позволит новой NodeGroup `front` начать управлять этим узлом:

   ```console
   kubectl label staticinstance static-worker-1 role=front --overwrite
   ```

1. Обновите ресурс NodeGroup `worker`, уменьшив значение параметра `count` с `1` до `0`:

   ```console
   kubectl patch nodegroup worker -p '{"spec": {"staticInstances": {"count": 0}}}' --type=merge
   ```

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

Узлы в такую группу добавляются вручную с помощью подготовленных скриптов. Также можно использовать способ добавления статических узлов с помощью Cluster API Provider Static.

### Системные узлы

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
