---
title: How to create a golden image for Linux?
section: vm_operations
lang: en
---

A golden image is a pre-configured virtual machine image that can be used to quickly create new VMs with pre-installed software and settings.

1. Create a virtual machine, install the required software on it, and perform all necessary configurations.

1. Install and configure qemu-guest-agent (recommended):

   - For RHEL/CentOS:

     ```bash
     yum install -y qemu-guest-agent
     ```

   - For Debian/Ubuntu:

     ```bash
     apt-get update
     apt-get install -y qemu-guest-agent
     ```

1. Enable and start the service:

   ```bash
   systemctl enable qemu-guest-agent
   systemctl start qemu-guest-agent
   ```

1. Set the VM run policy to [runPolicy: AlwaysOnUnlessStoppedManually](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-runpolicy) — this is required so you can shut down the VM.

1. Prepare the image. Clean unused filesystem blocks:

   ```bash
   fstrim -v /
   fstrim -v /boot
   ```

1. Clean network settings:

   - For RHEL:

     ```bash
     nmcli con delete $(nmcli -t -f NAME,DEVICE con show | grep -v ^lo: | cut -d: -f1)
     rm -f /etc/sysconfig/network-scripts/ifcfg-eth*
     ```

   - For Debian/Ubuntu:

     ```bash
     rm -f /etc/network/interfaces.d/*
     ```

1. Clean system identifiers:

   ```bash
   echo -n > /etc/machine-id
   rm -f /var/lib/dbus/machine-id
   ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Remove SSH host keys:

   ```bash
   rm -f /etc/ssh/ssh_host_*
   ```

1. Clean systemd journal:

   ```bash
   journalctl --vacuum-size=100M --vacuum-time=7d
   ```

1. Clean package manager cache:

   - For RHEL:

     ```bash
     yum clean all
     ```

   - For Debian/Ubuntu:

     ```bash
     apt-get clean
     ```

1. Clean temporary files:

   ```bash
   rm -rf /tmp/*
   rm -rf /var/tmp/*
   ```

1. Clean logs:

   ```bash
   find /var/log -name "*.log" -type f -exec truncate -s 0 {} \;
   ```

1. Clean command history:

   ```bash
   history -c
   ```

   For RHEL: reset and restore SELinux contexts (choose one of the following):

   - Option 1: Check and restore contexts immediately:

     ```bash
     restorecon -R /
     ```

   - Option 2: Schedule relabel on next boot:

     ```bash
     touch /.autorelabel
     ```

1. Verify that `/etc/fstab` references UUID or `LABEL` rather than names like `/dev/sdX`:

   ```bash
   blkid
   cat /etc/fstab
   ```

1. Reset cloud-init state (logs and seed):

   ```bash
   cloud-init clean --logs --seed
   ```

1. Perform final synchronization and buffer cleanup:

   ```bash
   sync
   echo 3 > /proc/sys/vm/drop_caches
   ```

1. Shut down the virtual machine:

   ```bash
   poweroff
   ```

1. Create a [VirtualImage](/modules/virtualization/cr.html#virtualimage) resource that references the prepared VM’s [VirtualDisk](/modules/virtualization/cr.html#virtualdisk):

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: <image-name>
     namespace: <namespace>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualDisk
         name: <source-disk-name>
   EOF
   ```

   Or create a [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) resource so the image is available cluster-wide for all projects:

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: <image-name>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualDisk
         name: <source-disk-name>
         namespace: <namespace>
   EOF
   ```

1. Create a new [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) from the resulting image:

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: <vm-disk-name>
     namespace: <namespace>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualImage
         name: <image-name>
   EOF
   ```

After completing these steps, you will have a golden image that can be used to quickly create new virtual machines with pre-installed software and configurations.
