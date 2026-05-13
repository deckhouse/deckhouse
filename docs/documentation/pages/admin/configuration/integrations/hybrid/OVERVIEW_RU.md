---
title: Гибридная интеграция
permalink: ru/admin/integrations/hybrid/overview.html
lang: ru
---

Гибридный кластер — это кластер DKP, в котором базовая часть размещается в собственной инфраструктуре, а дополнительные worker-узлы подключаются из внешнего облака или среды виртуализации, например из Yandex Cloud, VCD или vSphere.

Такой подход позволяет увеличивать вычислительные мощности, размещать часть нагрузки во внешней инфраструктуре или постепенно переносить туда сервисы без создания отдельного Kubernetes-кластера. Для приложений при этом сохраняется единая плоскость управления Kubernetes: общий API, единые ресурсы, единые механизмы планирования, мониторинга, обновления и эксплуатации.

В типовом сценарии сначала разворачивается кластер с [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype). В таком кластере control plane и базовые worker-узлы размещаются на собственных серверах или заранее подготовленных виртуальных машинах. Затем включается модуль соответствующего cloud provider, через который DKP получает информацию о внешней инфраструктуре и может работать с размещёнными в ней узлами.

В DKP гибридная архитектура строится на сочетании разных типов групп узлов:

- [`Static`](../../../../architecture/cluster-and-infrastructure/node-management/static-nodes.html) — постоянно существующие узлы, которые создаются и обслуживаются пользователем;
- [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) — узлы, которые DKP создаёт и удаляет автоматически через API провайдера;
- [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html) — узлы, которые создаются вручную во внешней инфраструктуре и затем подключаются к кластеру.

Подключение узлов из внешней инфраструктуры может выполняться двумя способами:

- **Автоматическое создание узлов**. Используется тип узлов `CloudEphemeral` (в Yandex — тип узлов `Cloud`). Параметры виртуальных машин описываются ресурсом `*InstanceClass` (например, [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass)), а количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup). После применения этих ресурсов DKP обращается к API провайдера, создаёт виртуальные машины, подготавливает их и подключает к существующему кластеру как worker-узлы.
- **Подключение вручную созданных узлов**. Используется тип узлов `CloudStatic`. Виртуальные машины создаются пользователем вручную во внешней инфраструктуре, после чего подключаются к кластеру с помощью bootstrap-скрипта DKP или через Cluster API Provider Static (CAPS) как worker-узлы.

В этом разделе описаны общие требования к гибридным кластерам, предварительная подготовка инфраструктуры и добавление узлов через поддерживаемых провайдеров.

## Общие сетевые требования

Между статическими узлами кластера и узлами, размещёнными во внешней инфраструктуре, должна быть настроена сетевая связность, достаточная для работы компонентов DKP.

Подключаемые узлы должны иметь доступ к Kubernetes API, DNS и необходимым адресам внешних сервисов, включая container registry. Компоненты DKP, взаимодействующие с внешней инфраструктурой, должны иметь доступ к API соответствующего провайдера.

Полный перечень соединений приведён в разделе [Сетевое взаимодействие](../../../../reference/network_interaction.html), а рекомендации по ограничениям доступа — в разделе [Настройка сетевых политик](../../configuration/network/policy/configuration.html).

Дополнительно рекомендуется проверить:

- маршрутизацию между сетями статических и подключаемых узлов;
- одинаковое значение MTU на всём сетевом пути, особенно при использовании туннелей;
- доступность DNS-серверов и разрешённых внешних адресов;
- доступность Kubernetes API для подключаемых узлов;
- параметры инкапсуляции трафика при использовании Cilium, включая [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), если между площадками применяется фильтрация трафика.

Конкретные требования к сетям, подсетям, шаблонам виртуальных машин, учётным данным и дополнительным параметрам зависят от используемого провайдера инфраструктуры и приведены в разделе «Предварительные требования» для соответствующего провайдера.

## Гибридный кластер с Yandex Cloud

Далее описан процесс создания гибридного кластера, объединяющего статические (bare-metal) узлы и облачные узлы в Yandex Cloud с использованием Deckhouse Kubernetes Platform (DKP).

