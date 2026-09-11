---
title: "Virtual machines"
permalink: en/user/virtualization/
description: "Quick start: creating a virtual machine from an image, attaching a disk, and logging in to the guest system over SSH."
search: quick start, creating a VM, first virtual machine
---

This section walks through a minimal scenario: you create an Ubuntu 24.04 image, a disk from that image, and a virtual machine (VM), connect to it over the console, and then delete the created resources.

{% tabs quickstart %}

{% tab "Using the CLI" %}

1. Create a [VirtualImage](/modules/virtualization/cr.html#virtualimage) from an external source:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: ubuntu
   spec:
     storage: ContainerRegistry
     dataSource:
       type: HTTP
       http:
         url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
   EOF
   ```

1. Create a [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) from that image. Make sure the cluster has a default StorageClass, then apply the manifest:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: linux-disk
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualImage
         name: ubuntu
   EOF
   ```

1. Create a [VirtualMachine](/modules/virtualization/cr.html#virtualmachine). The example uses a cloud-init script that creates the `cloud` user:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: linux-vm
   spec:
     virtualMachineClassName: generic
     cpu:
       cores: 1
     memory:
       size: 1Gi
     provisioning:
       type: UserData
       userData: |
         #cloud-config
         ssh_pwauth: True
         users:
           - name: cloud
             passwd: <PASSWORD_HASH>
             shell: /bin/bash
             sudo: ALL=(ALL) NOPASSWD:ALL
             lock_passwd: False
     blockDeviceRefs:
       - kind: VirtualDisk
         name: linux-disk
   EOF
   ```

   Where `<PASSWORD_HASH>` is the hash of the user password, in quotes. Generate it with `mkpasswd --method=SHA-512 --rounds=4096`: the command prompts for the password and prints the ready value. The script format is described in the [cloud-init documentation](https://cloudinit.readthedocs.io/).

1. Verify that the image and the disk are created and the VM is running. Resources don't become ready instantly, so wait for the expected values in the `PHASE` column:

   ```bash
   d8 k get vi,vd,vm
   ```

   Example output:

   ```console
   NAME                                                 PHASE   CDROM   PROGRESS   AGE
   virtualimage.virtualization.deckhouse.io/ubuntu      Ready   false   100%       7h50m

   NAME                                                 PHASE   CAPACITY   VIRTUALMACHINE   AGE
   virtualdisk.virtualization.deckhouse.io/linux-disk   Ready   4Gi        linux-vm         7h40m

   NAME                                                 PHASE     UPTIME   NODE           IPADDRESS    AGE
   virtualmachine.virtualization.deckhouse.io/linux-vm  Running   7h30m    virtlab-pt-2   10.66.10.2   7h46m
   ```
   {: .nowrap-default }

1. Connect to the VM over the console:

   ```bash
   d8 v console linux-vm
   ```

   Example output:

   ```console
   Successfully connected to linux-vm console. The escape sequence is ^]

   linux-vm login: cloud
   Password:
   ...
   cloud@linux-vm:~$
   ```
   {: .nowrap-default }

   To exit the console, press `Ctrl+]`.

1. Delete the created resources:

   ```bash
   d8 k delete vm linux-vm
   d8 k delete vd linux-disk
   d8 k delete vi ubuntu
   ```

{% endtab %}

{% tab "Using the web interface" %}

1. Create an image from an external source:

   1. Go to the **Projects** tab and select the project you need.
   1. Go to **Virtualization** → **Images**.
   1. Click **Create**.
   1. In the **Source** block, select **By link**.
   1. In the form that opens, enter `ubuntu` in the **Image name** field.
   1. In the **Storage** block, select `ContainerRegistry` in the **Storage type** field.
   1. In the **URL** field, paste `https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img`.
   1. Click **Create**.
   1. Check the image status on its page.

1. Create a disk from that image. You can skip this step and create the disk while creating the VM.

   1. Go to **Virtualization** → **Disks**.
   1. Click **Create**.
   1. In the form that opens, enter `linux-disk` in the **Disk name** field.
   1. In the **Source** field, select the `ubuntu` image from the drop-down list.
   1. If required, specify a larger size in the **Size** field, for example `5Gi`.
   1. In the **Storage class** field, select a StorageClass or keep the default one.
   1. Click **Create**.
   1. Check the disk status on its page.

   > If the selected StorageClass uses the `WaitForFirstConsumer` mode, the disk waits for the VM that uses it.
   > Until then, the disk shows the "CREATING 0%" status, but you can already select it when creating a VM.

1. Create a virtual machine:

   1. Go to **Virtualization** → **Virtual machines**.
   1. Click **Create**.
   1. In the form that opens, enter `linux-vm` in the **Name** field.
   1. In the **Platform** and **Resources** sections, keep the default settings.
   1. In the **Disks** section, click **Add**.

      If the disk is already created, select **Existing** in the **Disks / Images** window that opens and pick `linux-disk` from the list.

      If the disk isn't created, select **Create from** in the same window and set the parameters:

      - In the **Name** field, enter `linux-disk`.
      - In the **Source** field, select the `ubuntu` image from the drop-down list. The list shows the resource type.
      - If required, specify a larger size in the **Size** field, for example `5Gi`.
      - In the **Storage** field, select a StorageClass or keep the default one.

      Click **Add**.

   1. Scroll down to the **Cloud-init** toggle and enable it.
   1. Paste the script into the field that appears, replacing `<PASSWORD_HASH>` with the password hash in quotes, generated with `mkpasswd --method=SHA-512 --rounds=4096`:

      ```yaml
      #cloud-config
      ssh_pwauth: True
      users:
        - name: cloud
          passwd: <PASSWORD_HASH>
          shell: /bin/bash
          sudo: ALL=(ALL) NOPASSWD:ALL
          lock_passwd: False
      ```

   1. Click **Create**.
   1. Check the VM status on its page.

1. Connect to the VM over the console:

   1. Go to **Virtualization** → **Virtual machines**.
   1. Select the VM from the list and click its name.
   1. In the form that opens, go to the **TTY** tab and log in to the console window.

1. Delete the created resources:

   1. Go to **Virtualization** and select the section you need, for example **Virtual machines**, **Disks**, or **Images**.
   1. In the resource row, click the ellipsis button and select **Delete**. In some lists, for example in the list of VM snapshots, deletion is a separate button.
   1. In the confirmation window, click **Delete** or cancel the action with **Don't delete**.

   > **Important:** Deleting a resource is irreversible. A disk attached to a running virtual machine can't be deleted, and the **Delete** item is inactive for it.

{% endtab %}

{% endtabs %}
