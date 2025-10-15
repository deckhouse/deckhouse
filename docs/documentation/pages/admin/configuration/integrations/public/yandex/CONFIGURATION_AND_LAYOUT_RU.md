---
title: Схемы размещения и настройка
permalink: ru/admin/integrations/public/yandex/layout.html
lang: ru
---

## Схемы размещения

Данный раздел описывает возможные схемы размещения узлов кластера в инфраструктуре Yandex Cloud и связанные с ними настройки. От выбора схемы (layout) зависят принципы сетевого взаимодействия, наличие публичных IP-адресов, маршрутизация исходящего трафика и способ подключения к узлам.

### Standard

{% alert level="danger" %}
В данной схеме размещения узлы не будут иметь публичных IP-адресов и будут выходить в интернет через NAT-шлюз (NAT Gateway) Yandex Cloud. NAT-шлюз (NAT Gateway) использует случайные публичные IP-адреса из [выделенных диапазонов](https://yandex.cloud/ru/docs/overview/concepts/public-ips#virtual-private-cloud). Из-за этого невозможно добавить в белый список (whitelist) адреса облачных ресурсов, находящихся за конкретным NAT-шлюзом, на стороне других сервисов.
{% endalert %}

![Схема размещения Standard в Yandex Cloud](../../../../images/cloud-provider-yandex/yandex-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10422&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: <EXISTING_NETWORK_ID>
provider:
  cloudID: <CLOUD_ID>
  folderID: <FOLDER_ID>
  serviceAccountJSON: |
    {
    "id": "id",
    "service_account_id": "service_account_id",
    "key_algorithm": "RSA_2048",
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIwID....AQAB\n-----END PUBLIC KEY-----\n",
    "private_key": "-----BEGIN PRIVATE KEY-----\nMIIE....1ZPJeBLt+\n-----END PRIVATE KEY-----\n"
    }
masterNodeGroup:
  replicas: 3
  zones:
  - ru-central1-a
  - ru-central1-b
  - ru-central1-d
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    externalIPAddresses:
    - "<ZONE_A_EXTERNAL_IP_MASTER_1>"
    - "Auto"
    - "Auto"
    externalSubnetIDs:
    - <ZONE_A_SUBNET_ID>
    - <ZONE_B_SUBNET_ID>
    - <ZONE_D_SUBNET_ID>
    additionalLabels:
      takes: priority
nodeGroups:
- name: worker
  replicas: 2
  zones:
  - ru-central1-a
  - ru-central1-b
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    coreFraction: 50
    externalIPAddresses:
    - "Auto"
    - "Auto"
    externalSubnetIDs:
    - <ZONE_A_SUBNET_ID>
    - <ZONE_B_SUBNET_ID>
    additionalLabels:
      role: example
labels:
  billing: prod
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - <DNS_SERVER_1>
  - <DNS_SERVER_2>
```

### WithoutNAT

В данной схеме размещения NAT (любого вида) не используется, а каждому узлу выдается публичный IP-адрес.

{% alert level="warning" %}
В DKP нет поддержки групп безопасности (security group), поэтому все узлы кластера будут доступны без ограничения подключения.
{% endalert %}

![Схема размещения WithoutNAT в Yandex Cloud](../../../../images/cloud-provider-yandex/yandex-withoutnat.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10557&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
provider:
  cloudID: <CLOUD_ID>
  folderID: <FOLDER_ID>
  serviceAccountJSON: |
    {
    "id": "id",
    "service_account_id": "service_account_id",
    "key_algorithm": "RSA_2048",
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIwID....AQAB\n-----END PUBLIC KEY-----\n",
    "private_key": "-----BEGIN PRIVATE KEY-----\nMIIE....1ZPJeBLt+\n-----END PRIVATE KEY-----\n"
    }    
masterNodeGroup:
  replicas: 3
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    externalIPAddresses:
    - "Auto"
    - "Auto"
    - "Auto"
    externalSubnetIDs:
    - <ZONE_A_SUBNET_ID>
    - <ZONE_B_SUBNET_ID>
    - <ZONE_D_SUBNET_ID>
    zones:
    - ru-central1-a
    - ru-central1-b
    - ru-central1-d
nodeGroups:
- name: worker
  replicas: 2
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    coreFraction: 50
    externalIPAddresses:
    - "<ZONE_A_EXTERNAL_IP_WORKER_1>"
    - "Auto"
    externalSubnetIDs:
    - <ZONE_A_SUBNET_ID>
    - <ZONE_B_SUBNET_ID>
    zones:
    - ru-central1-a
    - ru-central1-b
sshPublicKey: "<SSH_PUBLIC_KEY>"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: <EXISTING_NETWORK_ID>
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - <DNS_SERVER_1>
  - <DNS_SERVER_2>
```

### WithNATInstance

В данной схеме размещения в отдельной подсети создается NAT-инстанс, а в таблицу маршрутизации подсетей зон добавляется правило с маршрутом на `0.0.0.0/0` с NAT-инстансом в качестве nexthop'а.
Подсеть выделяется для предотвращения петли маршрутизации и не должна пересекаться с другими сетями, используемыми в кластере.

Для размещения NAT-инстанса в существующей подсети используйте [параметр `withNATInstance.internalSubnetID`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-withnatinstance-internalsubnetid) — инстанс будет создан в зоне, соответствующей этой подсети.

Если необходимо создать новую подсеть, укажите [параметр `withNATInstance.internalSubnetCIDR`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-withnatinstance-internalsubnetcidr) — в ней будет размещён NAT-инстанс.

> Обязателен один из параметров: `withNATInstance.internalSubnetID` или `withNATInstance.internalSubnetCIDR`.

Если также указан [`withNATInstance.externalSubnetID`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-withnatinstance-externalsubnetid), NAT-инстанс будет подключён к этой подсети через вторичный сетевой интерфейс.

![Схема размещения WithNATInstance в Yandex Cloud](../../../../images/cloud-provider-yandex/yandex-withnatinstance.png)
<!--- Исходник: https://docs.google.com/drawings/d/1oVpZ_ldcuNxPnGCkx0dRtcAdL7BSEEvmsvbG8Aif1pE/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithNATInstance
withNATInstance:
  natInstanceExternalAddress: <NAT_INSTANCE_EXTERNAL_ADDRESS>
  internalSubnetID: <INTERNAL_SUBNET_ID>
  externalSubnetID: <EXTERNAL_SUBNET_ID>
provider:
  cloudID: <CLOUD_ID>
  folderID: <FOLDER_ID>
  serviceAccountJSON: |
    {
    "id": "id",
    "service_account_id": "service_account_id",
    "key_algorithm": "RSA_2048",
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIwID....AQAB\n-----END PUBLIC KEY-----\n",
    "private_key": "-----BEGIN PRIVATE KEY-----\nMIIE....1ZPJeBLt+\n-----END PRIVATE KEY-----\n"
    }    
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    externalIPAddresses:
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    zones:
    - ru-central1-a
nodeGroups:
- name: worker
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    coreFraction: 50
    externalIPAddresses:
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    zones:
    - ru-central1-a
sshPublicKey: "<SSH_PUBLIC_KEY>"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: <EXISTING_NETWORK_ID>
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - <DNS_SERVER_1>
  - <DNS_SERVER_2>
```

## Назначение YandexClusterConfiguration

Для интеграции Deckhouse Kubernetes Platform с Yandex Cloud необходимо описать инфраструктуру кластера с помощью ресурса YandexClusterConfiguration.

[YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) — это объект Custom Resource (CR), описывающий параметры интеграции с облаком Yandex Cloud. Он используется DKP для:

- размещения master и worker-узлов в облаке;
- задания схемы сетевого взаимодействия;
- подключения к API Yandex Cloud с использованием авторизационного JSON-ключа;
- настройки подсетей, публичных IP, ресурсов виртуальных машин и др.

Обязательные поля:

- `apiVersion` — должен быть `deckhouse.io/v1`;
- `kind` — всегда `YandexClusterConfiguration`.

Пример заголовка ресурса:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
```

Чтобы отредактировать этот ресурс в работающем кластере, выполните команду:

```shell
d8 platform edit provider-cluster-configuration
```

После внесения изменений их необходимо применить с помощью команды:

```shell
dhctl converge
```

### Пример конфигурации

Ниже приведён пример минимальной конфигурации ресурса YandexClusterConfiguration, описывающей кластер с одной master-группой и одной группой рабочих узлов. Конфигурация использует схему размещения Standard, заданы базовые параметры вычислительных ресурсов, публичный ключ SSH, а также идентификаторы облака и каталога:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
nodeNetworkCIDR: 127.0.0.1/8
labels:
  label-2: b
sshPublicKey: "<SSH_PUBLIC_KEY>"
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd8nb7ecsbvj76dfaa8b
nodeGroups:
- name: worker
  replicas: 1
  zones:
  - ru-central1-a
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd8nb7ecsbvj76dfaa8b
    coreFraction: 50
    externalIPAddresses:
    - 198.51.100.5
    - Auto
provider:
  cloudID: "<CLOUD_ID>"
  folderID: "<FOLDER_ID>"
  serviceAccountJSON: |
    {
    "id": "id",
    "service_account_id": "service_account_id",
    "key_algorithm": "RSA_2048",
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIwID....AQAB\n-----END PUBLIC KEY-----\n",
    "private_key": "-----BEGIN PRIVATE KEY-----\nMIIE....1ZPJeBLt+\n-----END PRIVATE KEY-----\n"
    }
```

- Master-группа состоит из одного узла без явного указания зоны.
- Worker-группа содержит один узел, размещённый в зоне `ru-central1-a`, с двумя внешними IP-адресами: один задан вручную (`198.51.100.5`), второй заказан автоматически (`Auto`).
- Указан ключ `serviceAccountJSON`, необходимый для подключения к API Yandex Cloud.
- Используется CIDR-подсеть `127.0.0.1/8` и добавлен label `label-2: b` на уровне кластера.

## Сетевые параметры и безопасность

Далее описаны настройки, связанные с адресацией, маршрутизацией, внешним трафиком и безопасностью сети в кластере Deckhouse Kubernetes Platform, развернутом в Yandex Cloud.

### Внутренняя адресация узлов кластера

[Параметр `nodeNetworkCIDR`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodenetworkcidr) используется для задания диапазона IP-адресов, который будет распределён между зонами доступности и применён к внутренним интерфейсам узлов.

```yaml
nodeNetworkCIDR: 192.168.12.13/24
```

В зависимости от выбранной схемы размещения (Standard или WithNATInstance), эта подсеть будет автоматически разделена на три равные части для каждой зоны размещения:

- ru-central1-a
- ru-central1-b
- ru-central1-d

Каждая из этих частей будет использоваться как отдельная внутренняя подсеть (subnet), к которой будут подключены узлы, создаваемые в соответствующей зоне.

{% alert level="info" %}
В случае, если вы планируете использовать одну и ту же подсеть в нескольких кластерах (например, с [`cni-simple-bridge`](/modules/cni-simple-bridge/)), необходимо учитывать ограничения: один кластер = одна таблица маршрутизации = одна подсеть. Разворачивание двух кластеров в одних и тех же подсетях с `cni-simple-bridge` невозможно. Если требуется использовать одинаковые подсети — используйте [`cni-cilium`](/modules/cni-cilium/).
{% endalert %}

### Назначение внешних IP-адресов и исходящего трафика

Параметр `externalSubnetIDs` указывается в секциях [`masterNodeGroup.instanceClass`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-externalsubnetids) и [`nodeGroups.instanceClass`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-externalsubnetids) и представляет собой массив идентификаторов подсетей Yandex Cloud, в которые будут подключены внешние сетевые интерфейсы узлов. Этот параметр обязателен, если требуется:

- Назначение публичного IP-адреса;
- Определение маршрута по умолчанию для исходящего трафика с узлов;
- Использование значений `"Auto"` в поле `externalIPAddresses`.

Пример:

```yaml
externalSubnetIDs:
  - <RU-CENTRAL1-A-SUBNET-ID>
  - <RU-CENTRAL1-B-SUBNET-ID>
  - <RU-CENTRAL1-D-SUBNET-ID>
```

{% alert level="info" %}
Параметр `externalSubnetIDs` обязателен для корректной работы автоматического назначения публичных IP через `externalIPAddresses: ["Auto", ...]`.
{% endalert %}

### Настройка DNS и DHCP для внутренних сетей

[Параметр `dhcpOptions`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-dhcpoptions) позволяет задать настройки DHCP-сервера, которые будут применены ко всем подсетям, создаваемым в рамках кластера Deckhouse Kubernetes Platform в Yandex Cloud.

Доступные поля:

- `domainName` — поисковый домен (search domain), который будет установлен в конфигурации сети.
- `domainNameServers` — массив IP-адресов DNS-серверов, которые будут использоваться как рекурсивные резолверы.

Пример:

```yaml
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 192.168.0.2
  - 192.168.0.3
```

{% alert level="info" %}
Указанные DNS-серверы в `domainNameServers` обязательно должны разрешать внешние и внутренние доменные зоны, используемые в кластере. В противном случае возможны сбои в работе сервисов.
{% endalert %}

После внесения изменений в `dhcpOptions`:

- Принудительно обновите `DHCP lease` на всех виртуальных машинах;
- Перезапустите все поды с `hostNetwork: true` — в частности, `kube-dns`, чтобы пересчитать содержимое `resolv.conf`.

Для применения изменений можно использовать:

```shell
netplan apply
```

или другой способ, соответствующий вашей системе (например, `systemd-networkd`, `dhclient` и т.д.).

### Использование заранее созданных подсетей

[Параметр `existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) позволяет указать соответствия между зонами доступности и ранее созданными подсетями в Yandex Cloud. Это особенно важно, если вы не хотите, чтобы DKP автоматически создавал подсети, а хотите использовать существующие.

Пример использования:

```yaml
existingZoneToSubnetIDMap:
  ru-central1-a: e2lu8r1tbbtryhdpa9ro
  ru-central1-b: e2lu8r1tbbtryhdpa9ro
  ru-central1-d: e2lu8r1tbbtryhdpa9ro
```

{% alert level="info" %}
DKP автоматически создаёт таблицу маршрутизации и не привязывает её к подсетям — это необходимо сделать вручную через интерфейс Yandex Cloud.
{% endalert %}

### Дополнительные внешние сети

DKP позволяет явно указать список дополнительных внешних сетей, IP-адреса из которых будут интерпретироваться как публичные (External IP). Это задаётся в [параметре `settings.additionalExternalNetworkIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-additionalexternalnetworkids) в ресурсе ModuleConfig.

Эта настройка полезна, если:

- имеются внешние подсети, не указанные в `externalSubnetIDs`, но требующие учёта как внешние;
- требуется точный контроль над интерпретацией IP-адресов при настройке балансировки, маршрутизации и экспорта статуса;
- используются нестандартные схемы подключения, например через внешние NAT-шлюзы или ручное резервирование IP.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  enabled: true
  settings:
    additionalExternalNetworkIDs:
      - enp6t4sno
```

Если параметр не задан, DKP будет использовать только те подсети, что явно указаны в YandexClusterConfiguration (например, через `externalSubnetIDs`), чтобы определять публичность IP.

## Настройка групп безопасности в Yandex Cloud

При создании [облачной сети](https://cloud.yandex.ru/ru/docs/vpc/concepts/network#network), Yandex Cloud добавляет [группу безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups) по умолчанию для всех подключенных сетей, включая сеть кластера Deckhouse Kubernetes Platform. Эта группа безопасности по умолчанию содержит правила разрешающие любой входящий и исходящий трафик и применяется для всех подсетей облачной сети, если на объект (интерфейс ВМ) явно не назначена другая группа безопасности.

{% alert level="danger" %}
Не удаляйте правила по умолчанию, разрешающие любой трафик, до того как закончите настройку правил группы безопасности. Это может нарушить работоспособность кластера.
{% endalert %}

Ниже приведены общие рекомендации по настройке групп безопасности. Некорректная настройка групп безопасности может сказаться на работоспособности кластера. Ознакомьтесь с [особенностями работы групп безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups#security-groups-notes) в Yandex Cloud перед использованием в продуктивных средах.

1. Определите облачную сеть, в которой работает кластер Deckhouse Kubernetes Platform.

   Название сети совпадает с полем `prefix` ресурса [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration). Его можно узнать с помощью команды:

   ```bash
   d8 k get secrets -n kube-system d8-cluster-configuration -ojson | \
     jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix | cut -d: -f2
   ```

1. В консоли Yandex Cloud выберите сервис Virtual Private Cloud и перейдите в раздел *Группы безопасности*. У вас должна отображаться одна группа безопасности с пометкой `Default`.

   ![Группа безопасности по умолчанию](../../../../images/cloud-provider-yandex/sg-ru-default.png)

1. Создайте правила согласно [инструкции Yandex Cloud](https://cloud.yandex.ru/ru/docs/managed-kubernetes/operations/connect/security-groups#rules-internal).

   ![Правила для группы безопасности](../../../../images/cloud-provider-yandex/sg-ru-rules.png)

1. Удалите правило, разрешающее любой **входящий** трафик (на скриншоте выше оно уже удалено), и сохраните изменения.

## Настройка доступа через bastion-хост

Для подключения к узлам, находящимся в приватных подсетях (например, при использовании схемы размещения Standard или WithNATInstance), используется bastion-хост — промежуточная машина с публичным IP, через которую осуществляется SSH-доступ к узлам.

Для настройки доступа выполните следующие шаги:

1. Выполните bootstrap базовой инфраструктуры. Перед созданием bastion-хоста необходимо выполнить начальную фазу установки DKP, которая подготовит сетевую инфраструктуру:

   ```shell
   dhctl bootstrap-phase base-infra --config config.yml
   ```

1. Создайте bastion-хост в Yandex Cloud:

   ```shell
   yc compute instance create \
     --name bastion \
     --hostname bastion \
     --create-boot-disk image-family=ubuntu-2204-lts,image-folder-id=standard-images,size=20,type=network-hdd \
     --memory 2 \
     --cores 2 \
     --core-fraction 100 \
     --ssh-key ~/.ssh/id_rsa.pub \
     --zone ru-central1-a \
     --public-address 178.154.226.159
   ```

   Убедитесь, что IP-адрес из параметра `--public-address` доступен из вашей сети и указан корректно.

1. Запустите основной bootstrap DKP через bastion-хост:

   ```shell
   dhctl bootstrap --ssh-bastion-host=178.154.226.159 --ssh-bastion-user=yc-user \
     --ssh-user=ubuntu --ssh-agent-private-keys=/tmp/.ssh/id_rsa --config=/config.yml
   ```

   Здесь:

   - `--ssh-bastion-user` — пользователь для подключения к bastion-хосту;
   - `--ssh-user` — пользователь на целевых узлах кластера;
   - `--ssh-agent-private-keys` — путь до приватного SSH-ключа;
   - `--config` — путь до конфигурационного файла DKP.