### Предварительные требования для Yandex Cloud

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и VPC Yandex Cloud настроена сетевая связность.
- Узлы, создаваемые в Yandex Cloud, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [Настройка сетевых политик](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [Авторизация в Yandex Cloud](../public/yandex/authorization.html):
  - подготовлен сервисный аккаунт;
  - выбран каталог, в котором будут создаваться ресурсы;
  - настроены необходимые роли и доступ к используемой VPC.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

### Добавление автоматически создаваемых узлов в Yandex Cloud

В этом сценарии исходный кластер DKP уже развёрнут как статический кластер. Control plane остаётся на статическом узле, а дополнительные worker-узлы создаются в Yandex Cloud через модуль `cloud-provider-yandex`.

Для выполнения подготовительных команд нужен Yandex Cloud CLI (`yc`). Его можно использовать на рабочей машине администратора. На master-узле кластера `yc` не требуется: в кластере нужно применить только подготовленные манифесты.

1. Подготовьте идентификаторы облака, каталога, сети, подсети и зоны, где будут создаваться worker-узлы:

   ```shell
   export CLOUD_ID="<CLOUD_ID>"
   export FOLDER_ID="<FOLDER_ID>"
   export NETWORK_ID="<NETWORK_ID>"
   export SUBNET_ID="<SUBNET_ID>"
   export ZONE="ru-central1-a"
   ```

   Получить значения можно через Yandex Cloud CLI:

   ```shell
   yc resource-manager cloud list
   yc resource-manager folder list
   yc vpc network list --folder-id "$FOLDER_ID"
   yc vpc subnet list --folder-id "$FOLDER_ID"
   ```

   Где:

   - `CLOUD_ID` — ID облака Yandex Cloud;
   - `FOLDER_ID` — ID каталога, в котором будут создаваться ресурсы;
   - `NETWORK_ID` — ID VPC-сети;
   - `SUBNET_ID` — ID подсети, в которой будут создаваться worker-узлы;
   - `ZONE` — зона доступности, соответствующая выбранной подсети.

   Подробнее в разделе [Авторизация в Yandex Cloud](../public/yandex/authorization.html)

1. Создайте Service Account в нужном каталоге Yandex Cloud и назначьте ему права:

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

1. Создайте ключ Service Account и сохраните его в JSON-файл:

   ```shell
   yc iam key create \
     --service-account-id "$SA_ID" \
     --output dkp-hybrid-sa-key.json
   ```

   Подготовьте значение `serviceAccountJSON` в однострочном формате:

   ```shell
   export SERVICE_ACCOUNT_JSON="$(jq -c . dkp-hybrid-sa-key.json)"
   ```

1. Подготовьте публичный SSH-ключ, который будет добавлен на создаваемые worker-узлы:

   ```shell
   export SSH_PUBLIC_KEY="$(cat ~/.ssh/id_rsa.pub)"
   ```

   Если используется другой ключ, укажите путь к нему вместо `~/.ssh/id_rsa.pub`.

   {% alert level="warning" %}
   В параметре `sshPublicKey` нужно передавать публичный SSH-ключ администратора, а не публичный ключ из JSON-файла Service Account.
   {% endalert %}

1. Получите ID образа операционной системы, из которого будут создаваться виртуальные машины:

   ```shell
   export IMAGE_ID="$(yc compute image get-latest-from-family ubuntu-2404-lts \
     --folder-id standard-images \
     --format json | jq -r .id)"
   ```

   {% alert level="warning" %}
   Параметр `imageID` — это ID образа ОС в Yandex Cloud. Не используйте в этом поле ID существующей виртуальной машины или ID ключа Service Account.
   {% endalert %}

1. Укажите CIDR сети, в которой будут размещаться узлы Yandex Cloud:

   ```shell
   export NODE_NETWORK_CIDR="<NODE_NETWORK_CIDR>"
   ```

   `NODE_NETWORK_CIDR` — CIDR, включающий внутренние IP-адреса узлов Yandex Cloud. Для одной зоны обычно совпадает с CIDR выбранной подсети. Например, если worker-узлы создаются в подсети `10.128.0.0/24`, укажите `10.128.0.0/24`. Узнать CIDR подсети можно командой:

   ```shell
   yc vpc subnet list --folder-id "$FOLDER_ID"
   ```

1. Создайте файл, например `cloud-provider-cluster-configuration.yaml` с конфигурацией провайдера:

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

   Где:

   - `nodeNetworkCIDR` — CIDR сети, который включает адреса подсетей, используемых для узлов Yandex Cloud;
   - `imageID` — ID образа ОС для создаваемых виртуальных машин;
   - `cloudID` — ID облака Yandex Cloud;
   - `folderID` — ID каталога Yandex Cloud;
   - `serviceAccountJSON` — JSON-ключ Service Account в однострочном формате;
   - `sshPublicKey` — публичный SSH-ключ для доступа к создаваемым узлам.

   {% alert level="info" %}
   В гибридном сценарии, когда control plane уже развёрнут как статический кластер, секция `masterNodeGroup` не приводит к созданию master-узлов в Yandex Cloud, но остаётся частью конфигурации провайдера.
   {% endalert %}

1. Создайте файл, например `cloud-provider-discovery-data.json` с discovery-данными Yandex Cloud:

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

   Где:

   - `internalNetworkIDs` — список ID сетей Yandex Cloud, через которые обеспечивается внутренняя связность между узлами;
   - `zoneToSubnetIdMap` — соответствие зоны доступности и подсети, в которой будут создаваться узлы;
   - `zones` — список зон, доступных для создания узлов;
   - `shouldAssignPublicIPAddress` — управляет назначением публичных IP-адресов создаваемым узлам.

   {% alert level="warning" %}
   Если параметр `shouldAssignPublicIPAddress` установлен в `false`, у создаваемых узлов не будет публичного IP-адреса. В этом случае узлы должны иметь доступ к registry и внешним сервисам через NAT Gateway, NAT-инстанс, proxy или другой egress-механизм. Для зон, в которых подсети отсутствуют, допустимо использовать значение `empty`.
   {% endalert %}

1. Закодируйте файлы `cloud-provider-cluster-configuration.yaml` и `cloud-provider-discovery-data.json` в Base64:

   ```shell
   export CLUSTER_CONFIGURATION_B64="$(base64 -w0 cloud-provider-cluster-configuration.yaml)"
   export DISCOVERY_DATA_B64="$(base64 -w0 cloud-provider-discovery-data.json)"
   ```

1. Создайте манифест с секретом `d8-provider-cluster-configuration` и ModuleConfig для модуля `cloud-provider-yandex`:

   ```shell
   cat > yandex-provider-secret-and-mc.yaml <<EOF
   apiVersion: v1
   kind: Secret
   metadata:
     labels:
       heritage: deckhouse
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

1. Скопируйте файл `yandex-provider-secret-and-mc.yaml` на master-узел кластера. Перед применением удалите объект ValidatingAdmissionPolicyBinding, если он запрещает создание объектов с лейблом `heritage: deckhouse`:

   ```shell
   d8 k delete validatingadmissionpolicybindings.admissionregistration.k8s.io \
     heritage-label-objects.deckhouse.io \
     --ignore-not-found
   ```

   Примените манифест:

   ```shell
   d8 k apply -f yandex-provider-secret-and-mc.yaml
   ```

1. Дождитесь включения модуля `cloud-provider-yandex` и появления ресурса YandexInstanceClass:

   ```shell
   d8 k get moduleconfig cloud-provider-yandex
   d8 k get crd yandexinstanceclasses.deckhouse.io
   d8 k -n d8-cloud-provider-yandex get pods -o wide
   ```

1. Создайте файл, например `yandex-instanceclass-nodegroup.yaml` с ресурсами YandexInstanceClass и NodeGroup:

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
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroup
   metadata:
     name: yc-worker
   spec:
     nodeType: Cloud
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
   - `nodeType: Cloud` означает, что узлы будут создаваться автоматически через облачного провайдера;
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

   ```console
   NAME                                 STATUS   ROLES                  AGE   VERSION    INTERNAL-IP
   static-master-0                      Ready    control-plane,master   1h    v1.33.10   10.128.0.15
   yc-worker-f3564dca-7fc59-s2w5d       Ready    yc-worker              10m   v1.33.10   10.128.0.21
   ```

1. Для диагностики состояния и поиска возможных проблем проверьте логи machine-controller-manager:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinedeployments.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager get machinesets.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager get machines.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

### Добавление вручную созданных узлов в Yandex Cloud через bootstrap-скрипт

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) включён и настроен.
- Компоненты модуля `cloud-provider-yandex` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-yandex get pods -o wide
  ```

- В Yandex Cloud создана виртуальная машина, которая будет подключена к кластеру.
- Виртуальная машина подключена к сети и подсети Yandex Cloud, используемым для гибридной интеграции с кластером.
- Внутренний IP-адрес виртуальной машины входит в диапазон адресов, используемый для облачных узлов Yandex Cloud.
- Имя виртуальной машины в Yandex Cloud совпадает с hostname внутри операционной системы.
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

1. На master-узле создайте файл, например `yandex-manual-nodegroup.yaml`, с ресурсом NodeGroup:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: yc-manual
   spec:
     nodeType: Hybrid
   ```

   {% alert level="info" %}
   Для вручную создаваемых узлов Yandex Cloud используется значение `nodeType: Hybrid`. В статусе NodeGroup такая группа может отображаться как `CloudStatic`.
   {% endalert %}

1. Примените манифест:

   ```shell
   d8 k apply -f yandex-manual-nodegroup.yaml
   ```

1. Убедитесь, что NodeGroup создана и синхронизирована:

   ```shell
   d8 k get nodegroup yc-manual
   d8 k describe nodegroup yc-manual
   ```

   Пример ожидаемого результата:

   ```console
   NAME        TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   yc-manual   CloudStatic   0       0       0                                                               1m    True
   ```

1. На master-узле получите bootstrap-скрипт для созданной NodeGroup:

   ```shell
   NODE_GROUP=yc-manual

   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} \
     -o jsonpath='{.data.bootstrap\.sh}' > ${NODE_GROUP}-bootstrap.b64
   ```

1. На master-узле проверьте, что файл содержит Base64-данные bootstrap-скрипта:

   ```shell
   head -c 80 ${NODE_GROUP}-bootstrap.b64
   echo
   base64 -d ${NODE_GROUP}-bootstrap.b64 | head -n 5
   ```

   В начале декодированного содержимого должен быть bash-скрипт:

   ```shell
   #!/bin/bash
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

   ```console
   NAME                    STATUS   ROLES       AGE   VERSION    INTERNAL-IP   EXTERNAL-IP
   static-master-0         Ready    master      1h    v1.33.10   10.128.0.15   <none>
   yandex-worker-hybrid    Ready    yc-manual   5m    v1.33.10   10.128.0.17   <PUBLIC_IP>
   ```

