---
title: "Cloud provider - zVirt: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в zVirt при работе облачного провайдера Deckhouse."
---

## Standard

![resources](images/zvirt-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11447&t=IvETjbByf1MSQzcm-0 --->

Пример конфигурации схемы размещения:

```yaml
---
apiVersion: deckhouse.io/v1
kind: ZvirtClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAAABBBB"
clusterID: "b46372e7-0d52-40c7-9bbf-fda31e187088"
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 4
    memory: 8192
    rootDiskSizeGb: 40
    template: ALT-p10
    vnicProfileID: "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
    storageDomainID: "c4bf82a5-b803-40c3-9f6c-b9398378f424"
```
