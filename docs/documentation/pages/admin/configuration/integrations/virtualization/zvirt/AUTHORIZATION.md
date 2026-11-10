---
title: Connection and authorization in zVirt
permalink: en/admin/integrations/virtualization/zvirt/authorization.html
---

For Deckhouse Platform to create virtual machines in zVirt, assign the required roles to the zVirt user, prepare a cloud image of the operating system and a virtual machine template, and pass the user login and password in the credentials Secret.

## Requirements

Deckhouse Platform requires the following to work with zVirt:

- A working zVirt installation with an accessible API
- zVirt version `4.0`–`4.4`
- A user with the permissions listed in the ["User permissions"](#user-permissions) section
- A storage domain with an uploaded cloud image of the operating system
- A virtual machine template prepared from the cloud image

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

## User permissions

Deckhouse Platform does not require the `SuperUser` role for the zVirt user. It is enough to assign the following predefined zVirt roles to the user on the objects the cluster works with:

| Role | Object | Purpose |
| --- | --- | --- |
| `UserVmManager` | zVirt cluster from the [`clusterID`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-clusterid) parameter | Creating, starting, stopping, and deleting node virtual machines, creating their network interfaces, attaching and detaching disks |
| `UserTemplateBasedVm` | Virtual machine template | Creating virtual machines from the template |
| `DiskOperator` | Storage domains that contain the template disk and where disks for etcd and cluster volumes are created | Creating, resizing, attaching, detaching, and deleting disks |
| `VnicProfileUser` | vNIC profile | Creating a virtual machine network interface from the vNIC profile |
| `TagManager` | Entire system (system permission) | Creating the cluster tag and assigning it to virtual machines of CloudEphemeral nodes |

The `TagManager` role is assigned only on the entire system, because zVirt checks the tag management permission at this level.

{% alert level="warning" %}
The `TagManager` role is administrative. With it, the user can view all objects via the zVirt API but can modify only the objects on which the roles from the table are assigned to the user.
{% endalert %}

zVirt assigns some roles to the Everyone group by default, so you do not need to assign them to the user separately:

- `UserTemplateBasedVm` on a template created with the "Allow all users to access this Template" checkbox selected
- `VnicProfileUser` on a public vNIC profile
- `CpuProfileOperator` on the cluster CPU profile
- `DiskProfileUser` on the disk profiles of storage domains

If these permissions were removed in your zVirt installation, assign the roles to the user.

To assign a role on an object:

1. Open the object in the zVirt Administration Portal, for example, a cluster in the "Compute → Clusters" section.
1. Go to the "Permissions" tab and click "Add".
1. Select the user and the role and save the permission.

Assign the `TagManager` role in the "Administration → Configure" section on the "System Permissions" tab.

## Preparing a cloud image

The cloud image of the operating system (OS) is uploaded to the zVirt disk storage. To upload the image:

1. Open the zVirt Administration Portal and go to the "Storage → Disks" section.
1. Upload the OS cloud image (for example, `.qcow2` or `.img`) to the storage.

   ![Starting the upload of the OS cloud image](../../../../images/cloud-provider-zvirt/template/disk-upload-menu-en.png)

   ![Uploading the OS cloud image](../../../../images/cloud-provider-zvirt/template/disk-upload-dialog-en.png)

1. Wait for the upload to complete. The disk must get the `OK` status.

   ![Finishing the upload of the OS cloud image](../../../../images/cloud-provider-zvirt/template/disk-upload-status-en.png)

## Preparing a virtual machine template

The template is created from a virtual machine (VM) with the OS cloud image attached. To prepare the template:

1. Open the "Compute → Virtual Machines" section and create a VM.
1. Set the following parameters:

   - "Template": `Blank`.
   - "Operating System": The OS of the uploaded cloud image.
   - "Optimized for": "High Performance".
   - "Instance Images": Click "Attach", select the uploaded cloud image, select the checkbox in the "OS" column to make the disk bootable, and select the "VirtIO" interface.

   ![Setting general VM parameters](../../../../images/cloud-provider-zvirt/template/vm-attach-disk-en.png)

1. Do not add network interfaces. Deckhouse Platform creates a network interface when it creates a VM for a node.

   ![Skipping network interface setting](../../../../images/cloud-provider-zvirt/template/vm-network-interfaces-en.png)

1. Leave the other parameters at their defaults and create the VM.
1. After the VM is created, select it in the list, open the additional actions menu (⋮), and select "Make Template".

   ![Creating a template based on a VM](../../../../images/cloud-provider-zvirt/template/vm-make-template-menu-en.png)

1. Specify the template name and enable the "Seal Template (Linux only)" parameter.

   ![Template parameters](../../../../images/cloud-provider-zvirt/template/template-dialog-en.png)

1. Make sure the template is displayed in the "Compute → Templates" section.

   ![Checking the created template](../../../../images/cloud-provider-zvirt/template/template-list-en.png)

## Credentials Secret

The zVirt user login and password are stored in a separate Secret rather than in the [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig). The [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/) module reads it and passes the credentials to its components that access the zVirt API.

The Secret is created during cluster installation together with the other resources of the initial configuration. The Secret must meet the following requirements:

- The name is `d8-credentials` and the namespace is `d8-cloud-provider-zvirt`.
- The type is `cloud-provider.deckhouse.io/credentials`.
- The `authScheme` field is set to `userPassword`.
- The `identity` field contains the zVirt user login.
- The `secret` field contains the zVirt user password.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-zvirt
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: userPassword
  identity: <USERNAME>
  secret: <PASSWORD>
```

Replace `<USERNAME>` and `<PASSWORD>` with the zVirt user login and password.

A Secret that does not meet these requirements fails the check both during cluster installation and when applied in a running cluster.

To change the credentials, update the `identity` and `secret` fields:

```shell
d8 k -n d8-cloud-provider-zvirt edit secret d8-credentials
```

In clusters migrated to ModuleConfig, the login and password are moved to this Secret from the [`provider.username`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-provider-username) and [`provider.password`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-provider-password) parameters of the [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) resource.
