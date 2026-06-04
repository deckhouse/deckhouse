---
title: Гибридный кластер с Yandex Cloud
permalink: ru/admin/integrations/hybrid/yandex-hybrid.html
lang: ru
search: гибрид с Yandex Cloud
description: Подготовка к гибридной интеграции с Yandex Cloud в Deckhouse Kubernetes Platform.
---

Далее описан процесс добавления worker-узлов из Yandex Cloud в существующий статический кластер Deckhouse Kubernetes Platform (DKP).

Для интеграции с Yandex Cloud используется модуль [`cloud-provider-yandex`](/modules/cloud-provider-yandex/). Он обеспечивает взаимодействие DKP с API Yandex Cloud, получение информации об облачной инфраструктуре, создание виртуальных машин, работу с сетевыми параметрами и подключение узлов к существующему кластеру.

В разделе описаны два способа добавления worker-узлов:

- **Автоматическое создание узлов в Yandex Cloud**. DKP создаёт виртуальные машины через API Yandex Cloud. Параметры ВМ задаются ресурсом [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass), а требуемое количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом `CloudEphemeral`.
- **Подключение вручную созданных узлов через bootstrap-скрипт**. Виртуальная машина создаётся пользователем заранее и подключается к кластеру с помощью bootstrap-скрипта DKP. Для такого сценария используется [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом `CloudStatic`.

## Предварительные требования

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и VPC Yandex Cloud настроена [сетевая связность](./overview#общие-сетевые-требования).
- Узлы Yandex Cloud, добавляемые в кластер, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [«Сетевое взаимодействие»](../../../../reference/network_interaction.html) и [«Настройка сетевых политик»](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [«Подключение и авторизация в Yandex Cloud»](../public/yandex/authorization.html):
  - подготовлен сервисный аккаунт;
  - выбран каталог, в котором будут создаваться ресурсы;
  - настроены необходимые роли и доступ к используемой VPC.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

## Добавление автоматически создаваемых узлов

Для выполнения подготовительных команд нужен [Yandex Cloud CLI](https://yandex.cloud/ru/docs/cli/) (`yc`). Его можно использовать на рабочей машине администратора. На master-узле кластера `yc` не требуется: в кластере нужно применить только подготовленные манифесты.

1. Получите идентификаторы облака и каталога, в котором будут создаваться worker-узлы:

   ```shell
   yc resource-manager cloud list
   yc resource-manager folder list
   ```

1. Укажите полученные идентификаторы в переменных:

   ```shell
   export CLOUD_ID="<CLOUD_ID>"
   export FOLDER_ID="<FOLDER_ID>"
   ```

   Где:

   - `CLOUD_ID` — ID облака Yandex Cloud;
   - `FOLDER_ID` — ID каталога, в котором будут создаваться ресурсы.

1. Получите идентификаторы сети, подсети и зоны, где будут создаваться worker-узлы:

   ```shell
   yc vpc network list --folder-id "$FOLDER_ID"
   yc vpc subnet list --folder-id "$FOLDER_ID"
   ```

1. Укажите полученные значения в переменных:

   ```shell
   export NETWORK_ID="<NETWORK_ID>"
   export SUBNET_ID="<SUBNET_ID>"
   export ZONE="<ZONE>"
   ```

   Где:

   - `NETWORK_ID` — ID VPC-сети;
   - `SUBNET_ID` — ID подсети, в которой будут создаваться worker-узлы;
   - `ZONE` — зона доступности, соответствующая выбранной подсети, например `ru-central1-a`.

   Подробнее в разделе [«Подключение и авторизация в Yandex Cloud»](../public/yandex/authorization.html).

1. Создайте сервисный аккаунт в нужном каталоге Yandex Cloud и назначьте ему права:

   ```shell
   yc iam service-account create \
     --name dkp-hybrid \
     --folder-id "$FOLDER_ID"

   export SA_ID="$(yc iam service-account get \
     --name dkp-hybrid \
     --folder-id "$FOLDER_ID" \
     --format json | jq -r .id)"

   yc resource-manager folder add-access-binding "$FOLDER_ID" \
     --role editor \
     --subject "serviceAccount:${SA_ID}"

   yc resource-manager folder add-access-binding "$FOLDER_ID" \
     --role vpc.admin \
     --subject "serviceAccount:${SA_ID}"
   ```

   Роль `editor` нужна для создания и управления облачными ресурсами, а `vpc.admin` — для работы с сетевыми ресурсами VPC.

1. Создайте ключ сервисного аккаунта и сохраните его в JSON-файл:

   ```shell
   yc iam key create \
     --service-account-id "$SA_ID" \
     --output dkp-hybrid-sa-key.json
   ```

   Сохраните JSON-ключ сервисного аккаунта в переменную окружения `SERVICE_ACCOUNT_JSON` в однострочном формате:

   ```shell
   export SERVICE_ACCOUNT_JSON="$(jq -c . dkp-hybrid-sa-key.json)"
   ```

1. Сохраните публичный SSH-ключ администратора в переменную окружения `SSH_PUBLIC_KEY`:

   ```shell
   export SSH_PUBLIC_KEY="$(cat ~/.ssh/id_rsa.pub)"
   ```

   Если используется другой ключ, укажите путь к нему вместо `~/.ssh/id_rsa.pub`.

   {% alert level="warning" %}
   В переменную `SSH_PUBLIC_KEY` нужно сохранить публичный SSH-ключ администратора, который будет использоваться для доступа к создаваемым worker-узлам. Не используйте публичный ключ из JSON-файла сервисного аккаунта.
   {% endalert %}

1. Получите ID образа операционной системы, из которого будут создаваться виртуальные машины, и сохраните его в переменную окружения `IMAGE_ID`:

   ```shell
   export IMAGE_ID="$(yc compute image get-latest-from-family ubuntu-2404-lts \
     --folder-id standard-images \
     --format json | jq -r .id)"
   ```

   {% alert level="warning" %}
   В переменную `IMAGE_ID` нужно сохранить ID образа ОС в Yandex Cloud. Не используйте ID существующей виртуальной машины или ID ключа сервисного аккаунта.
   {% endalert %}

1. Укажите CIDR сети, в которой будут размещаться узлы Yandex Cloud:

   ```shell
   export NODE_NETWORK_CIDR="<NODE_NETWORK_CIDR>"
   ```

   `NODE_NETWORK_CIDR` — CIDR, включающий внутренние IP-адреса узлов Yandex Cloud. Для одной зоны обычно совпадает с CIDR выбранной подсети. Например, если worker-узлы создаются в подсети `10.128.0.0/24`, укажите `10.128.0.0/24`. Узнать CIDR подсети можно командой:

   ```shell
   yc vpc subnet list --folder-id "$FOLDER_ID"
   ```

1. Создайте файл с конфигурацией провайдера. Например, `cloud-provider-cluster-configuration.yaml`:

   ```shell
   cat > cloud-provider-cluster-configuration.yaml <<EOF
   apiVersion: deckhouse.io/v1
   kind: YandexClusterConfiguration
   layout: WithoutNAT
   masterNodeGroup:
     replicas: 1
     instanceClass:
       cores: 4
       memory: 8192
       imageID: ${IMAGE_ID}
       diskSizeGB: 100
       platform: standard-v3
       externalIPAddresses:
         - "Auto"
   nodeNetworkCIDR: ${NODE_NETWORK_CIDR}
   existingNetworkID: empty
   provider:
     cloudID: ${CLOUD_ID}
     folderID: ${FOLDER_ID}
     serviceAccountJSON: '${SERVICE_ACCOUNT_JSON}'
   sshPublicKey: '${SSH_PUBLIC_KEY}'
   EOF
   ```

   В манифест автоматически подставляются значения переменных окружения, заданных на предыдущих шагах: `CLOUD_ID`, `FOLDER_ID`, `IMAGE_ID`, `NODE_NETWORK_CIDR`, `SERVICE_ACCOUNT_JSON` и `SSH_PUBLIC_KEY`.

   {% alert level="info" %}
   В гибридном сценарии, когда control plane уже развёрнут как статический кластер, секция `masterNodeGroup` не приводит к созданию master-узлов в Yandex Cloud, но остаётся частью конфигурации провайдера.
   {% endalert %}

1. Создайте файл с discovery-данными Yandex Cloud. Например, `cloud-provider-discovery-data.json`:

   ```shell
   cat > cloud-provider-discovery-data.json <<EOF
   {
     "apiVersion": "deckhouse.io/v1",
     "defaultLbTargetGroupNetworkId": "empty",
     "internalNetworkIDs": [
       "${NETWORK_ID}"
     ],
     "kind": "YandexCloudDiscoveryData",
     "monitoringAPIKey": "",
     "region": "ru-central1",
     "routeTableID": "empty",
     "shouldAssignPublicIPAddress": false,
     "zoneToSubnetIdMap": {
       "${ZONE}": "${SUBNET_ID}"
     },
     "zones": [
       "${ZONE}"
     ]
   }
   EOF
   ```

   В файл автоматически подставляются значения переменных окружения, заданных на предыдущих шагах: `NETWORK_ID`, `SUBNET_ID` и `ZONE`.

   Параметр `shouldAssignPublicIPAddress` управляет назначением публичных IP-адресов создаваемым worker-узлам. В примере указано значение `false`, поэтому создаваемые узлы будут получать только внутренние IP-адреса.

   {% alert level="warning" %}
   Если `shouldAssignPublicIPAddress` установлен в `false`, создаваемые узлы должны иметь доступ к хранилищу образов и внешним сервисам через NAT Gateway, NAT-инстанс, прокси или другой egress-механизм. Для зон, в которых подсети отсутствуют, допустимо использовать значение `empty`.
   {% endalert %}

1. Закодируйте файлы `cloud-provider-cluster-configuration.yaml` и `cloud-provider-discovery-data.json` в Base64:

   ```shell
   export CLUSTER_CONFIGURATION_B64="$(base64 -w0 cloud-provider-cluster-configuration.yaml)"
   export DISCOVERY_DATA_B64="$(base64 -w0 cloud-provider-discovery-data.json)"
   ```

1. Создайте манифест с секретом `d8-provider-cluster-configuration` и ModuleConfig для включения и настройки модуля `cloud-provider-yandex`:

   ```shell
   cat > yandex-provider-secret-and-mc.yaml <<EOF
   apiVersion: v1
   kind: Secret
   metadata:
     labels:
       name: d8-provider-cluster-configuration
     name: d8-provider-cluster-configuration
     namespace: kube-system
   type: Opaque
   data:
     cloud-provider-cluster-configuration.yaml: ${CLUSTER_CONFIGURATION_B64}
     cloud-provider-discovery-data.json: ${DISCOVERY_DATA_B64}
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-yandex
   spec:
     version: 1
     enabled: true
     settings:
       storageClass:
         default: network-ssd
   EOF
   ```

1. Скопируйте файл `yandex-provider-secret-and-mc.yaml` на master-узел кластера. Примените манифест:

   ```shell
   d8 k apply -f yandex-provider-secret-and-mc.yaml
   ```

1. Дождитесь включения модуля `cloud-provider-yandex` и появления кастомного ресурса YandexInstanceClass:

   ```shell
   d8 k get moduleconfig cloud-provider-yandex
   d8 k get crd yandexinstanceclasses.deckhouse.io
   d8 k -n d8-cloud-provider-yandex get pods -o wide
   ```

1. Создайте файл с манифестами [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup). Например, `yandex-instanceclass-nodegroup.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: YandexInstanceClass
   metadata:
     name: yc-worker
   spec:
     cores: 4
     memory: 8192
     diskSizeGB: 50
     diskType: network-ssd
     mainSubnet: <SUBNET_ID>
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: yc-worker
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: YandexInstanceClass
         name: yc-worker
       minPerZone: 1
       maxPerZone: 1
       zones:
         - ru-central1-a
   ```

   Где:

   - YandexInstanceClass описывает параметры виртуальной машины, которая будет создана в Yandex Cloud;
   - `mainSubnet` — ID подсети, из которой создаваемые worker-узлы должны иметь доступ к статическим узлам кластера;
   - NodeGroup описывает группу узлов, которую DKP должен поддерживать в кластере;
   - `nodeType: CloudEphemeral` означает, что узлы будут создаваться автоматически через облачного провайдера;
   - `cloudInstances.zones` должен содержать зоны из списка `zones` в `cloud-provider-discovery-data.json`.

1. Примените манифест:

   ```shell
   d8 k apply -f yandex-instanceclass-nodegroup.yaml
   ```

   После применения DKP начнёт создавать виртуальную машину в Yandex Cloud через machine-controller-manager.

1. Проверьте появление узла в кластере:

   ```shell
   d8 k get nodes -o wide
   ```

   Пример ожидаемого результата:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                                 STATUS   ROLES                  AGE   VERSION    INTERNAL-IP
   static-master-0                      Ready    control-plane,master   1h    v1.33.10   10.128.0.15
   yc-worker-f3564dca-7fc59-s2w5d       Ready    yc-worker              10m   v1.33.10   10.128.0.21
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Для диагностики состояния и поиска возможных проблем проверьте логи machine-controller-manager:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinedeployments.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager get machinesets.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager get machines.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

## Добавление вручную созданных узлов через bootstrap-скрипт

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) включён и настроен:

  ```shell
  d8 k get moduleconfig cloud-provider-yandex 
  d8 k get module cloud-provider-yandex -o wide
  ```

- Компоненты модуля `cloud-provider-yandex` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-yandex get pods -o wide
  ```

- В Yandex Cloud создана виртуальная машина, которая будет подключена к кластеру.
- Виртуальная машина подключена к сети и подсети Yandex Cloud, используемым для гибридной интеграции с кластером.
- У виртуальной машины есть сетевой интерфейс в VPC-сети и подсети Yandex Cloud, используемых для гибридной интеграции с кластером. IP-адрес этого интерфейса должен входить в CIDR, указанный в `nodeNetworkCIDR`, и быть доступен со стороны статических узлов кластера.
- Имя виртуальной машины в Yandex Cloud совпадает с именем хоста (hostname) внутри операционной системы.
- На виртуальной машине установлены необходимые базовые пакеты для поддерживаемой ОС. Для РЕД ОС заранее установите `which` и пакетный менеджер, если они отсутствуют.

1. Проверьте метаданные виртуальной машины в Yandex Cloud.

   В метаданных ВМ должен быть настроен `cloud-init` с пользователем, через которого будет выполняться подключение по SSH.

   Пример метаданных:

   ```yaml
   #cloud-config
   datasource:
     Ec2:
       strict_id: false
   ssh_pwauth: no
   users:
     - name: <USER>
       sudo: ALL=(ALL) NOPASSWD:ALL
       shell: /bin/bash
       ssh_authorized_keys:
         - <SSH_PUBLIC_KEY>
   ```

   Где:

   - `<USER>` — имя пользователя для SSH-доступа к виртуальной машине;
   - `<SSH_PUBLIC_KEY>` — публичный SSH-ключ администратора.

1. На master-узле создайте файл с манифестом NodeGroup, указав имя группы узлов (в примере здесь и далее используется имя `yc-manual`). Например, `yandex-manual-nodegroup.yaml`:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: yc-manual
   spec:
     nodeType: CloudStatic
   ```

1. Убедитесь, что NodeGroup создана и синхронизирована:

   ```shell
   d8 k get nodegroup yc-manual
   d8 k describe nodegroup yc-manual
   ```

   Пример ожидаемого результата:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME        TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   yc-manual   CloudStatic   0       0       0                                                               1m    True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. На master-узле получите bootstrap-скрипт для созданной NodeGroup:

   ```shell
   NODE_GROUP=yc-manual

   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} \
     -o jsonpath='{.data.bootstrap\.sh}' > ${NODE_GROUP}-bootstrap.b64
   ```

1. На master-узле проверьте, что файл содержит корректные Base64-данные bootstrap-скрипта:

   ```shell
   base64 -d ${NODE_GROUP}-bootstrap.b64 > /dev/null
   ```

   Проверьте начало декодированного содержимого:

   ```shell
   base64 -d ${NODE_GROUP}-bootstrap.b64 | head -n 5
   ```

   В начале декодированного содержимого должен быть bash-скрипт:

   ```console
   #!/bin/bash
   ...
   ```

   {% alert level="info" %}
   Для копирования и запуска bootstrap-скрипта используйте пользователя, указанного в метаданных ВМ.
   {% endalert %}

1. Скопируйте bootstrap-скрипт на подключаемую ВМ. Если SSH-доступ к ВМ есть с master-узла, выполните на master-узле:

   ```shell
   scp ${NODE_GROUP}-bootstrap.b64 <USER>@<NODE_PUBLIC_OR_INTERNAL_IP>:/tmp/bootstrap.b64
   ```

   Если SSH-доступ к ВМ есть только с рабочей машины администратора, сначала скопируйте файл с master-узла на рабочую машину, а затем с рабочей машины на ВМ:

   ```shell
   scp <MASTER_USER>@<MASTER_IP>:/root/${NODE_GROUP}-bootstrap.b64 ./bootstrap.b64
   scp ./bootstrap.b64 <USER>@<NODE_PUBLIC_OR_INTERNAL_IP>:/tmp/bootstrap.b64
   ```

   Где:

   - `<MASTER_USER>` — пользователь для SSH-доступа к master-узлу;
   - `<MASTER_IP>` — IP-адрес master-узла;
   - `<USER>` — пользователь на подключаемой ВМ;
   - `<NODE_PUBLIC_OR_INTERNAL_IP>` — публичный или внутренний IP-адрес подключаемой ВМ.

1. На подключаемой ВМ декодируйте bootstrap-скрипт, назначьте права и запустите его:

   ```shell
   base64 -d /tmp/bootstrap.b64 > /tmp/bootstrap.sh
   chmod +x /tmp/bootstrap.sh

   sudo bash /tmp/bootstrap.sh
   ```

   После запуска bootstrap-скрипт установит необходимые компоненты, настроит container runtime, kubelet и подключит узел к кластеру.

1. На master-узле проверьте появление нового узла:

   ```shell
   d8 k get nodes -o wide
   ```

   Пример ожидаемого результата:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                    STATUS   ROLES       AGE   VERSION    INTERNAL-IP   EXTERNAL-IP
   static-master-0         Ready    master      1h    v1.33.10   10.128.0.15   <none>
   yandex-worker-hybrid    Ready    yc-manual   5m    v1.33.10   10.128.0.17   <PUBLIC_IP>
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. При сбоях подключения проверьте состояние NodeGroup, события и логи компонентов:

   ```shell
   d8 k get nodegroup yc-manual
   d8 k describe nodegroup yc-manual
   d8 k describe node yandex-worker-hybrid
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```
