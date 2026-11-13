---
title: How do I replace the OS image in a cloud cluster?
subsystems:
  - cluster_infrastructure
lang: en
---

Deckhouse Platform creates `CloudEphemeral` and `CloudPermanent` nodes, including master nodes, based on an operating system (OS) image specified in the node group configuration. To move the nodes to a new image, specify it in this configuration instead of the previous one. Deckhouse Platform recreates the `CloudEphemeral` nodes from the new image automatically. The `CloudPermanent` nodes and master nodes are recreated when you apply the configuration with the `dhctl converge` command.

For `CloudStatic` nodes, the administrator creates and maintains the virtual machines. To replace the image of such a node, prepare a virtual machine with the required OS and connect it using the [instructions for adding CloudStatic nodes](admin/configuration/platform-scaling/node/cloud-node.html#adding-a-cloudstatic-node-to-a-cluster).

The resource where the image is specified depends on the node type and the cloud provider configuration method:

| Node type | Provider configuration method | Where to change the image |
| --- | --- | --- |
| `CloudEphemeral` | Through ModuleConfig or `<PROVIDER>ClusterConfiguration` | In the `<PROVIDER>InstanceClass` resource referenced by the node group |
| `CloudPermanent` and master nodes | Through ModuleConfig | In the `<PROVIDER>InstanceClass` resource referenced by the node group |
| `CloudPermanent` and master nodes | Through `<PROVIDER>ClusterConfiguration` | In the `instanceClass` section of the required node group in that resource |

{% alert level="info" %}
Migration from `<PROVIDER>ClusterConfiguration` to the new ModuleConfig-based configuration model is supported only for built-in virtualization (DVP), Yandex Cloud, and zVirt.
{% endalert %}

The image parameters depend on the resource whose configuration you edit. Select the appropriate tab.

{% tabs os_image_parameters %}
{% tab "InstanceClass" %}

For `CloudEphemeral` nodes, use the instance class parameters with either provider configuration method. With a ModuleConfig-based configuration, these parameters also define the image for `CloudPermanent` nodes and master nodes.

| Cloud provider | Parameter in the `spec` section | Instance class resource |
| --- | --- | --- |
| [Amazon Web Services](/modules/cloud-provider-aws/) | [`ami`](/modules/cloud-provider-aws/cr.html#awsinstanceclass-v1-spec-ami) | [AWSInstanceClass](/modules/cloud-provider-aws/cr.html#awsinstanceclass) |
| [Microsoft Azure](/modules/cloud-provider-azure/) | [`urn`](/modules/cloud-provider-azure/cr.html#azureinstanceclass-v1-spec-urn) | [AzureInstanceClass](/modules/cloud-provider-azure/cr.html#azureinstanceclass) |
| [Google Cloud Platform](/modules/cloud-provider-gcp/) | [`image`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-image) | [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass) |
| [Yandex Cloud](/modules/cloud-provider-yandex/) | [`imageID`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-imageid) | [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) |
| [OpenStack](/modules/cloud-provider-openstack/) | [`imageName`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-imagename) | [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) |
| [Huawei Cloud](/modules/cloud-provider-huaweicloud/) | [`imageName`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-imagename) | [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) |
| [Basis Dynamix](/modules/cloud-provider-dynamix/) | [`imageName`](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass-v1-spec-imagename) | [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass) |
| [VMware Cloud Director](/modules/cloud-provider-vcd/) | [`template`](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-template) | [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) |
| [VMware vSphere](/modules/cloud-provider-vsphere/) | [`template`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-template) | [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) |
| [zVirt](/modules/cloud-provider-zvirt/) | [`template`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-template) | [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass) |
| [Built-in virtualization](/modules/cloud-provider-dvp/) | [`rootDisk.image`](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass-v1alpha1-spec-rootdisk-image) | [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) |

{% endtab %}
{% tab "ClusterConfiguration" %}

With a `<PROVIDER>ClusterConfiguration`-based provider configuration, the master node image is set in `masterNodeGroup.instanceClass`, and the image for other `CloudPermanent` nodes is set in `nodeGroups[].instanceClass`.

| Cloud provider | Master nodes | `CloudPermanent` worker nodes |
| --- | --- | --- |
| [Amazon Web Services](/modules/cloud-provider-aws/) | [`masterNodeGroup.instanceClass.ami`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-masternodegroup-instanceclass-ami) | [`nodeGroups[].instanceClass.ami`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-nodegroups-instanceclass-ami) |
| [Microsoft Azure](/modules/cloud-provider-azure/) | [`masterNodeGroup.instanceClass.urn`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-masternodegroup-instanceclass-urn) | [`nodeGroups[].instanceClass.urn`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-nodegroups-instanceclass-urn) |
| [Google Cloud Platform](/modules/cloud-provider-gcp/) | [`masterNodeGroup.instanceClass.image`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-image) | [`nodeGroups[].instanceClass.image`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-nodegroups-instanceclass-image) |
| [Yandex Cloud](/modules/cloud-provider-yandex/) | [`masterNodeGroup.instanceClass.imageID`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-imageid) | [`nodeGroups[].instanceClass.imageID`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-imageid) |
| [OpenStack](/modules/cloud-provider-openstack/) | [`masterNodeGroup.instanceClass.imageName`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-masternodegroup-instanceclass-imagename) | [`nodeGroups[].instanceClass.imageName`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-nodegroups-instanceclass-imagename) |
| [Huawei Cloud](/modules/cloud-provider-huaweicloud/) | [`masterNodeGroup.instanceClass.imageName`](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-masternodegroup-instanceclass-imagename) | [`nodeGroups[].instanceClass.imageName`](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-nodegroups-instanceclass-imagename) |
| [Basis Dynamix](/modules/cloud-provider-dynamix/) | [`masterNodeGroup.instanceClass.imageName`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-imagename) | [`nodeGroups[].instanceClass.imageName`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-nodegroups-instanceclass-imagename) |
| [VMware Cloud Director](/modules/cloud-provider-vcd/) | [`masterNodeGroup.instanceClass.template`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-masternodegroup-instanceclass-template) | [`nodeGroups[].instanceClass.template`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-nodegroups-instanceclass-template) |
| [VMware vSphere](/modules/cloud-provider-vsphere/) | [`masterNodeGroup.instanceClass.template`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-template) | [`nodeGroups[].instanceClass.template`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-template) |
| [zVirt](/modules/cloud-provider-zvirt/) | [`masterNodeGroup.instanceClass.template`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-masternodegroup-instanceclass-template) | [`nodeGroups[].instanceClass.template`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-nodegroups-instanceclass-template) |
| [Built-in virtualization](/modules/cloud-provider-dvp/) | [`masterNodeGroup.instanceClass.rootDisk.image`](/modules/cloud-provider-dvp/cluster_configuration.html#dvpclusterconfiguration-masternodegroup-instanceclass-rootdisk-image) | [`nodeGroups[].instanceClass.rootDisk.image`](/modules/cloud-provider-dvp/cluster_configuration.html#dvpclusterconfiguration-nodegroups-instanceclass-rootdisk-image) |

{% endtab %}
{% endtabs %}

With cloud providers other than [VMware Cloud Director](/modules/cloud-provider-vcd/), [VMware vSphere](/modules/cloud-provider-vsphere/), [zVirt](/modules/cloud-provider-zvirt/), and [built-in virtualization](/modules/cloud-provider-dvp/), the parameter value is a string with the OS image name or ID. In VMware Cloud Director, VMware vSphere, and zVirt, the `template` parameter sets a virtual machine template prepared based on the OS image. To move the nodes of these providers to a new image, prepare a new virtual machine template based on it.

{% alert level="info" %}
In built-in virtualization, the image is specified as an object. In the [`rootDisk.image.kind`](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass-v1alpha1-spec-rootdisk-image-kind) parameter, specify the type of the resource with the image (`ClusterVirtualImage`, `VirtualImage`, or `VirtualDisk`), and in the [`rootDisk.image.name`](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass-v1alpha1-spec-rootdisk-image-name) parameter, specify the name of this resource.
{% endalert %}

{% offtopic title="Example of the image parameter in built-in virtualization" %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: worker
spec:
  rootDisk:
    # The other instance class parameters are not shown.
    image:
      kind: ClusterVirtualImage
      name: ubuntu-24-04-20260204
```

{% endofftopic %}

#### Requirements for the new image

The OS image or the virtual machine template prepared from it must meet the requirements
of the selected cloud provider module.

Before the replacement, make sure that the new image or virtual machine template meets the following general requirements:

- The image parameter value differs from the previous one. If the image or the template is set by name, include the version or the date in the name, for example `ubuntu-24-04-20260204`.
- The image or the template prepared based on it exists in the cloud by the time the nodes are recreated.
- The image contains `cloud-init` and an operating system supported by Deckhouse Platform. The list of supported systems is given in ["Supported OS versions"](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#linux).

For some providers, preparing the image or template involves additional steps.
Follow the instructions in the corresponding module documentation:

- For OpenStack, refer to ["How do I create an image in OpenStack?"](/modules/cloud-provider-openstack/faq.html#how-do-i-create-an-image-in-openstack).
- For Basis Dynamix, refer to ["Prepare an operating system image"](/modules/cloud-provider-dynamix/environment.html#prepare-an-operating-system-image).
- For VMware Cloud Director, refer to ["Virtual machine template"](/modules/cloud-provider-vcd/environment.html#virtual-machine-template).
- For VMware vSphere, refer to ["VM image requirements"](/modules/cloud-provider-vsphere/environment.html#vm-image-requirements), which includes instructions for preparing the image.
- For zVirt, refer to ["Prepare an operating system image"](/modules/cloud-provider-zvirt/environment.html#prepare-an-operating-system-image) and ["Prepare a virtual machine template"](/modules/cloud-provider-zvirt/environment.html#prepare-a-virtual-machine-template).

{% alert level="warning" %}
Deckhouse Platform compares the parameter values in the configuration, not the image content in the cloud. If you replace the image or the template in the cloud keeping its previous name, the nodes are not recreated.
{% endalert %}

#### Changing the image in the configuration

Create a new image in the cloud that meets the requirements above.
In VMware Cloud Director, VMware vSphere, and zVirt, prepare a new virtual machine template based on it.
Then select the cloud provider configuration method.

{% alert level="warning" %}
In Yandex Cloud, `CloudEphemeral` groups whose instance classes do not specify `imageID` use the master node image by default. Changing the master node image does not recreate the existing nodes in these groups. New nodes may use the new image, so a group can contain nodes with different images.

To keep the previous group image, explicitly set its `imageID` in YandexInstanceClass. To recreate the entire group from the new image, explicitly set the new `imageID`.
{% endalert %}

For DVP, Yandex Cloud, and zVirt, check the master node group configuration before selecting a tab:

```shell
d8 k get nodegroup master -o yaml
```

If `spec.cloudInstances.classReference.kind` and `spec.cloudInstances.classReference.name` are set and migration is complete, select the "Through ModuleConfig" tab.
Make sure the migration alert for your provider is not firing:

- For DVP, this is [`D8CloudProviderDVPMigrationPending`](reference/alerts.html#cloud-provider-dvp-d8cloudproviderdvpmigrationpending).
- For Yandex Cloud, this is [`D8CloudProviderYandexCloudMigrationPending`](reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending).
- For zVirt, this is [`D8CloudProviderZvirtMigrationPending`](reference/alerts.html#cloud-provider-zvirt-d8cloudproviderzvirtmigrationpending).

If the alert is firing, [complete the migration](#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat) first.
If the master node group has no `classReference`, or the cluster uses another provider, select the "Through ClusterConfiguration" tab.

{% alert level="info" %}
After completing the migration to ModuleConfig, change the image in the instance class referenced by the required node group.

The cluster may retain the `d8-provider-cluster-configuration` Secret with the old provider configuration. The `d8 system edit provider-cluster-configuration` command lets you edit this configuration, but the platform no longer applies image changes from it.
{% endalert %}

Select the appropriate tab depending on how the cloud provider is configured:

{% tabs os_image_configuration %}
{% tab "Through ModuleConfig" %}

<a id="replacing-the-image-with-moduleconfig-based-configuration"></a>

With a [ModuleConfig-based provider configuration](#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat), the image for `CloudEphemeral` and `CloudPermanent` nodes, including master nodes, is specified in an instance class.
To replace the image, edit the instance class referenced by the required node group:

1. Open the node group configuration:

   ```shell
   d8 k get nodegroup <NODE_GROUP_NAME> -o yaml
   ```

   Replace `<NODE_GROUP_NAME>` with the node group name. For master nodes, use `master`.

   Check `spec.nodeType` for the node type and `spec.cloudInstances.classReference.kind` and `spec.cloudInstances.classReference.name` for the instance class kind and name.
   If multiple groups use the instance class, changing the image affects all of them.

1. Specify the new image or virtual machine template in the instance class of the node group and save the file:

   ```shell
   d8 k edit <PROVIDER>instanceclass <INSTANCE_CLASS_NAME>
   ```

   Replace `<PROVIDER>` with the cloud provider name in lowercase, for example `yandex`, and `<INSTANCE_CLASS_NAME>` with the instance class name.

   An example for Yandex Cloud:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: YandexInstanceClass
   metadata:
     name: worker
   spec:
     cores: 4
     memory: 8192
     imageID: fd83ergat2e815oohe7o
   ```

{% offtopic title="Example of the master node configuration in built-in virtualization" %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: master
spec:
  rootDisk:
    # The other instance class parameters are not shown.
    image:
      kind: ClusterVirtualImage
      name: ubuntu-24-04-20260204
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
    minPerZone: 1
    maxPerZone: 1
```

{% endofftopic %}

After changing the instance class, proceed to [applying the changes](#applying-the-changes).

{% endtab %}
{% tab "Through ClusterConfiguration" %}

With a `<PROVIDER>ClusterConfiguration`-based provider configuration, the resource to edit depends on the node type.
Select the node type of the group.

{% tabs os_image_legacy_node_type %}
{% tab "CloudEphemeral" %}

For `CloudEphemeral` nodes, the image is specified in the instance class referenced by the node group:

1. Check `spec.cloudInstances.classReference.kind` and `spec.cloudInstances.classReference.name` in the NodeGroup resource for the instance class kind and name:

   ```shell
   d8 k get nodegroup <NODE_GROUP_NAME> -o yaml
   ```

   Replace `<NODE_GROUP_NAME>` with the node group name.

1. Open the instance class and change the image parameter listed in the table above:

   ```shell
   d8 k edit <PROVIDER>instanceclass <INSTANCE_CLASS_NAME>
   ```

   Replace `<PROVIDER>` with the cloud provider name in lowercase, for example `yandex`, and `<INSTANCE_CLASS_NAME>` with the instance class name.

{% endtab %}
{% tab "CloudPermanent" %}

For `CloudPermanent` nodes, change the image in the provider configuration:

1. Open the cloud provider configuration:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Change the image parameter in the `instanceClass` section of the required group in the `nodeGroups` list and save the file. An example for Yandex Cloud:

   ```yaml
   nodeGroups:
   - name: front
     replicas: 2
     instanceClass:
       cores: 4
       memory: 8192
       imageID: fd83ergat2e815oohe7o
   ```

{% endtab %}
{% tab "Master nodes" %}

For master nodes, change the image in the provider configuration:

1. Open the cloud provider configuration:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Change the image parameter in the `masterNodeGroup.instanceClass` section and save the file.

{% endtab %}
{% endtabs %}
{% endtab %}
{% endtabs %}

After saving the configuration, apply the changes according to the node type.

#### Applying the changes

The way to apply the changes depends on the node type and is the same for both provider configuration methods.
Select the appropriate tab.

{% tabs os_image_apply %}
{% tab "CloudEphemeral" %}

<a id="replacing-the-image-of-cloudephemeral-nodes"></a>

Deckhouse Platform automatically recreates `CloudEphemeral` nodes after the image in the instance class changes.
To check that the replacement is complete, follow these steps:

1. Wait until Deckhouse Platform recreates the nodes of the group:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<NODE_GROUP_NAME>
   ```

   Replace `<NODE_GROUP_NAME>` with the node group name.

   During the replacement, the list may contain both the previous and the new nodes. The new nodes have a smaller value in the `AGE` column. An example of the output during the replacement:

   ```console
   NAME                                     STATUS     ROLES    AGE     VERSION
   cloud-demo-worker-92f9049e-gm2lw-6hkm2   NotReady   worker   37s     v1.34.1
   cloud-demo-worker-92f9049e-xn29v-66rm9   Ready      worker   8m21s   v1.34.1
   ```

   The replacement is complete when only the new nodes in the `Ready` status remain in the list.

{% endtab %}
{% tab "CloudPermanent and master nodes" %}

<a id="replacing-the-image-of-cloudpermanent-nodes"></a>
<a id="replacing-the-image-of-master-nodes"></a>

To apply the new image to `CloudPermanent` nodes and master nodes, run `dhctl converge`.
The command reads the current configuration from the cluster, including ModuleConfig, NodeGroup, and instance classes when the provider uses a ModuleConfig-based configuration.

Before replacing master nodes, [back up etcd](admin/configuration/backup/backup-and-restore.html#backing-up-etcd).
Also back up the `/etc/kubernetes` directory and copy both backups outside the cluster.
Master nodes are recreated one at a time. In a single-master cluster, the command may offer a temporary transition to three master nodes and a return to one master node after the replacement.

1. On the local machine, get the Deckhouse Platform edition and version in the cluster:

   ```shell
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}')
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]')
   ```

1. On the local machine, run the Deckhouse Platform installer container of the same edition and version:

   ```shell
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

   If the cluster uses a different container registry, specify its address instead of `registry.deckhouse.io`.

1. In the installer container, check which changes will be made in the cloud:

   ```shell
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host=<MASTER_IP>
   ```

   Replace `<SSH_KEY_FILENAME>` with the name of the private SSH key file, `<USERNAME>` with the user for connecting to the master node, and `<MASTER_IP>` with the master node address.

   When replacing master nodes, specify all master node addresses by repeating `--ssh-host` for each address.

   In the `dhctl terraform check` output, verify that the changes to nodes and other cloud resources match your expectations. In the final `nodes` list, nodes that require recreation have the `destructively_changed` status.
   If multiple groups use the same instance class, check the changes for each group.

1. In the installer container, apply the changes using the same SSH connection parameters as in the check:

   ```shell
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host=<MASTER_IP>
   ```

   The command displays the plan of changes and asks for the `Do you want to CHANGE objects state in the cloud?` confirmation before replacing each node. To replace the node, enter `y`. The command also asks you to confirm that the master node name matches its address.

   Before confirming the replacement of the next master node, check etcd and the control plane using the [master node verification procedure](admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#changing-the-os-image-of-master-nodes-in-a-multi-master-cluster).

1. Check that the nodes of the group are recreated:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<NODE_GROUP_NAME>
   ```

   Replace `<NODE_GROUP_NAME>` with the node group name. For master nodes, use `master`.
   Wait until the new nodes reach the `Ready` status.

{% endtab %}
{% endtabs %}
