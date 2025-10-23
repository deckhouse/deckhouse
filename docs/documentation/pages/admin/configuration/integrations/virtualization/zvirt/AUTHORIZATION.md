---
title: Connection and authorization
permalink: en/admin/integrations/virtualization/zvirt/authorization.html
---

## Requirements

To ensure proper operation of Deckhouse Kubernetes Platform with the zVirt cloud, the following are required:

- A working zVirt installation with accessible API.
- zVirt version `4.0`-`4.4`.
- A user account with permissions to access the API and manage virtual machines.
- A storage domain with an uploaded cloud OS image.
- A prepared virtual machine template based on the cloud image.

{% alert level="info" %}
The zVirt cloud provider is in experimental status.
API and behavior may change in future versions.
{% endalert %}

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

## Preparing a cloud image

1. Go to **zVirt** → **Storage** → **Disks**.
1. Upload a cloud OS image (for example, `.qcow2` or `.img`) to the storage.

   ![Start of uploading the cloud OS image into the repository](../../../../images/cloud-provider-zvirt/template/step_env_en_01.png)

   ![Process of uploading a cloud OS image into the repository](../../../../images/cloud-provider-zvirt/template/step_env_en_02.png)

1. Wait for the upload to complete. The disk status should display `OK`.

   ![Finishing the upload of the cloud OS image into the repository](../../../../images/cloud-provider-zvirt/template/step_env_en_03.png)

## Preparing a virtual machine template

1. Open **Resources** → **Virtual Machines** and create a new VM.

1. Set the following parameters:
   - **Template**: `Blank`.
   - **Operating system**: Matches the uploaded cloud image.
   - **Workload profile**: High performance.
   - **Disks**: Attach the uploaded cloud image and set it as bootable.

   ![Setting general VM parameters](../../../../images/cloud-provider-zvirt/template/step_env_en_04.png)

1. Do not add any network interfaces.
   They will be created automatically when Deckhouse provisions a node.

   ![Skipping network interface setting](../../../../images/cloud-provider-zvirt/template/step_env_en_05.png)

1. Save and create the virtual machine.

1. After the VM is created, open its details and create a template based on it.

   ![Creating a template based on a VM](../../../../images/cloud-provider-zvirt/template/step_env_en_07.png)

1. Specify the template name and enable the **Seal Template** option.

   ![Template parameters](../../../../images/cloud-provider-zvirt/template/step_env_en_08.png)

1. Ensure the template has been successfully created in **Resources** → **Templates**.

   ![Checking the created template](../../../../images/cloud-provider-zvirt/template/step_env_en_09.png)
