---
title: Конфигурация и схема размещения
permalink: ru/admin/integrations/virtualization/zvirt/сonfiguration-and-layout-scheme.html
lang: ru
---

## Схемы размещения

DKP поддерживает одну схему размещения ресурсов в zVirt.

### Standard

![resources](../../../../images/cloud-provider-zvirt/zvirt-standard.png)
<!--- Исходник: https://docs.google.com/drawings/d/1aosnFD7AzBgHrQGvxxQHZPfV0PSaTM66A-EPMWgPEqw/edit --->

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

## Конфигурация

Интеграции с vSphere осуществляется с помощью ресурса ZvirtClusterConfiguration, который описывает конфигурацию облачного кластера в zVirt и используется системой виртаулизации, если управляющий слой (control plane) кластера размещён в системе. Отвечающий за интеграцию модуль DKP настраивается автоматически, исходя из выбранной схемы размещения.

Выполните следующую команду, чтобы изменить конфигурацию в работающем кластере:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

> После изменения параметров узлов необходимо выполнить команду [dhctl converge](../../deckhouse-faq.html#изменение-конфигурации), чтобы изменения вступили в силу.

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: ZvirtClusterConfiguration
layout: Standard
clusterID: b46372e7-0d52-40c7-9bbf-fda31e187088
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: debian-bookworm
    vnicProfileID: 49bb4594-0cd4-4eb7-8288-8594eafd5a86
    storageDomainID: c4bf82a5-b803-40c3-9f6c-b9398378f424
nodeGroups:
  - name: worker
    replicas: 1
    instanceClass:
      numCPUs: 4
      memory: 8192
      template: debian-bookworm
      vnicProfileID: 49bb4594-0cd4-4eb7-8288-8594eafd5a86
provider:
  server: "<SERVER>"
  username: "<USERNAME>"
  password: "<PASSWORD>"
  insecure: true
```

### Получение vNicProfileId

VNicProfileId можно получить путем запроса к zVirt API:

```bash
curl -u "<имя пользователя>@<профить>:<пароль>" -X GET https://<zVirt API URL>/vnicprofiles
```

Пример ответа:

```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<vnic_profiles>
    <vnic_profile href="/ovirt-engine/api/vnicprofiles/49bb4594-0cd4-4eb7-8288-8594eafd5a86" id="49bb4594-0cd4-4eb7-8288-8594eafd5a86">
        <name>vm-net-01</name>
        <link href="/ovirt-engine/api/vnicprofiles/49bb4594-0cd4-4eb7-8288-8594eafd5a86/permissions" rel="permissions"/>
        <pass_through>
            <mode>disabled</mode>
        </pass_through>
        <port_mirroring>false</port_mirroring>
        <network href="/ovirt-engine/api/networks/74a741c9-0d40-4008-8e58-1c903ee6eba7" id="74a741c9-0d40-4008-8e58-1c903ee6eba7"/>
    </vnic_profile>
    ...
</vnic_profiles>
```
