---
title: How to install an operating system in a virtual machine from an ISO image?
sections:
- vm_operations
lang: en
---

Below is a typical Windows guest OS installation scenario from an ISO image. Before you begin, host the ISO on an HTTP endpoint reachable from the cluster.

1. Create an empty [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) for OS installation:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: win-disk
     namespace: default
   spec:
     persistentVolumeClaim:
       size: 100Gi
       storageClassName: local-path
   ```

1. Create [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) resources for the Windows OS ISO and the VirtIO driver ISO:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: win-11-iso
   spec:
     dataSource:
       type: HTTP
       http:
         url: "http://example.com/win11.iso"
   ```

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: win-virtio-iso
   spec:
     dataSource:
       type: HTTP
       http:
         url: "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso"
   ```

1. Create a virtual machine:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: win-vm
     namespace: default
     labels:
       vm: win
   spec:
     virtualMachineClassName: generic
     runPolicy: Manual
     osType: Windows
     bootloader: EFI
     cpu:
       cores: 6
       coreFraction: 50%
     memory:
       size: 8Gi
     enableParavirtualization: true
     blockDeviceRefs:
       - kind: VirtualDisk
         name: win-disk
       - kind: ClusterVirtualImage
         name: win-11-iso
       - kind: ClusterVirtualImage
         name: win-virtio-iso
   ```

1. Start the virtual machine:

   ```bash
   d8 v start win-vm
   ```

1. Connect to the VM console and complete the OS installation and VirtIO drivers using the graphical installer.

   VNC connection:

   ```bash
   d8 v vnc -n default win-vm
   ```

1. After the installation is complete, restart the virtual machine.

1. For further work, connect via VNC again:

   ```bash
   d8 v vnc -n default win-vm
   ```
