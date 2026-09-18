---
title: How to create a golden image for Linux?
subsystems:
- virtualization
lang: en
---

A golden image is a pre-configured virtual machine (VM) image that can be used to quickly create new VMs with pre-installed software and settings.

1. Create a virtual machine, install the required software on it, and perform all necessary configurations.

1. Install and configure qemu-guest-agent (recommended):

   - For RHEL/CentOS:

     ```shell
     yum install -y qemu-guest-agent
     ```

   - For Debian/Ubuntu:

     ```shell
     apt-get update
     apt-get install -y qemu-guest-agent
     ```

1. Enable and start the service:

   ```shell
   systemctl enable qemu-guest-agent
   systemctl start qemu-guest-agent
   ```

1. Set the machine run policy to [`AlwaysOnUnlessStoppedManually`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-runpolicy), otherwise you will not be able to shut it down.

1. Prepare the image. Clean unused filesystem blocks:

   ```shell
   fstrim -v /
   fstrim -v /boot
   ```

1. Clean network settings:

   - For RHEL:

     ```shell
     nmcli con delete $(nmcli -t -f NAME,DEVICE con show | grep -v ^lo: | cut -d: -f1)
     rm -f /etc/sysconfig/network-scripts/ifcfg-eth*
     ```

   - For Debian/Ubuntu:

     ```shell
     rm -f /etc/network/interfaces.d/*
     ```

1. Clean system identifiers:

   ```shell
   echo -n > /etc/machine-id
   rm -f /var/lib/dbus/machine-id
   ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Remove the SSH host keys:

   ```shell
   rm -f /etc/ssh/ssh_host_*
   ```

1. Clean the systemd journal:

   ```shell
   journalctl --vacuum-size=100M --vacuum-time=7d
   ```

1. Clean package manager cache:

   - For RHEL:

     ```shell
     yum clean all
     ```

   - For Debian/Ubuntu:

     ```shell
     apt-get clean
     ```

1. Clean temporary files:

   ```shell
   rm -rf /tmp/*
   rm -rf /var/tmp/*
   ```

1. Clean logs:

   ```shell
   find /var/log -name "*.log" -type f -exec truncate -s 0 {} \;
   ```

1. Clean command history:

   ```shell
   history -c
   ```

1. On RHEL, reset and restore the SELinux contexts in one of two ways.

   Restore the contexts right away:

   ```shell
   restorecon -R /
   ```

   Or schedule a relabel for the next boot:

   ```shell
   touch /.autorelabel
   ```

1. Verify that `/etc/fstab` references UUID or `LABEL` rather than names like `/dev/sdX`:

   ```shell
   blkid
   cat /etc/fstab
   ```

1. Reset cloud-init state (logs and seed):

   ```shell
   cloud-init clean --logs --seed
   ```

1. Perform final synchronization and buffer cleanup:

   ```shell
   sync
   echo 3 > /proc/sys/vm/drop_caches
   ```

1. Shut down the virtual machine:

   ```shell
   poweroff
   ```

1. Create a [VirtualImage](/modules/virtualization/cr.html#virtualimage) resource that references the prepared VM’s [VirtualDisk](/modules/virtualization/cr.html#virtualdisk):

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: <IMAGE_NAME>
     namespace: <NAMESPACE>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualDisk
         name: <SOURCE_DISK_NAME>
   EOF
   ```

   Or create a [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) resource so the image is available cluster-wide for all projects:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: <IMAGE_NAME>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualDisk
         name: <SOURCE_DISK_NAME>
         namespace: <NAMESPACE>
   EOF
   ```

   Here, `<IMAGE_NAME>` is the name of the image being created, `<NAMESPACE>` is the namespace of the prepared machine, and `<SOURCE_DISK_NAME>` is the name of its disk.

1. Create a new [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) from the resulting image:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: <VM_DISK_NAME>
     namespace: <NAMESPACE>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualImage
         name: <IMAGE_NAME>
   EOF
   ```

   Here, `<VM_DISK_NAME>` is the name of the disk of the new machine.

After completing these steps, you will have a golden image that can be used to quickly create new virtual machines with pre-installed software and configurations.
