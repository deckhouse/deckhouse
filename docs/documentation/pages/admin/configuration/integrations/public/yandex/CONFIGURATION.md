---
title: Integration configuration in Yandex Cloud
permalink: en/admin/integrations/public/yandex/configuration.html
lang: en
---

## Configuration

The Yandex [Cloud provider](../../../../glossary.html#cloud-provider) parameters are defined in four resources:

- The [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module defines the [layout](layout.html#layouts), the [network parameters](network.html#network-parameters-and-security), and the cloud and folder IDs.
- The `d8-credentials` Secret in the `d8-cloud-provider-yandex` namespace stores the service account key. Its format is described in ["Credentials Secret"](authorization.html#credentials-secret).
- [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) describes the virtual machine parameters.
- [NodeGroup](/modules/node-manager/cr.html#nodegroup) describes the [node groups](../../../../glossary.html#node-group) and refers to an [instance class](../../../../glossary.html#instance-class). The [master node](../../../../glossary.html#master-node) group is described by the same resource with the [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) type.

To change the provider settings in a running cluster, edit the ModuleConfig:

```shell
d8 k edit mc cloud-provider-yandex
```

After changing the node parameters, run the following command:

```shell
dhctl converge
```

{% alert level="warning" %}
If the cluster was installed with the [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) schema, it must be migrated to the ModuleConfig-based configuration. Until the migration is complete, the [`D8CloudProviderYandexCloudMigrationPending`](../../../../reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending) alert fires and a Deckhouse Platform update can be blocked.

For instructions, see ["How to migrate a cloud provider to ModuleConfig-based configuration?"](../../../../faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat).

If the module settings had the `storageClass.default` parameter, set the default StorageClass with the global [`defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass) parameter after the migration. The conversion does not carry this value over.
{% endalert %}

### Configuration example

Below is a minimal configuration of a cluster with the Standard [layout](../../../../glossary.html#layout), one master node, and one group of worker nodes:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 10.241.32.0/20
        sshPublicKey: <SSH_PUBLIC_KEY>
        labels:
          billing: prod
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: master
spec:
  cores: 4
  memory: 8192
  imageID: <IMAGE_ID>
  etcdDiskSizeGB: 10
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: master
    minPerZone: 1
    maxPerZone: 1
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
spec:
  cores: 4
  memory: 8192
  coreFraction: 50
  imageID: <IMAGE_ID>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: worker
    minPerZone: 1
    maxPerZone: 3
    zones:
      - ru-central1-a
```

Where:

- `CLOUD_ID` and `FOLDER_ID` are the Yandex Cloud cloud and folder identifiers, specified by the [`cloudID`](/modules/cloud-provider-yandex/configuration.html#parameters-provider-parameters-cloudid) and [`folderID`](/modules/cloud-provider-yandex/configuration.html#parameters-provider-parameters-folderid) parameters in the `provider.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module. For instructions on obtaining these identifiers, see ["Connecting and authorizing in Yandex Cloud"](authorization.html).
- `SSH_PUBLIC_KEY` is the public SSH key used to access nodes, specified by the [`sshPublicKey`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-sshpublickey) parameter in the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module.
- `IMAGE_ID` is the operating system image ID used to create virtual machines, specified by the [`imageID`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-imageid) parameter of the YandexInstanceClass resource. If the parameter is not set, the image of the `master` node group instance class is used.

The YandexInstanceClass resource for the `master` node group must specify the [`etcdDiskSizeGB`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-etcddisksizegb) parameter, which defines the size of a separate disk for etcd. This parameter must not be specified for other node groups.

The rest of the parameters are described in the [module settings](/modules/cloud-provider-yandex/configuration.html) and in the [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) resource description.

The service account key is not stored in ModuleConfig. It is defined in a [separate Secret](authorization.html#credentials-secret).
