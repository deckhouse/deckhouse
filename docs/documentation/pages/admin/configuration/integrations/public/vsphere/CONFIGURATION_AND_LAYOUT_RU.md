---
title: Схемы размещения и настройка
permalink: ru/admin/integrations/public/vsphere/vsphere-layout.html
lang: ru
---

## Standard

Схема Standard предназначена для размещения кластера внутри инфраструктуры vSphere с возможностью управления ресурсами, сетями и хранилищем.

Особенности:

- Использование vSphere Datacenter в качестве региона (`region`);
- Использование vSphere Cluster в качестве зоны (`zone`);
- Поддержка нескольких зон и размещения узлов по зонам;
- Использование различных datastore для дисков и volume’ов;
- Поддержка подключения сетей, включая дополнительную сетевую изоляцию (например, MetalLB + BGP).

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
vmFolderPath: dev
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
region: X1
internalNetworkCIDR: 192.168.199.0/24
masterNodeGroup:
  replicas: 1
  zones:
    - ru-central1-a
    - ru-central1-b
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: net3-k8s
nodeGroups:
  - name: khm
    replicas: 1
    zones:
      - ru-central1-a
    instanceClass:
      numCPUs: 4
      memory: 8192
      template: dev/golden_image
      datastore: dev/lun_1
      mainNetwork: net3-k8s
sshPublicKey: "<SSH_PUBLIC_KEY>"
zones:
  - ru-central1-a
  - ru-central1-b
```

Обязательные параметры:

- `region` — тег, присвоенный объекту Datacenter;
- `zoneTagCategory` и `regionTagCategory` — категории тегов, по которым распознаются регионы и зоны;
- `internalNetworkCIDR` — подсеть для назначения внутренних IP-адресов;
- `vmFolderPath` — путь до папки, в которой будут размещаться виртуальные машины кластера;
- `sshPublicKey` — публичный SSH-ключ для доступа к узлам;
- `zones` — список зон, доступных для размещения узлов.

> Все узлы, размещённые в разных зонах, должны иметь доступ к общим datastore с аналогичными тегами зоны.
