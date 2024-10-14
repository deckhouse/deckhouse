---
title: "Cloud provider — zVirt: Preparing environment"
description: "Configuring zVirt for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

### Prepare an operating system image

Operating system vendors typically provide special cloud builds of their operating systems for use in virtualization environments. These builds typically contain virtual hardware drivers, cloud-init, virtualization guest agents, and are distributed as IMG or QCOW2 disk images. We recommend that you use these cloud images as the OS on the nodes in your clusters.
The cloud image of the operating system must be placed in the zVirt disk storage. Follow these steps to upload the OS image to the storage:

1. Go to the administration portal to the section _zVirt -> Storage -> Disks_.
2. Upload the cloud image of the OS to the repository.

   ![ Start loading the cloud-image of the OS into the repository ](../../images/030-cloud-provider-zvirt/template/step_env_en_01.png)

   ![ Uploading a cloud-image of the OS to the repository ](../../images/030-cloud-provider-zvirt/template/step_env_en_02.png)

3. Wait until the image has finished loading into the storage. The status “OK” should appear in the “Status” column. This completes the preparation of the OS image.

   ![ Finalizing the download of the cloud-image of the OS to the repository ](../../images/030-cloud-provider-zvirt/template/step_env_en_03.png)

### Prepare a virtual machine template

To create a virtual machine template, go to the _Compute -> Virtual Machines_ section of the zVirt Administration Portal and create a new virtual machine with the following parameters:

- Section _General_:
  - **Template:** `Blank`
  - **Operating system:** depending on the OS in the cloud image.
  - **Optimized for:** High Performance.
  - **Instance Images:** Attach -> Select the previously downloaded cloud-image of the OS, check “Bootable”. No other disks need to be created or attached.

    ![ General section ](../../images/030-cloud-provider-zvirt/template/step_env_en_04.png)

  - Network interfaces do not need to be attached, Deckhouse will create and configure them on the virtual machine by itself during installation.

    ![ Network interfaces ](../../images/030-cloud-provider-zvirt/template/step_env_en_05.png)

- Leave the rest of the parameters as default and create a virtual machine. When the VM creation process is finished, create a template based on it:

  - Select the created VM in the list and go to the window of creating a new template.

    ![ VM creation ](../../images/030-cloud-provider-zvirt/template/step_env_en_07.png)

  - Fill in the template parameters, only the name is mandatory. Enable the parameter _Seal Template (Linux only)_:

    ![ Template parameters ](../../images/030-cloud-provider-zvirt/template/step_env_en_08.png)

While the template is being created, the virtual machine and the cloud image disk will go into a state of _Image locked_. You can check that the template was successfully created in the section _Compute -> Templates_:

![ Template check ](../../images/030-cloud-provider-zvirt/template/step_env_en_09.png)