1. При сбоях подключения проверьте состояние NodeGroup, события и логи компонентов:

   ```shell
   d8 k get nodegroup yc-manual
   d8 k describe nodegroup yc-manual
   d8 k describe node yandex-worker-hybrid
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

### Добавление вручную созданных узлов в Yandex Cloud через CAPS

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) включён и настроен.
- Компоненты модуля `cloud-provider-yandex` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-yandex get pods -o wide
  ```

- В Yandex Cloud создана виртуальная машина, которая будет подключена к кластеру.
- Виртуальная машина подключена к сети и подсети Yandex Cloud, используемым для гибридной интеграции с кластером.
- Внутренний IP-адрес виртуальной машины входит в диапазон адресов, используемый для облачных узлов Yandex Cloud.
- Имя виртуальной машины в Yandex Cloud совпадает с hostname внутри операционной системы.
- На виртуальной машине установлены необходимые базовые пакеты для поддерживаемой ОС. Для РЕД ОС заранее установите `which` и пакетный менеджер, если они отсутствуют.

1. На master-узле задайте переменные для создаваемой NodeGroup и подключаемой виртуальной машины:

   ```shell
   export NODE_GROUP="yc-caps"
   export NODE_NAME="yandex-worker-hybrid-caps"
   export NODE_SSH_IP="<NODE_PUBLIC_OR_INTERNAL_IP>"
   export CAPS_USER="caps"
   ```

   Где:

   - `NODE_GROUP` — имя NodeGroup, в которую будет добавлен узел;
   - `NODE_NAME` — имя подключаемого узла;
   - `NODE_SSH_IP` — IP-адрес виртуальной машины, доступный по SSH;
   - `CAPS_USER` — пользователь, под которым CAPS будет подключаться к виртуальной машине.

