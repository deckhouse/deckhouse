---
title: Настройка встроенной виртуализации
permalink: ru/admin/integrations/virtualization/dvp/configuration.html
lang: ru
---

## Конфигурация

{% alert level="warning" %}
Если кластер был установлен со схемой DVPClusterConfiguration, необходима миграция на конфигурацию через ModuleConfig.
Пока миграция не выполнена, может срабатывать алерт `D8CloudProviderDVPMigrationPending`, а обновление Deckhouse — блокироваться.

Инструкция: [Как мигрировать облачный провайдер на конфигурацию через ModuleConfig](../../../../faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec).
{% endalert %}

Параметры [облачного провайдера](../../../../glossary.html#cloud-provider) встроенной виртуализации задаются в четырёх ресурсах:

- [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля `cloud-provider-dvp` задаёт [схему размещения](../../../../glossary.html#layout) и параметры подключения к родительскому кластеру;
- секрет `d8-credentials` в неймспейсе `d8-cloud-provider-dvp` хранит учётные данные для доступа к API родительского кластера, его формат описан в разделе [«Секрет с учётными данными»](authorization.html#секрет-с-учётными-данными);
- [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) описывает параметры виртуальных машин;
- [NodeGroup](/modules/node-manager/cr.html#nodegroup) описывает [группы узлов](../../../../glossary.html#node-group) и ссылается на [инстанс-класс](../../../../glossary.html#instance-class).

Чтобы изменить настройки провайдера в работающем кластере, отредактируйте ModuleConfig:

```shell
d8 k edit mc cloud-provider-dvp
```

После изменения параметров узлов выполните команду:

```shell
dhctl converge
```

Пример конфигурации с множеством параметров:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 2
  enabled: true
  settings:
    nodes:
      parameters:
        layout: Standard
        sshPublicKey: "<SSH_PUBLIC_KEY>"
        region: r1
        zones:
          - zone-a
          - zone-b
          - zone-c
        ipAddresses:
          master:
            - 10.66.30.100
            - 10.66.30.101
            - 10.66.30.102
    provider:
      parameters:
        namespace: default
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: kubeconfig
  secret: dGVzdC12YWx1ZQ==
---
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: master
spec:
  virtualMachine:
    cpu:
      cores: 1
      coreFraction: 100%
    memory:
      size: 4Gi
    virtualMachineClassName: generic
    additionalLabels:
      additional-vm-label: label-value
    additionalAnnotations:
      additional-vm-annotation: annotation-value
    tolerations:
      - key: dedicated.deckhouse.io
        operator: Equal
        value: system
    nodeSelector:
      nodeSelectorTerms:
        - matchExpressions:
            - key: beta.kubernetes.io/os
              operator: In
              values:
                - linux
  rootDisk:
    size: 10Gi
    storageClass: linstor-thin-r1
    image:
      kind: ClusterVirtualImage
      name: ubuntu-2204
  etcdDisk:
    size: 10Gi
    storageClass: linstor-thin-r1
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      kind: DVPInstanceClass
      name: master
    maxPerZone: 1
    minPerZone: 1
    zones:
      - zone-a
      - zone-b
      - zone-c
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: worker
spec:
  virtualMachine:
    cpu:
      cores: 4
      coreFraction: 100%
    memory:
      size: 8Gi
    virtualMachineClassName: generic
  rootDisk:
    size: 10Gi
    image:
      kind: ClusterVirtualImage
      name: ubuntu-2204
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      kind: DVPInstanceClass
      name: worker
    maxPerZone: 1
    minPerZone: 1
    zones:
      - zone-a
      - zone-b
      - zone-c
```

## Замена образа ОС

Параметр [`rootDisk.image`](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass-v1alpha1-spec-rootdisk-image) ресурса DVPInstanceClass задаёт образ узлов типа `CloudEphemeral`. При настройке провайдера через ModuleConfig этот параметр также задаёт образ узлов типа `CloudPermanent` и master-узлов.

При настройке через DVPClusterConfiguration образ узлов типа `CloudPermanent` и master-узлов задаётся параметром `rootDisk.image` в секциях `nodeGroups[].instanceClass` и `masterNodeGroup.instanceClass` соответственно.

Порядок замены описан в разделе [«Как заменить образ ОС в облачном кластере?»](../../../../faq.html#как-заменить-образ-ос-в-облачном-кластере).
