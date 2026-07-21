---
title: Схемы размещения и настройка Deckhouse Virtualization Platform
permalink: ru/admin/integrations/virtualization/dvp/configuration-and-layout-scheme.html
lang: ru
---

![Схема размещения Standard](../../../../images/cloud-provider-dvp/dvp-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=1314-7740&t=5VUUyoMpasR1vVxZ-4 --->

{% alert level="warning" %}
Если кластер был установлен со схемой DVPClusterConfiguration, необходима миграция на конфигурацию через ModuleConfig.
Пока миграция не выполнена, может срабатывать алерт `D8CloudProviderDVPMigrationPending`, а обновление Deckhouse — блокироваться.

Инструкция: [Как мигрировать облачный провайдер на конфигурацию через ModuleConfig](/faq.html#subsystem-cluster_infrastructure).
{% endalert %}

Пример конфигурации схемы размещения:

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
        sshPublicKey: <SSH_PUBLIC_KEY>
        ipAddresses:
          master:
            - Auto
    provider:
      parameters:
        namespace: demo
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: kubeconfig
  secret: <KUBE_CONFIG_BASE64>
---
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: master
spec:
  virtualMachine:
    cpu:
      cores: 4
      coreFraction: 100%
    memory:
      size: 8Gi
    virtualMachineClassName: generic
  rootDisk:
    size: 50Gi
    storageClass: ceph-pool-r2-csi-rbd-immediate
    image:
      kind: ClusterVirtualImage
      name: ubuntu-2204
  etcdDisk:
    size: 15Gi
    storageClass: ceph-pool-r2-csi-rbd-immediate
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
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
```

## Конфигурация

Для описания конфигурации кластера в DVP Deckhouse использует:

* [ModuleConfig](/modules/cloud-provider-dvp/configuration.html) `cloud-provider-dvp` — настройки провайдера и схемы размещения;
* Secret `d8-credentials` типа `cloud-provider.deckhouse.io/credentials` — kubeconfig для доступа к API родительского DVP;
* [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup) — параметры узлов.

Для изменения настроек модуля в работающем кластере выполните:

```shell
d8 k edit mc cloud-provider-dvp
```

После изменения параметров узлов (InstanceClass / NodeGroup) выполните команду:

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
  secret: ZXhhbXBsZQo=
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
      beta.kubernetes.io/os: linux
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