1. На master-узле создайте NodeGroup:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: ${NODE_GROUP}
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: ${NODE_GROUP}
   EOF
   ```

   В этом сценарии используется `nodeType: Static`, потому что узел уже создан вручную, а CAPS будет только подключать и настраивать его по SSH.

1. Убедитесь, что NodeGroup создана и синхронизирована:

   ```shell
   d8 k get nodegroup ${NODE_GROUP}
   d8 k describe nodegroup ${NODE_GROUP}
   ```

   Пример ожидаемого результата:

   ```console
   NAME      TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   yc-caps   Static   0       0       0                                                               1m    True
   ```

1. На master-узле сгенерируйте SSH-ключ, который CAPS будет использовать для подключения к виртуальной машине:

   ```shell
   ssh-keygen -t ed25519 \
     -f /dev/shm/${NODE_GROUP}-id \
     -C "" \
     -N ""
   ```

   {% alert level="info" %}
   Ключ создаётся с пустой парольной фразой, так как CAPS должен использовать его автоматически.
   {% endalert %}

1. На master-узле создайте ресурс [SSHCredentials](/modules/node-manager/cr.html#sshcredentials):

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha2
   kind: SSHCredentials
   metadata:
     name: ${NODE_GROUP}
   spec:
     user: ${CAPS_USER}
     privateSSHKey: "$(base64 -w0 /dev/shm/${NODE_GROUP}-id)"
   EOF
   ```

   Ресурс SSHCredentials хранит имя пользователя и приватный SSH-ключ, с помощью которых CAPS будет подключаться к виртуальной машине.

1. Убедитесь, что ресурс SSHCredentials создан:

   ```shell
   d8 k get sshcredentials
   d8 k describe sshcredentials ${NODE_GROUP}
   ```

1. На master-узле выведите публичную часть SSH-ключа:

   ```shell
   cat /dev/shm/${NODE_GROUP}-id.pub
   ```

   Этот ключ понадобится на следующем шаге для настройки пользователя на подключаемой виртуальной машине.

