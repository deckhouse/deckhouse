---
title: Схемы размещения и настройка
permalink: ru/admin/integrations/virtualization/zvirt/layout.html
lang: ru
---

## Standard

Схема размещения Standard используется для интеграции Deckhouse Kubernetes Platform с виртуальной инфраструктурой zVirt. Эта схема предполагает развертывание всех узлов в пределах одного кластера zVirt с централизованным управлением шаблонами, хранилищем и сетями.

Особенности:

- Использование одного zVirt-кластера ([`clusterID`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-clusterid));
- Хранилище (Storage Domain), доступное всем хостам кластера;
- Использование шаблона виртуальной машины, созданного из cloud-образа;
- Присвоение сетевого профиля vNIC при заказе ВМ;
- Полная автоматизация создания и удаления узлов через API zVirt.

![resources](../../../../images/cloud-provider-zvirt/zvirt-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11447&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: ZvirtClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAAABBBB"
clusterID: "b46372e7-0d52-40c7-9bbf-fda31e187088"
provider:
  server: "<SERVER>"
  username: "<USERNAME>"
  password: "<PASSWORD>"
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

Обязательные параметры [ресурса ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration):

- `clusterID` — UUID кластера в zVirt, где размещаются виртуальные машины;
- `sshPublicKey` — публичный SSH-ключ для доступа на узлы;
- `template` — имя подготовленного шаблона виртуальной машины;
- `vnicProfileID` — UUID сетевого профиля (vNIC);
- `storageDomainID` — UUID хранилища (Storage Domain), в котором размещаются диски.

{% alert level="info" %}
Значения UUID (`clusterID`, `vnicProfileID`, `storageDomainID`) можно получить через API zVirt или интерфейс администратора.
{% endalert %}
