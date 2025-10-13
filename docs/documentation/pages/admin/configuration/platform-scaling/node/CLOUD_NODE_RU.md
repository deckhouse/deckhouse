---
title: "Добавление и управление облачными узлами"
permalink: ru/admin/configuration/platform-scaling/node/cloud-node.html
lang: ru
---

В Deckhouse Kubernetes Platform (DKP) облачные узлы могут быть следующих типов:

- **CloudEphemeral** — временные, автоматически создаваемые и удаляемые узлы;
- **CloudPermanent** — постоянные узлы, управляемые вручную через `replicas`;
- **CloudStatic** — статические облачные узлы. Машины создаются вручную или внешними средствами, а DKP подключает их к кластеру и управляет ими как обычными узлами.

Ниже приведены инструкции по добавлению и настройке каждого типа.

## Добавление CloudEphemeral-узлов в облачном кластере

CloudEphemeral-узлы автоматически создаются и управляются в кластере с помощью Machine Controller Manager (MCM) или Cluster API (в зависимости от конфигурации) — оба компонента входят в состав модуля [`node-manager`](/modules/node-manager/) в DKP.

Для добавления узлов:

1. Убедитесь, что включён модуль облачного провайдера. Например: [`cloud-provider-yandex`](/modules/cloud-provider-yandex/), [`cloud-provider-openstack`](/modules/cloud-provider-openstack/), [`cloud-provider-aws`](/modules/cloud-provider-aws/).

1. Создайте объект [InstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) с конфигурацией машин. Этот объект описывает параметры виртуальных машин, создаваемых в облаке:

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