1. На подключаемой виртуальной машине создайте пользователя, под которым CAPS будет выполнять настройку узла. Выполните команды на подключаемой виртуальной машине, указав публичный SSH-ключ, полученный на предыдущем шаге:

   ```shell
   export CAPS_USER="caps"
   export KEY='<SSH_PUBLIC_KEY>'

   useradd -m -s /bin/bash ${CAPS_USER}
   usermod -aG sudo ${CAPS_USER}

   echo "${CAPS_USER} ALL=(ALL) NOPASSWD: ALL" | EDITOR='tee -a' visudo

   mkdir -p /home/${CAPS_USER}/.ssh
   echo "${KEY}" > /home/${CAPS_USER}/.ssh/authorized_keys

   chown -R ${CAPS_USER}:${CAPS_USER} /home/${CAPS_USER}
   chmod 700 /home/${CAPS_USER}/.ssh
   chmod 600 /home/${CAPS_USER}/.ssh/authorized_keys
   ```

   {% alert level="info" %}
   Значение `KEY` необходимо указывать в кавычках, так как публичный SSH-ключ содержит пробелы.
   {% endalert %}

   {% alert level="info" %}
   Для операционных систем семейства Astra Linux при использовании модуля мандатного контроля целостности Parsec дополнительно задайте максимальный уровень целостности для пользователя:

   ```shell
   pdpl-user -i 63 ${CAPS_USER}
   ```

   {% endalert %}

1. На master-узле проверьте, что CAPS-пользователь может подключиться к виртуальной машине по SSH и выполнять команды через `sudo` без пароля:

   ```shell
   ssh -i /dev/shm/${NODE_GROUP}-id ${CAPS_USER}@${NODE_SSH_IP} \
     'hostname; sudo -n true; echo OK'
   ```

   В выводе должно быть имя узла и строка `OK`.

