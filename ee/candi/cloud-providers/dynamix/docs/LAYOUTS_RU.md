---
title: "Cloud provider - Dynamix: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в Dynamix при работе облачного провайдера Deckhouse."
---

## Standard

![resources](../../images/030-cloud-provider-zvirt/network/zvirt-standard.svg)
<!--- Исходник: https://docs.google.com/drawings/d/1aosnFD7AzBgHrQGvxxQHZPfV0PSaTM66A-EPMWgPEqw/edit --->

Пример конфигурации схемы размещения:

```yaml
---
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAAA"
location: dynamix
account: acc_user
nodeNetworkCIDR: "10.241.32.0/24"
nameservers: ["10.0.0.10"]
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

Пример конфигурации схемы размещения:

```yaml
---
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: StandardWithInternalNetwork
sshPublicKey: "ssh-rsa AAAA"
location: dynamix
account: acc_user
nodeNetworkCIDR: "10.241.32.0/24"
nameservers: ["10.0.0.10"]
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
