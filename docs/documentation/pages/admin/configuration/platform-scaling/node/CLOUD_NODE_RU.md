---
title: "Добавление и управление облачными узлами"
permalink: ru/admin/configuration/platform-scaling/node/cloud-node.html
lang: ru
---

В Deckhouse Kubernetes Platform облачные узлы могут быть следующих типов:

- **CloudEphemeral** — временные, автоматически создаваемые и удаляемые узлы;
- **CloudPermanent** — постоянные узлы, управляемые вручную через `replicas`;
- (опционально) **CloudStatic** — узлы, созданные вне Deckhouse, но интегрированные в кластер;
- (опционально) **CloudHybrid** — узлы, управляемые совместно с внешними системами.

Ниже приведены инструкции по добавлению и настройке каждого типа.

## Добавление CloudEphemeral-узлов в облачном кластере

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

Дополнительные настройки для облачных узлов можно задать с помощью объектов NodeGroupConfiguration. Они позволяют:

- изменять параметры ОС (например, `sysctl`);
- добавлять корневые сертификаты;
- настраивать доверие к собственным реестрам образов и т.п.

NodeGroupConfiguration применяется к новым узлам при их создании, включая CloudEphemeral.
  
> NodeGroupConfiguration можно применять только к узлам с определённым образом операционной системы, указав соответствующий `bundle`. Например:
>
> - `ubuntu-lts`
> - `centos-7`
> - `rocky-linux`
> - `*` — ко всем.

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

## Добавление CloudPermanent-узлов в облачном кластере

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

## Добавление master-узлов в облачном кластере

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