1. На master-узле создайте ресурс [StaticInstance](/modules/node-manager/cr.html#staticinstance) для подключаемой виртуальной машины:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha2
   kind: StaticInstance
   metadata:
     name: ${NODE_NAME}
     labels:
       role: ${NODE_GROUP}
   spec:
     address: "${NODE_SSH_IP}"
     credentialsRef:
       kind: SSHCredentials
       name: ${NODE_GROUP}
   EOF
   ```

   Где:

   - `metadata.name` — имя подключаемого узла;
   - `metadata.labels.role` — лейбл, по которому NodeGroup выбирает этот StaticInstance;
   - `spec.address` — IP-адрес виртуальной машины, доступный по SSH;
   - `spec.credentialsRef.name` — имя созданного ранее ресурса SSHCredentials.

1. Проверьте состояние StaticInstance:

   ```shell
   d8 k get staticinstances
   d8 k describe staticinstance ${NODE_NAME}
   ```

1. Дождитесь подключения узла и проверьте его состояние:

   ```shell
   d8 k get nodes -o wide
   ```

   Пример ожидаемого результата:

   ```console
   NAME                         STATUS   ROLES     AGE   VERSION    INTERNAL-IP   EXTERNAL-IP
   static-master-0              Ready    master    1h    v1.33.10   10.128.0.15   <none>
   yandex-worker-hybrid-caps    Ready    yc-caps   5m    v1.33.10   10.128.0.29   <none>
   ```

1. При сбоях подключения проверьте состояние NodeGroup, StaticInstance, Machine и события в кластере:

   ```shell
   d8 k get nodegroup ${NODE_GROUP}
   d8 k describe nodegroup ${NODE_GROUP}

   d8 k get staticinstances
   d8 k describe staticinstance ${NODE_NAME}

   d8 k -n d8-cloud-instance-manager get machines,machinesets,machinedeployments -o wide
   d8 k get events -A --sort-by=.lastTimestamp | tail -n 100
   ```

## Гибридный кластер с VCD

Далее описан процесс создания гибридного кластера, объединяющего статические (bare-metal) узлы и облачные узлы в VMware vCloud Director (VCD) с использованием Deckhouse Kubernetes Platform (DKP).

### Предварительные требования для VCD

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и сетью виртуальных машин в VCD настроена сетевая связность.
- Узлы, создаваемые в VCD, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [Настройка сетевых политик](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [Подключение и авторизация в VMware vCloud Director](../virtualization/vcd/connection-and-authorization.html):
  - настроен тенант в VCD с выделенными ресурсами;
  - подготовлена учётная запись VCD со статичным паролем и правами администратора;
  - настроена рабочая сеть в VCD с включённым DHCP-сервером;
  - подготовлены необходимые ресурсы VCD: VDC, vApp, шаблоны, политики и другие параметры.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

### Добавление автоматически создаваемых узлов в VCD

1. Создайте файл, например, `cloud-provider-vcd-mc.yaml` с ресурсом ModuleConfig:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-vcd
   spec:
     version: 1
     enabled: true
     settings:
       mainNetwork: <NETWORK_NAME>
       organization: <ORGANIZATION>
       virtualDataCenter: <VDC_NAME>
       virtualApplicationName: <VAPP_NAME>
       sshPublicKey: <SSH_PUBLIC_KEY>
       provider:
         server: <API_URL>
         username: <USER_NAME>
         password: <PASSWORD>
         insecure: false
   ```

   Где:
   - `mainNetwork` — имя сети, в которой будут размещаться облачные узлы в VCD;
   - `organization` — имя Organization в VCD;
   - `virtualDataCenter` — имя Virtual Data Center в VCD;
   - `virtualApplicationName` — имя vApp, где будут создаваться узлы, например dkp-vcd-app;
   - `sshPublicKey` — публичный SSH-ключ для доступа к узлам;
   - `provider.server` — URL-адрес API VCD;
   - `provider.username` — имя пользователя VCD;
   - `provider.password` — пароль пользователя VCD;
   - `provider.insecure` — установите значение true, если VCD использует самоподписанный TLS-сертификат.

   Если для аутентификации используется токен, вместо `username` и `password` укажите `apiToken`:

   ```yaml
   provider:
     server: <API_URL>
     apiToken: <API_TOKEN>
     username: ""
     password: ""
     insecure: false
   ```

1. Примените ModuleConfig:

   ```shell
   d8 k apply -f cloud-provider-vcd-mc.yaml
   d8 k get mc cloud-provider-vcd
   ```

1. Убедитесь, что все поды в пространстве имён `d8-cloud-provider-vcd` находятся в состоянии `Running`:

   ```shell
   d8 k get pods -n d8-cloud-provider-vcd
   ```

1. Убедитесь, что в кластере созданы StorageClass для VCD:

   ```shell
   d8 k get sc
   ```

1. Создайте файл, например, `vcd-instanceclass-nodegroup.yaml` с ресурсами [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup):

   ```yaml
   ---
   apiVersion: deckhouse.io/v1
   kind: VCDInstanceClass
   metadata:
     name: worker
   spec:
     rootDiskSizeGb: 50
     sizingPolicy: <SIZING_POLICY>
     storageProfile: <STORAGE_PROFILE>
     template: <VAPP_TEMPLATE>
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: VCDInstanceClass
         name: worker
       maxPerZone: 2
       minPerZone: 1
     nodeTemplate:
       labels:
         node-role/worker: ""
    ```

1. Примените манифест:

   ```shell
   d8 k apply -f vcd-instanceclass-nodegroup.yaml
   ```

   После применения манифеста DKP начнёт создавать виртуальные машины в VCD, управляемые модулем `node-manager`.

1. Убедитесь, что в кластере появилось требуемое количество узлов:

   ```shell
   d8 k get nodes -o wide
   ```

1. При сбоях создания ВМ проверьте объекты Machine, MachineSet и логи machine-controller-manager:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

### Добавление вручную созданных узлов в VCD через bootstrap-скрипт

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-vcd`](/modules/cloud-provider-vcd/) включён и настроен.
- Компоненты модуля `cloud-provider-vcd` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-vcd get pods -o wide
  ```

- В кластере созданы StorageClass для VCD:
  
  ```shell
  d8 k get sc
  ```

- В VCD создана виртуальная машина, которая будет подключена к кластеру.
- Имя виртуальной машины в VCD совпадает с hostname внутри операционной системы.
- В дополнительных параметрах ВМ в VCD задано значение:

  ```text
  disk.EnableUUID = 1
  ```

- Виртуальная машина подключена к сети, указанной в параметре [`mainNetwork`](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-mainnetwork) модуля `cloud-provider-vcd`.
- На виртуальной машине установлены необходимые базовые пакеты для поддерживаемой ОС. Для РЕД ОС заранее установите `which` и пакетный менеджер, если они отсутствуют.

1. Создайте файл, например `cloud-static-nodegroup.yaml`, с ресурсом NodeGroup и типом узлов CloudStatic:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: cloud-static
   spec:
     nodeType: CloudStatic
   ```

1. Примените манифест:

   ```shell
   d8 k apply -f cloud-static-nodegroup.yaml
   ```

1. Убедитесь, что NodeGroup создана и синхронизирована:

   ```shell
   d8 k get nodegroup cloud-static
   ```

   Пример ожидаемого результата:

   ```console
   NAME           TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   cloud-static   CloudStatic   0       0       0                                                               1m    True
   ```

1. Получите bootstrap-скрипт для созданной NodeGroup:

   ```shell
   NODE_GROUP=cloud-static

   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} \
     -o jsonpath='{.data.bootstrap\.sh}' > bootstrap.b64
   ```

1. Скопируйте bootstrap-скрипт на подключаемую виртуальную машину:

   ```shell
   scp bootstrap.b64 <USER>@<NODE_IP>:/tmp/bootstrap.b64
   ```

1. Подключитесь к виртуальной машине по SSH:

   ```shell
   ssh <USER>@<NODE_IP>
   ```

1. На виртуальной машине наначьте права и запустите bootstrap-скрипт:

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

   ```console
   NAME                       STATUS   ROLES          AGE   VERSION    INTERNAL-IP
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.138
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.151
   ```

## Гибридный кластер с vSphere

Далее описан процесс создания гибридного кластера, объединяющего статические (bare-metal) узлы и облачные узлы в vSphere с использованием Deckhouse Kubernetes Platform (DKP).

В этом сценарии исходный кластер DKP уже развёрнут как статический кластер. Control-plane остаётся на статических узлах, а новые worker-узлы создаются во vSphere через модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/).

Параметры виртуальных машин задаются ресурсом [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), а количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом узлов [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html).

### Предварительные требования для vSphere

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и сетью виртуальных машин во vSphere настроена сетевая связность.
- Узлы, создаваемые во vSphere, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [Настройка сетевых политик](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [Подключение и авторизация в VMware vSphere](../virtualization/vsphere/authorization.html):
  - настроен доступ к vCenter;
  - подготовлена учётная запись vSphere с необходимыми привилегиями;
  - подготовлен шаблон виртуальной машины;
  - настроены сети, Datastore, теги регионов и зон.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

### Добавление автоматически создаваемых узлов в vSphere

Для подключения уже работающего статического кластера к vCenter используйте ресурс [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/).

В параметре `spec.settings` укажите параметры доступа к vCenter, сетевые настройки, теги региона и зоны, а также SSH-ключи, которые будут добавлены на создаваемые виртуальные машины.

Пример конфигурации и описание доступных параметров приведены в [примерах модуля](/modules/cloud-provider-vsphere/examples.html) и в разделе [Конфигурация модуля `cloud-provider-vsphere`](/modules/cloud-provider-vsphere/configuration.html).

1. Создайте файл, например `vsphere-mc.yaml`, с ModuleConfig для модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-vsphere
   spec:
     version: 2
     enabled: true
     settings:
       host: "<VCENTER_FQDN>"
       username: "<USERNAME@DOMAIN.LOCAL>"
       password: "<PASSWORD>"
       insecure: true
       vmFolderPath: "<FOLDER_PATH_UNDER_DATACENTER>"
       regionTagCategory: "<TAG_CATEGORY_FOR_REGION>"
       zoneTagCategory: "<TAG_CATEGORY_FOR_ZONE>"
       region: "<REGION_TAG_NAME_ON_DATACENTER>"
       zones:
         - "<ZONE_TAG_NAME_ON_CLUSTER>"
       internalNetworkNames:
         - "<PORT_GROUP_NAME_FOR_INTERNAL_IP>"
       sshKeys:
         - "<SSH_PUBLIC_KEY_ONE_LINE>"
   ```

   Значения параметров:

   - `host` — адрес vCenter;
   - `username`, `password` — учётные данные пользователя vSphere;
   - `insecure` — отключение проверки TLS-сертификата vCenter;
   - `vmFolderPath` — папка, в которой будут создаваться виртуальные машины;
   - `regionTagCategory`, `zoneTagCategory` — категории тегов региона и зоны;
   - `region` — тег региона;
   - `zones` — список зон, в которых можно создавать узлы;
   - `internalNetworkNames` — список сетей vSphere для подключения создаваемых узлов;
   - `sshKeys` — публичные SSH-ключи, которые будут добавлены на создаваемые виртуальные машины.

