---
title: Конфигурация и схема размещения
permalink: ru/admin/integrations/public/yandex/yandex-layout.html
lang: ru
---

Для интеграции Deckhouse Kubernetes Platform с Yandex Cloud необходимо описать инфраструктуру кластера с помощью ресурса Kubernetes типа YandexClusterConfiguration. Этот объект управляется модулем `cloud-provider-yandex` и содержит полную информацию о схеме размещения, зонах, параметрах узлов и сетевой конфигурации.

Этот ресурс используется в процессе `bootstrap` и при модификации кластера с помощью утилиты `dhctl` или компонента `deckhouse-controller`.

## Назначение YandexClusterConfiguration

YandexClusterConfiguration — это объект Custom Resource (CR), описывающий параметры интеграции с облаком Yandex Cloud. Он используется платформой Deckhouse для:

- размещения master и worker-узлов в облаке;
- задания схемы сетевого взаимодействия;
- подключения к API Yandex Cloud с использованием авторизационного JSON-ключа;
- настройки подсетей, публичных IP, ресурсов виртуальных машин и др.

Обязательные поля:

- `apiVersion` — должен быть `deckhouse.io/v1`;
- `kind` — всегда YandexClusterConfiguration.

Пример заголовка ресурса:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
```

Чтобы отредактировать этот ресурс в работающем кластере, выполните команду:

```console
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

После внесения изменений их необходимо применить с помощью:

```console
dhctl converge
```

### Пример конфигурации

Ниже приведён пример полной рабочей конфигурации с тремя master-узлами и двумя worker-узлами, размещёнными в зонах `ru-central1-a`, `ru-central1-b`, `ru-central1-d`:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
nodeNetworkCIDR: 192.168.12.13/24
sshPublicKey: "<SSH_PUBLIC_KEY>"

provider:
  cloudID: "<CLOUD_ID>"
  folderID: "<FOLDER_ID>"
  serviceAccountJSON: |
    {
      "id": "...",
      "service_account_id": "...",
      "key_algorithm": "RSA_2048",
      "public_key": "...",
      "private_key": "..."
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
      - "Auto"
      - "Auto"
      - "Auto"
    externalSubnetIDs:
      - <ZONE_A_SUBNET_ID>
      - <ZONE_B_SUBNET_ID>
      - <ZONE_D_SUBNET_ID>

nodeGroups:
- name: worker
  replicas: 2
  zones:
    - ru-central1-a
    - ru-central1-b
  instanceClass:
    cores: 4
    memory: 8192
    coreFraction: 50
    imageID: <IMAGE_ID>
    externalIPAddresses:
      - "Auto"
      - "Auto"
    externalSubnetIDs:
      - <ZONE_A_SUBNET_ID>
      - <ZONE_B_SUBNET_ID>
```

## Сетевые параметры и безопасность

### nodeNetworkCIDR

Подсеть, которая будет автоматически разделена на три зоны (для Standard/WithNATInstance). Пример:

```yaml
nodeNetworkCIDR: 192.168.12.13/24
```

### externalSubnetIDs

Массив ID подсетей, по одному на каждую зону. Используется для назначения публичных IP-адресов.

### dhcpOptions (необязательно)

Позволяет задать параметры DHCP-сервера: поисковый домен и DNS-серверы.

Пример:

```yaml
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 192.168.0.2
  - 192.168.0.3
```

> Использование внешних DNS имеет ограничения. Убедитесь, что DNS-серверы разрешают и внешние, и внутренние имена.

### Группы безопасности

По умолчанию в облаке создаётся группа безопасности с разрешающими правилами. Не удаляйте их до завершения настройки.

Чтобы найти название используемой сети выполните команду:

```console
kubectl get secrets -n kube-system d8-cluster-configuration -ojson | \
  jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix | cut -d: -f2
```

## Схемы размещения

Схема размещения указывается в поле `layout`. Поддерживаются три варианта: Standard, WithoutNAT, WithNATInstance.

### Standard

(image)[TODO]

- Узлы без публичного IP.
- Доступ в интернет через NAT-шлюз Yandex Cloud.
- Поддерживаются группы безопасности.

### WithoutNAT

(image)[TODO]

- Каждый узел получает публичный IP.
- Нет NAT, но не поддерживаются группы безопасности — узлы доступны извне.

### WithNATInstance

(image)[TODO]

- Создаётся отдельный NAT-инстанс.
- На маршрут по умолчанию (0.0.0.0/0) назначается этот инстанс.
- Можно указать внешний IP, либо позволить Deckhouse создать его автоматически.

Пример:

```yaml
layout: WithNATInstance
withNATInstance:
  natInstanceExternalAddress: <IP>
  internalSubnetID: <ID>
  externalSubnetID: <ID>
```

Если передать `withNATInstance: {}`, все ресурсы будут созданы автоматически.

### Дополнительные параметры и рекомендации

`labels` — лейблы, назначаемые на облачные ресурсы.
`zones` — допустимые зоны размещения: ru-central1-a, ru-central1-b, ru-central1-d.
`diskSizeGB`, `coreFraction`, `platform`, `etcdDiskSizeGb` — детальная настройка виртуальных машин.
`externalIPAddresses: ["Auto", ...]` — автоматическое получение публичных IP по зонам.

> Количество IP-адресов и externalSubnetID должно соответствовать числу узлов и порядку зон.

