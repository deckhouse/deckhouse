---
title: Integration configuration in built-in virtualization
permalink: en/admin/integrations/virtualization/dvp/configuration.html
lang: en
---

## Configuration

{% alert level="warning" %}
If the cluster was installed with the DVPClusterConfiguration schema, migration to ModuleConfig-based configuration is required.
Until the migration is completed, the `D8CloudProviderDVPMigrationPending` alert may fire and Deckhouse updates may be blocked.

See: [How to migrate a cloud provider to ModuleConfig-based configuration](../../../../faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat).
{% endalert %}

The built-in virtualization [cloud provider](../../../../glossary.html#cloud-provider) parameters are set in four resources:

- The [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) of the `cloud-provider-dvp` module defines the [layout](../../../../glossary.html#layout) and the parent cluster connection parameters.
- The `d8-credentials` Secret in the `d8-cloud-provider-dvp` namespace stores the credentials for accessing the parent cluster API. Its format is covered in the [Credentials Secret](authorization.html#credentials-secret) section.
- The [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) resource describes the virtual machine parameters.
- The [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource describes the [node groups](../../../../glossary.html#node-group) and references the [instance class](../../../../glossary.html#instance-class).

To modify the provider settings in a running cluster, edit the ModuleConfig:

```shell
d8 k edit mc cloud-provider-dvp
```

After changing node parameters, run the command:

```shell
dhctl converge
```

Example of a configuration with multiple parameters:

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

## Replacing the OS image

The [`rootDisk.image`](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass-v1alpha1-spec-rootdisk-image) parameter of DVPInstanceClass defines the image for `CloudEphemeral` nodes. With a ModuleConfig-based provider configuration, this parameter also defines the image for `CloudPermanent` nodes and master nodes.

With a DVPClusterConfiguration-based configuration, the image for `CloudPermanent` nodes and master nodes is set by `rootDisk.image` in `nodeGroups[].instanceClass` and `masterNodeGroup.instanceClass`, respectively.

The replacement procedure is described in ["How do I replace the OS image in a cloud cluster?"](../../../../faq.html#how-do-i-replace-the-os-image-in-a-cloud-cluster).
