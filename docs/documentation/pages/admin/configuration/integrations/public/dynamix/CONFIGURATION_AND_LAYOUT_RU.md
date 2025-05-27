---
title: Схемы размещения и настройка
permalink: ru/admin/integrations/public/dynamix/dynamix-layout.html
lang: ru
---

Платформа Deckhouse поддерживает две схемы размещения в облаке Базис.DynamiX:

- Standard — схема с использованием только внешней сети;
- StandardWithInternalNetwork — схема с внутренней (приватной) сетью и DNS-серверами.

Обе схемы позволяют управлять размещением узлов кластера, настройкой сетей, образом ОС и хранилищем данных.

## Standard

```yaml
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAAA"
location: dynamix
account: acc_user
provider:
  controllerUrl: "<controller url>"
  oAuth2Url: "<oAuth2 url>"
  appId: "<app id>"
  appSecret: "<app secret>"
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: "<image name>"
    storageEndpoint: "<storage endpoint>"
    pool: "<pool>"
    externalNetwork: "<external network>"
```

## StandardWithInternalNetwork

```yaml
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: StandardWithInternalNetwork
sshPublicKey: "ssh-rsa AAAA"
location: dynamix
account: acc_user
nodeNetworkCIDR: "10.241.32.0/24"
nameservers:
  - "10.0.0.10"
provider:
  controllerUrl: "<controller url>"
  oAuth2Url: "<oAuth2 url>"
  appId: "<app id>"
  appSecret: "<app secret>"
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: "<image name>"
    storageEndpoint: "<storage endpoint>"
    pool: "<pool>"
    externalNetwork: "<external network>"
```

## Обязательные параметры

`sshPublicKey`— публичный ключ для доступа к узлам;
`location` — имя расположения облака (например, dynamix);
`account` — имя аккаунта в облаке;
`provider.controllerUrl`, `oAuth2Url`, `appId`, `appSecret` — параметры доступа к API;
`imageName` — название образа ОС;
`externalNetwork` — имя внешней сети;
`storageEndpoint`, `pool` — параметры хранилища;
`nodeNetworkCIDR` и `nameservers` — параметры внутренней сети (только для схемы StandardWithInternalNetwork).

После изменения параметров необходимо выполнить команду `dhctl converge`, чтобы изменения вступили в силу.