1. Примените конфигурацию модуля:

   ```shell
   d8 k apply -f vsphere-mc.yaml
   ```

1. Дождитесь готовности модуля `cloud-provider-vsphere`:

   ```shell
   d8 k get moduleconfig cloud-provider-vsphere
   d8 k get pods -n d8-cloud-provider-vsphere -o wide
   ```

1. Создайте файл, например `vsphere-instance.yaml`, c ресурсами [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup) со значением `nodeType: CloudEphemeral`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VsphereInstanceClass
   metadata:
     name: ephemeral
   spec:
     numCPUs: 2
     memory: 4096
     rootDiskSize: 40
     template: "<PATH_TO_TEMPLATE_FROM_DATACENTER>"
     mainNetwork: "<PORT_GROUP_NAME>"
     datastore: "<DATASTORE_OR_FOLDER/DATASTORE>"
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: ephemeral
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: VsphereInstanceClass
         name: ephemeral
       maxPerZone: 1
       minPerZone: 1
       zones:
         - "<ZONE_TAG_NAME_ON_CLUSTER>"
     disruptions:
       approvalMode: Automatic
   ```

   Где:

   - [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) описывает параметры виртуальной машины, которая будет создана во vSphere;
   - [NodeGroup](/modules/node-manager/cr.html#nodegroup) описывает группу узлов, которую DKP должен поддерживать в кластере;
   - `nodeType: CloudEphemeral` означает, что узлы будут создаваться автоматически через облачный провайдер;
   - `cloudInstances.classReference` указывает на VsphereInstanceClass;
   - `cloudInstances.zones` должен содержать зоны из списка `zones` в ModuleConfig.

1. Примените манифест:

   ```shell
   d8 k apply -f vsphere-instance.yaml
   ```

   После применения манифеста DKP начнёт создавать виртуальную машину во vSphere. После загрузки ВМ kubelet подключится к Kubernetes API, и новый узел появится в кластере.

1. Проверьте состояние узлов:

   ```shell
   d8 k get nodes -o wide
   ```

   Пример ожидаемого результата:

   ```console
   NAME                             STATUS   ROLES                  AGE   VERSION
   static-master-0                  Ready    control-plane,master   1h    v1.33.10
   ephemeral-1ca02a5b-7588b-k89dc   Ready    ephemeral              10m   v1.33.10
   ```

1. При сбоях создания ВМ проверьте объекты Machine, MachineSet и логи machine-controller-manager:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

   Также проверьте события в кластере:

   ```shell
   d8 k get events -A --sort-by=.lastTimestamp | tail -n 100
   ```

### Добавление вручную созданных узлов в vSphere через bootstrap-скрипт

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) включён и настроен.
- Компоненты модуля `cloud-provider-vsphere` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-vsphere get pods -o wide
  ```