1. Создайте [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом `CloudEphemeral`. Пример манифеста:

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

## Настройки для групп с узлами CloudEphemeral

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

## Изменение конфигурации облачного провайдера в кластере

Настройки используемого облачного провайдера в облачном или гибридном кластере хранятся в структуре `<PROVIDER_NAME>ClusterConfiguration`, где `<PROVIDER_NAME>` — название/код провайдера. Например, для провайдера OpenStack структура будет называться [OpenStackClusterConfiguration]({% if site.mode == 'module' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/products/kubernetes-platform/documentation/v1/{% endif %}/modules/cloud-provider-openstack/cluster_configuration.html).

Независимо от используемого облачного провайдера его настройки можно изменить с помощью следующей команды:

```shell
d8 platform edit provider-cluster-configuration
```

## Автомасштабирование группы узлов

В Deckhouse Kubernetes Platform (DKP) автомасштабирование группы узлов происходит на основе потребностей в ресурсах (CPU и память) и выполняется компонентом `Cluster Autoscaler`, входящим в модуль [`node-manager`](/modules/node-manager/).

Автоматическое масштабирование происходит только при наличии Pending-подов, которые не могут быть запущены на существующих узлах из-за нехватки ресурсов (например, CPU или памяти). В этом случае `Cluster Autoscaler` пытается добавить узлы, основываясь на конфигурации NodeGroup.

Основные параметры масштабирования задаются в [секции `cloudInstances`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances) ресурса NodeGroup:

- `minPerZone` — минимальное количество виртуальных машин в каждой зоне. Это число всегда поддерживается даже при отсутствии нагрузки.
- `maxPerZone` — максимальное количество узлов, которые можно создать в каждой зоне. Определяет верхнюю границу масштабирования.
- `maxUnavailablePerZone` — ограничивает количество недоступных узлов в процессе обновлений, удаления или создания.
- `standby` — опциональный параметр, позволяющий заранее запускать дополнительные узлы в режиме ожидания.
- `priority` — целочисленный приоритет группы. При масштабировании `Cluster Autoscaler` сначала выбирает группы с большим значением `priority`. Используется для задания порядка масштабирования между несколькими группами узлов.

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

Предположим, у вас есть следующая группа узлов:

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

В этом случае для запуска всех реплик потребуются 3 узла — по одному в каждую зону.

Теперь увеличим количество реплик до 5. В результате два пода окажутся в статусе `Pending`.

`Cluster Autoscaler`:

- отследит эту ситуацию;
- просчитает, сколько ресурсов не хватает;
- решит создать ещё два узла;
- передаст задание Machine Controller Manager;
- в облаке появятся 2 новые VM, которые автоматически подключатся к кластеру;
- поды будут размещены на новых узлах.

### Выделение узлов под специфические нагрузки

{% alert level="warning" %}
Запрещено использование домена `deckhouse.io` в ключах `labels` и `taints` у [NodeGroup](/modules/node-manager/cr.html#nodegroup). Он зарезервирован для компонентов DKP. Следует отдавать предпочтение в пользу ключей `dedicated` или `dedicated.client.com`.
{% endalert %}

Для решений данной задачи существуют два механизма:

1. Установка меток в [NodeGroup](/modules/node-manager/cr.html#nodegroup) `spec.nodeTemplate.labels` для последующего использования их в `Pod` [`spec.nodeSelector`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) или [`spec.affinity.nodeAffinity`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity). Указывает, какие именно узлы будут выбраны планировщиком для запуска целевого приложения.
1. Установка ограничений в NodeGroup `spec.nodeTemplate.taints` с дальнейшим снятием их в `Pod` [`spec.tolerations`](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Запрещает исполнение не разрешенных явно приложений на этих узлах.

{% alert level="info" %}
DKP по умолчанию поддерживает использование taint'а с ключом `dedicated`, поэтому рекомендуется применять этот ключ с любым значением для taints на ваших выделенных узлах.

Если требуется использовать другие ключи для taints (например, `dedicated.client.com`), необходимо добавить соответствующее значение ключа в параметр `modules.placement.customTolerationKeys`. Это обеспечит разрешение системным компонентам, таким как `cni-flannel`, использовать эти узлы.
{% endalert %}

### Ускорение заказа узлов в облаке при горизонтальном масштабировании приложений

Для ускорения запуска новых реплик приложений при автоматическом горизонтальном масштабировании рекомендуется поддерживать в кластере определённое количество предварительно подготовленных (standby) узлов. Это позволяет быстро размещать новые поды приложений без ожидания создания и инициализации узлов.
Следует учитывать, что наличие "запасных" узлов увеличивает расходы на инфраструктуру.

Необходимые настройки целевой [NodeGroup](/modules/node-manager/cr.html#nodegroup) будут следующие:

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

## Пример описания NodeGroup

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

## Конфигурация CloudEphemeral-узлов через NodeGroupConfiguration

Дополнительные настройки для облачных узлов можно задать с помощью объектов [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Они позволяют:

- изменять параметры ОС (например, `sysctl`);
- добавлять корневые сертификаты;
- настраивать доверие к собственным container registries и т.п.

NodeGroupConfiguration применяется к новым узлам при их создании, включая CloudEphemeral.

{% alert level="info" %}  
NodeGroupConfiguration применяется только к узлам с указанным образом операционной системы (`bundle`).
В качестве значения `bundle` можно указать как конкретное имя (например, `ubuntu-lts`, `centos-7`, `rocky-linux`), так и `*` — чтобы применить настройку ко всем образам ОС.
{% endalert %}

### Примеры описания NodeGroupConfiguration

#### Задание параметра sysctl

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

#### Добавление корневого сертификата в хост

{% alert level="warning" %}

- Данный пример приведен для ОС Ubuntu.  
  Способ добавления сертификатов в хранилище может отличаться в зависимости от ОС.
  При адаптации скрипта под другую ОС измените параметры `bundles` и `content`.
- После добавления сертификата для его использования в `containerd` требуется перезапустить сервис `containerd` на узле.

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
    ...
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

#### Добавление сертификата в ОС и containerd

{% alert level="warning" %}
Данный пример приведен для ОС Ubuntu.  
Способ добавления сертификатов в хранилище может отличаться в зависимости от ОС.
При адаптации скрипта под другую ОС измените параметры `bundles` и `content`.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca-containerd.sh
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
    ...
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

### Обновление ядра на узлах

#### Для дистрибутивов, основанных на Debian

Создайте ресурс [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration), указав в переменной `desired_version` shell-скрипта (параметр `spec.content` ресурса) желаемую версию ядра:

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

Создайте ресурс [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration), указав в переменной `desired_version` shell-скрипта (параметр `spec.content` ресурса) желаемую версию ядра:

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

## Добавление CloudPermanent-узлов в облачном кластере

Чтобы добавить узлы типа CloudPermanent в облачный кластер DKP:

1. Убедитесь, что включён модуль облачного провайдера. Например, [`cloud-provider-yandex`](/modules/cloud-provider-yandex/), [`cloud-provider-openstack`](/modules/cloud-provider-openstack/), [`cloud-provider-aws`](/modules/cloud-provider-aws/) и др.

   Это можно проверить с помощью команды:

   ```shell
   d8 k -n d8-system get modules
   ```

   Или посмотреть в веб-интерфейсе Deckhouse.

1. Создайте объект [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом `CloudPermanent`. Узлы типа `CloudPermanent` управляются через Terraform, встроенный в DKP. Конфигурация таких узлов находится в объекте `(Provider)ClusterConfiguration`. Редактировать его нужно с помощью утилиты `dhctl` в установочном контейнере. Пример:

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
   - при необходимости: диск etcd, зоны, volume types и т.д.

     Для других облаков названия и структура параметров могут отличаться. Актуальные поля можно посмотреть в описании CRD или в документации по соответствующему облачному провайдеру.

1. Примените конфигурацию с помощью `dhctl converge`. После редактирования `(Provider)ClusterConfiguration` выполните:

   ```shell
   dhctl converge \
     --ssh-host <IP мастер-узла> \
     --ssh-user <имя пользователя> \
     --ssh-agent-private-keys /tmp/.ssh/<ключ>
   ```

   Эта команда:
   - запустит Terraform;
   - создаст нужные виртуальные машины;
   - выполнит на них установку DKP (через `bootstrap.sh`);
   - зарегистрирует узлы в кластере.

1. Готово — новые узлы появятся в кластере автоматически. Их можно увидеть выполнив команду:

   ```shell
   d8 k get nodes
   ```

   Также список новых узлов доступен в веб-интерфейсе Deckhouse.

Deckhouse Kubernetes Platform может работать поверх сервисов Managed Kubernetes (например, GKE и EKS). При этом модуль [`node-manager`](/modules/node-manager/) обеспечивает управление конфигурацией и автоматизацию действий с узлами, но возможности могут быть ограничены API соответствующего облачного провайдера.

## Добавление CloudStatic узла в кластер

Добавление статического узла можно выполнить вручную или с помощью Cluster API Provider Static (CAPS).

### Вручную

Чтобы добавить новый статический узел в кластер вручную, выполните следующие шаги:

1. Для [CloudStatic-узлов](/modules/node-manager/cr.html#nodegroup) в облачных провайдерах, перечисленных ниже, выполните описанные в документации шаги:
   - [для AWS](/modules/cloud-provider-aws/faq.html#добавление-cloudstatic-узлов-в-кластер);
   - [для GCP](/modules/cloud-provider-gcp/faq.html#добавление-cloudstatic-узлов-в-кластер);
   - [для YC](/modules/cloud-provider-yandex/faq.html#добавление-cloudstatic-узлов-в-кластер).
1. Используйте существующий или создайте новый ресурс [NodeGroup](/modules/node-manager/cr.html#nodegroup). Параметр `nodeType` в ресурсе NodeGroup для статических узлов должен быть `Static` или `CloudStatic`.
1. Получите код скрипта в кодировке Base64 для добавления и настройки узла.

   Пример получения кода скрипта в кодировке Base64 для добавления узла в NodeGroup `worker`:

   ```shell
   NODE_GROUP=worker
   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} -o json | jq '.data."bootstrap.sh"' -r
   ```

1. Выполните предварительную настройку нового узла в соответствии с особенностями вашего окружения. Например:
   - добавьте необходимые точки монтирования в файл `/etc/fstab` (NFS, Ceph и т. д.);
   - установите необходимые пакеты;
   - настройте сетевую связность между новым узлом и остальными узлами кластера.
1. Зайдите на новый узел по SSH и выполните следующую команду, вставив полученную в п. 3 Base64-строку:

   ```shell
   echo <Base64-КОД-СКРИПТА> | base64 -d | bash
   ```

### С помощью Cluster API Provider Static

Простой пример добавления статического узла в кластер с помощью Cluster API Provider Static (CAPS):

1. Подготовьте необходимые ресурсы.

   * Выделите сервер (или виртуальную машину), настройте сетевую связность и т. п., при необходимости установите специфические пакеты ОС и добавьте точки монтирования которые потребуются на узле.

   * Создайте пользователя (в примере — `caps`) с возможностью выполнять `sudo`, выполнив **на сервере** следующую команду:

     ```shell
     useradd -m -s /bin/bash caps 
     usermod -aG sudo caps
     ```

   * Разрешите пользователю выполнять команды через `sudo` без пароля. Для этого **на сервере** внесите следующую строку в конфигурацию `sudo`(отредактировав файл `/etc/sudoers`, выполнив команду `sudo visudo` или другим способом):

     ```text
     caps ALL=(ALL) NOPASSWD: ALL
     ```

   * Сгенерируйте **на сервере** пару SSH-ключей с пустой парольной фразой:

     ```shell
     ssh-keygen -t rsa -f caps-id -C "" -N ""
     ```

     Публичный и приватный ключи пользователя `caps` будут сохранены в файлах `caps-id.pub` и `caps-id` в текущей директории на сервере.

   * Добавьте полученный публичный ключ в файл `/home/caps/.ssh/authorized_keys` пользователя `caps`, выполнив в директории с ключами **на сервере** следующие команды:

     ```shell
     mkdir -p /home/caps/.ssh 
     cat caps-id.pub >> /home/caps/.ssh/authorized_keys 
     chmod 700 /home/caps/.ssh 
     chmod 600 /home/caps/.ssh/authorized_keys
     chown -R caps:caps /home/caps/
     ```

   В операционных системах семейства Astra Linux, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя `caps`:

     ```shell
     pdpl-user -i 63 caps
     ```

1. Создайте в кластере ресурс [SSHCredentials](/modules/node-manager/cr.html#sshcredentials).

   В директории с ключами пользователя **на сервере** выполните следующую команду для получения закрытого ключа в формате Base64:

   ```shell
   base64 -w0 caps-id
   ```

   На любом компьютере с `d8 k`, настроенным на управление кластером, создайте переменную окружения со значением закрытого ключа созданного пользователя в Base64, полученным на предыдущем шаге:

   ```shell
    CAPS_PRIVATE_KEY_BASE64=<ЗАКРЫТЫЙ_КЛЮЧ_В_BASE64>
   ```

   Выполните следующую команду для создания в кластере ресурса [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) (здесь и далее также используйте `d8 k`, настроенный на управление кластером):

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: SSHCredentials
   metadata:
     name: credentials
   spec:
     user: caps
     privateSSHKey: "${CAPS_PRIVATE_KEY_BASE64}"
   EOF
   ```

1. Создайте в кластере ресурс [StaticInstance](/modules/node-manager/cr.html#staticinstance), указав IP-адрес сервера статического узла:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-worker-1
     labels:
       role: worker
   spec:
     # Укажите IP-адрес сервера статического узла.
     address: "<SERVER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   EOF
   ```

1. Создайте в кластере ресурс [NodeGroup](/modules/node-manager/cr.html#nodegroup). Параметр `count` обозначает количество `staticInstances`, подпадающих под `labelSelector`, которые будут добавлены в кластер, в данном случае `1`:

   > Поле `labelSelector` в ресурсе NodeGroup является неизменным. Чтобы обновить `labelSelector`, нужно создать новую NodeGroup и перенести в неё статические узлы, изменив их лейблы (labels).

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: worker
   EOF
   ```

### С помощью Cluster API Provider Static для нескольких групп узлов

Пример использования фильтров в `label selector` [StaticInstance](/modules/node-manager/cr.html#staticinstance), для группировки статических узлов и использования их в разных NodeGroup. В примере используются две группы узлов (`front` и `worker`), предназначенные для разных задач, которые должны содержать разные по характеристикам узлы — два сервера для группы `front` и один для группы `worker`.

1. Подготовьте необходимые ресурсы (3 сервера или виртуальные машины) и создайте ресурс [SSHCredentials](/modules/node-manager/cr.html#sshcredentials), аналогично п.1 и п.2 [примера](#с-помощью-cluster-api-provider-static).

1. Создайте в кластере два ресурса [NodeGroup](/modules/node-manager/cr.html#nodegroup):

   > Поле `labelSelector` в ресурсе NodeGroup является неизменным. Чтобы обновить `labelSelector`, нужно создать новую NodeGroup и перенести в неё статические узлы, изменив их лейблы.

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: front
   spec:
     nodeType: Static
     staticInstances:
       count: 2
       labelSelector:
         matchLabels:
           role: front
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: worker
   EOF
   ```

1. Создайте в кластере ресурсы [StaticInstance](/modules/node-manager/cr.html#staticinstance), указав актуальные IP-адреса серверов:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-1
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP1>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-2
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP2>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-worker-1
     labels:
       role: worker
   spec:
     address: "<SERVER-WORKER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   EOF
   ```

## Добавление master-узлов в облачном кластере

Чтобы добавить master-узлы в облачном кластере:

1. Убедитесь, что включён модуль [`control-plane-manager`](/modules/control-plane-manager/).

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

   ```shell
   dhctl converge \
     --ssh-host <IP мастер-узла> \
     --ssh-user <пользователь> \
     --ssh-agent-private-keys /tmp/.ssh/<ключ>
   ```

## Использование NodeGroup с приоритетом

С помощью параметра `priority` кастомного ресурса [NodeGroup](/modules/node-manager/cr.html#nodegroup) можно задавать порядок заказа узлов в кластере.
Например, можно сделать так, чтобы сначала заказывались узлы типа *spot-node*, а если они закончились — обычные узлы. Или чтобы при наличии ресурсов в облаке заказывались узлы большего размера, а при их исчерпании — узлы меньшего размера.

Пример создания двух NodeGroup с использованием узлов типа spot-node:

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

В приведенном выше примере, `Cluster Autoscaler` сначала попытается заказать узел типа *_spot-node*. Если в течение 15 минут его не получится добавить в кластер, NodeGroup `worker-spot` будет поставлен на паузу (на 20 минут) и `Cluster Autoscaler` начнет заказывать узлы из NodeGroup `worker`.

Если через 30 минут в кластере возникнет необходимость развернуть еще один узел, `Cluster Autoscaler` сначала попытается заказать узел из NodeGroup `worker-spot` и только потом — из NodeGroup `worker`.

После того как NodeGroup `worker-spot` достигнет своего максимума (5 узлов в примере выше), узлы будут заказываться из NodeGroup `worker`.

Шаблоны узлов (labels/taints) для NodeGroup `worker` и `worker-spot` должны быть одинаковыми, или как минимум подходить для той нагрузки, которая запускает процесс увеличения кластера.

## Состояния группы узлов и их интерпретация

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

1. Когда число узлов меньше *желаемого числа узлов в группе*, то есть когда нужно увеличить число узлов в группе.
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