- В кластере созданы StorageClass для vSphere:
  
  ```shell
  d8 k get sc
  ```

- В vSphere создана виртуальная машина, которая будет подключена к кластеру.
- Имя виртуальной машины в vSphere, значение local-hostname в метаданных и hostname внутри операционной системы совпадают.
- В дополнительных параметрах ВМ в vSphere заданы параметры:

  ```text
  disk.EnableUUID = TRUE
  guestinfo.metadata = <BASE64_ENCODED_METADATA>
  guestinfo.metadata.encoding = base64
  ```

  Параметр `guestinfo.metadata` должен содержать конфигурацию метаданных, закодированных в Base64. Пример файла `metadata.json`:

  ```json
  {
     "instance-id": "cloud-static-worker-0",
     "local-hostname": "cloud-static-worker-0",
     "public-keys-data": "<SSH_PUBLIC_KEY>",
     "network": {
       "version": 2,
       "ethernets": {
         "id0": {
           "match": {
             "driver": "vmxnet3"
           },
           "set-name": "ens192",
           "dhcp4": true
         }
       }
     }
   }
  ```

  Где:

  - `instance-id` — идентификатор виртуальной машины;
  - `local-hostname` — hostname узла внутри операционной системы;
  - `public-keys-data` — публичный SSH-ключ для доступа к виртуальной машине;
  - `network` — сетевые настройки, которые будут применены внутри виртуальной машины.

  Чтобы получить значение для параметра `guestinfo.metadata`, выполните:

  ```shell
  METADATA_B64="$(base64 -w0 metadata.json)"
  echo "$METADATA_B64"
  ```

- Виртуальная машина подключена к сети, указанной в параметре [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) конфигурации модуля `cloud-provider-vsphere`.
- На виртуальной машине установлены необходимые базовые пакеты для поддерживаемой ОС. Для РЕД ОС заранее установите `which` и пакетный менеджер, если они отсутствуют.

1. Создайте файл, например `cloud-static-nodegroup.yaml`, с ресурсом NodeGroup и типом узлов CloudStatic:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: cloud-static
   spec:
     nodeType: CloudStatic
   ```

1. Примените манифест:

   ```shell
   d8 k apply -f cloud-static-nodegroup.yaml
   ```

1. Убедитесь, что NodeGroup создана и синхронизирована:

   ```shell
   d8 k get nodegroup cloud-static
   ```

   Пример ожидаемого результата:

   ```console
   NAME           TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   cloud-static   CloudStatic   0       0       0                                                               1m    True
   ```

1. Получите bootstrap-скрипт для созданной NodeGroup:

   ```shell
   NODE_GROUP=cloud-static

   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} \
     -o jsonpath='{.data.bootstrap\.sh}' > bootstrap.b64
   ```

1. Скопируйте bootstrap-скрипт на подключаемую виртуальную машину:

   ```shell
   scp bootstrap.b64 <USER>@<NODE_IP>:/tmp/bootstrap.b64
   ```

1. Подключитесь к виртуальной машине по SSH:

   ```shell
   ssh <USER>@<NODE_IP>
   ```

1. На виртуальной машине назначьте права и запустите bootstrap-скрипт:

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

   ```console
   NAME                       STATUS   ROLES          AGE   VERSION    INTERNAL-IP
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.135
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.152
   ```
