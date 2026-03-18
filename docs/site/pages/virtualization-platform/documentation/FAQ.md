---
title: "Deckhouse Virtualization Platform"
permalink: en/virtualization-platform/documentation/faq.html
---

## How to install an operating system in a virtual machine from an iso-image?

Let's consider installing an operating system in a virtual machine from an iso-image,
using Windows OS installation as an example.

To install the OS we will need an iso-image of Windows OS.
We need to download it and publish it on some http-service available from the cluster.

1. Create an empty disk for OS installation:

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

1. Create resources with iso-images of Windows OS and virtio drivers:

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

1. After creating the resource, start the VM:

   ```bash
   d8 v start win-vm
   ```

1. You need to connect to it and use the graphical wizard to add the `virtio` drivers
   and perform the OS installation.

   Command to connect:

   ```bash
   d8 v vnc -n default win-vm
   ```

1. After the installation is complete, restart the virtual machine.

1. To continue working with it, use the following command:

   ```bash
   d8 v vnc -n default win-vm
   ```

## How to provide windows answer file(Sysprep)?

To perform an unattended installation of Windows,
create answer file (usually named unattend.xml or autounattend.xml).
For example, let's take a file that allows you to:

- Add English language and keyboard layout
- Specify the location of the virtio drivers needed for the installation
  (hence the order of disk devices in the VM specification is important)
- Partition the disks for installing windows on a VM with EFI
- Create an user with name *cloud* and the password *cloud* in the Administrators group
- Create a non-privileged user with name *user* and the password *user*

<details><summary><b>autounattend.xml</b></summary>

```xml
<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State">
  <settings pass="offlineServicing"></settings>
  <settings pass="windowsPE">
    <component name="Microsoft-Windows-International-Core-WinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <SetupUILanguage>
        <UILanguage>en-US</UILanguage>
      </SetupUILanguage>
      <InputLocale>0409:00000409</InputLocale>
      <SystemLocale>en-US</SystemLocale>
      <UILanguage>en-US</UILanguage>
      <UserLocale>en-US</UserLocale>
    </component>
    <component name="Microsoft-Windows-PnpCustomizationsWinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <DriverPaths>
        <PathAndCredentials wcm:keyValue="4b29ba63" wcm:action="add">
          <Path>E:\amd64\w11</Path>
        </PathAndCredentials>
        <PathAndCredentials wcm:keyValue="25fe51ea" wcm:action="add">
          <Path>E:\NetKVM\w11\amd64</Path>
        </PathAndCredentials>
      </DriverPaths>
    </component>
    <component name="Microsoft-Windows-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <DiskConfiguration>
        <Disk wcm:action="add">
          <DiskID>0</DiskID>
          <WillWipeDisk>true</WillWipeDisk>
          <CreatePartitions>
            <!-- Recovery partition -->
            <CreatePartition wcm:action="add">
              <Order>1</Order>
              <Type>Primary</Type>
              <Size>250</Size>
            </CreatePartition>
            <!-- EFI system partition (ESP) -->
            <CreatePartition wcm:action="add">
              <Order>2</Order>
              <Type>EFI</Type>
              <Size>100</Size>
            </CreatePartition>
            <!-- Microsoft reserved partition (MSR) -->
            <CreatePartition wcm:action="add">
              <Order>3</Order>
              <Type>MSR</Type>
              <Size>128</Size>
            </CreatePartition>
            <!-- Windows partition -->
            <CreatePartition wcm:action="add">
              <Order>4</Order>
              <Type>Primary</Type>
              <Extend>true</Extend>
            </CreatePartition>
          </CreatePartitions>
          <ModifyPartitions>
            <!-- Recovery partition -->
            <ModifyPartition wcm:action="add">
              <Order>1</Order>
              <PartitionID>1</PartitionID>
              <Label>Recovery</Label>
              <Format>NTFS</Format>
              <TypeID>de94bba4-06d1-4d40-a16a-bfd50179d6ac</TypeID>
            </ModifyPartition>
            <!-- EFI system partition (ESP) -->
            <ModifyPartition wcm:action="add">
              <Order>2</Order>
              <PartitionID>2</PartitionID>
              <Label>System</Label>
              <Format>FAT32</Format>
            </ModifyPartition>
            <!-- MSR partition does not need to be modified -->
            <!-- Windows partition -->
            <ModifyPartition wcm:action="add">
              <Order>3</Order>
              <PartitionID>4</PartitionID>
              <Label>Windows</Label>
              <Letter>C</Letter>
              <Format>NTFS</Format>
            </ModifyPartition>
          </ModifyPartitions>
        </Disk>
        <WillShowUI>OnError</WillShowUI>
      </DiskConfiguration>
      <ImageInstall>
        <OSImage>
          <InstallTo>
            <DiskID>0</DiskID>
            <PartitionID>4</PartitionID>
          </InstallTo>
        </OSImage>
      </ImageInstall>
      <UserData>
        <ProductKey>
          <Key>VK7JG-NPHTM-C97JM-9MPGT-3V66T</Key>
          <WillShowUI>OnError</WillShowUI>
        </ProductKey>
        <AcceptEula>true</AcceptEula>
      </UserData>
      <UseConfigurationSet>false</UseConfigurationSet>
    </component>
  </settings>
  <settings pass="generalize"></settings>
  <settings pass="specialize">
    <component name="Microsoft-Windows-Deployment" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <RunSynchronous>
        <RunSynchronousCommand wcm:action="add">
          <Order>1</Order>
          <Path>powershell.exe -NoProfile -Command "$xml = [xml]::new(); $xml.Load('C:\Windows\Panther\unattend.xml'); $sb = [scriptblock]::Create( $xml.unattend.Extensions.ExtractScript ); Invoke-Command -ScriptBlock $sb -ArgumentList $xml;"</Path>
        </RunSynchronousCommand>
        <RunSynchronousCommand wcm:action="add">
          <Order>2</Order>
          <Path>powershell.exe -NoProfile -Command "Get-Content -LiteralPath 'C:\Windows\Setup\Scripts\Specialize.ps1' -Raw | Invoke-Expression;"</Path>
        </RunSynchronousCommand>
        <RunSynchronousCommand wcm:action="add">
          <Order>3</Order>
          <Path>reg.exe load "HKU\DefaultUser" "C:\Users\Default\NTUSER.DAT"</Path>
        </RunSynchronousCommand>
        <RunSynchronousCommand wcm:action="add">
          <Order>4</Order>
          <Path>powershell.exe -NoProfile -Command "Get-Content -LiteralPath 'C:\Windows\Setup\Scripts\DefaultUser.ps1' -Raw | Invoke-Expression;"</Path>
        </RunSynchronousCommand>
        <RunSynchronousCommand wcm:action="add">
          <Order>5</Order>
          <Path>reg.exe unload "HKU\DefaultUser"</Path>
        </RunSynchronousCommand>
      </RunSynchronous>
    </component>
  </settings>
  <settings pass="auditSystem"></settings>
  <settings pass="auditUser"></settings>
  <settings pass="oobeSystem">
    <component name="Microsoft-Windows-International-Core" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <InputLocale>0409:00000409</InputLocale>
      <SystemLocale>en-US</SystemLocale>
      <UILanguage>en-US</UILanguage>
      <UserLocale>en-US</UserLocale>
    </component>
    <component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <UserAccounts>
        <LocalAccounts>
          <LocalAccount wcm:action="add">
            <Name>cloud</Name>
            <DisplayName>cloud</DisplayName>
            <Group>Administrators</Group>
            <Password>
              <Value>cloud</Value>
              <PlainText>true</PlainText>
            </Password>
          </LocalAccount>
          <LocalAccount wcm:action="add">
            <Name>User</Name>
            <DisplayName>user</DisplayName>
            <Group>Users</Group>
            <Password>
              <Value>user</Value>
              <PlainText>true</PlainText>
            </Password>
          </LocalAccount>
        </LocalAccounts>
      </UserAccounts>
      <AutoLogon>
        <Username>cloud</Username>
        <Enabled>true</Enabled>
        <LogonCount>1</LogonCount>
        <Password>
          <Value>cloud</Value>
          <PlainText>true</PlainText>
        </Password>
      </AutoLogon>
      <OOBE>
        <ProtectYourPC>3</ProtectYourPC>
        <HideEULAPage>true</HideEULAPage>
        <HideWirelessSetupInOOBE>true</HideWirelessSetupInOOBE>
        <HideOnlineAccountScreens>false</HideOnlineAccountScreens>
      </OOBE>
      <FirstLogonCommands>
        <SynchronousCommand wcm:action="add">
          <Order>1</Order>
          <CommandLine>powershell.exe -NoProfile -Command "Get-Content -LiteralPath 'C:\Windows\Setup\Scripts\FirstLogon.ps1' -Raw | Invoke-Expression;"</CommandLine>
        </SynchronousCommand>
      </FirstLogonCommands>
    </component>
  </settings>
</unattend>
```

</details>

Create a secret from this xml file:

```bash
d8 k create secret generic sysprep-config --type="provisioning.virtualization.deckhouse.io/sysprep" --from-file=./autounattend.xml
```

Then you can create a virtual machine that will use an answer file during installation.
To provide the Windows virtual machine with the answer file,
you need to specify provisioning with the type SysprepRef.
You can also specify here other files in base64 format, that you need to successfully execute scripts inside the answer file.

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
  provisioning:
    type: SysprepRef
    sysprepRef:
      kind: Secret
      name: sysprep-config
  runPolicy: AlwaysOn
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

## How to use cloud-init to configure virtual machines?

Cloud-Init is a tool for automatically configuring virtual machines on first boot. The configuration is written in YAML format and must start with the `#cloud-config` header.

{% alert level="warning" %}
When using cloud images (for example, official distribution images), you must provide a cloud-init configuration. Without it, some distributions do not configure network connectivity, and the virtual machine becomes unreachable on the network, even if the main network (Main) is attached.

In addition, cloud images do not allow login by default — you must either add SSH keys for the default user or create a new user with SSH access. Otherwise, you will not be able to access the virtual machine.
{% endalert %}

### Updating and installing packages

Example configuration for updating the system and installing packages:

```yaml
#cloud-config
# Update package lists
package_update: true
# Upgrade installed packages to latest versions
package_upgrade: true
# List of packages to install
packages:
  - nginx
  - curl
  - htop
# Commands to run after package installation
runcmd:
  - systemctl enable --now nginx.service
```

### Creating a user

Example configuration for creating a user with a password and SSH key:

```yaml
#cloud-config
# List of users to create
users:
  - name: cloud                    # Username
    passwd: "$6$rounds=4096$saltsalt$..."  # Password hash (SHA-512)
    lock_passwd: false            # Do not lock the account
    sudo: ALL=(ALL) NOPASSWD:ALL  # Sudo privileges without password prompt
    shell: /bin/bash              # Default shell
    ssh-authorized-keys:          # SSH keys for access
      - ssh-ed25519 AAAAC3NzaC... your-public-key ...
# Allow password authentication via SSH
ssh_pwauth: true
```

To generate a password hash, use the `mkpasswd --method=SHA-512 --rounds=4096` command.

### Creating a file with required permissions

Example configuration for creating a file with specified access permissions:

```yaml
#cloud-config
# List of files to create
write_files:
  - path: /opt/scripts/start.sh    # File path
    content: |                     # File content
      #!/bin/bash
      echo "Starting application"
    owner: cloud:cloud            # File owner (user:group)
    permissions: '0755'           # Access permissions (octal format)
```

### Configuring disk and filesystem

Example configuration for disk partitioning, filesystem creation, and mounting:

```yaml
#cloud-config
# Disk partitioning setup
disk_setup:
  /dev/sdb:                        # Disk device
    table_type: gpt                # Partition table type (gpt or mbr)
    layout: true                   # Automatically create partitions
    overwrite: false               # Do not overwrite existing partitions

# Filesystem setup
fs_setup:
  - label: data                    # Filesystem label
    filesystem: ext4               # Filesystem type
    device: /dev/sdb1              # Partition device
    partition: auto                # Automatically detect partition

# Filesystem mounting
mounts:
  # [device, mount_point, fs_type, options, dump, pass]
  - ["/dev/sdb1", "/mnt/data", "ext4", "defaults", "0", "2"]
```

### Configuring network interfaces for additional networks

For more information on connecting additional networks to a virtual machine, see the [Additional network interfaces](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#additional-network-interfaces) section.

{% alert level="warning" %}
The settings described in this section apply only to additional networks. The main network (Main) is configured automatically via cloud-init and does not require manual configuration.
{% endalert %}

If additional networks are connected to a virtual machine, they must be configured manually via cloud-init. Use `write_files` to create configuration files and `runcmd` to apply the settings.

#### For systemd-networkd

Use the following example on distributions that use `systemd-networkd` (for example, Debian, CoreOS):

```yaml
#cloud-config
write_files:
  - path: /etc/systemd/network/10-eth1.network
    content: |
      [Match]
      Name=eth1

      [Network]
      Address=192.168.1.10/24
      Gateway=192.168.1.1
      DNS=8.8.8.8

runcmd:
  - systemctl restart systemd-networkd
```

#### For Netplan (Ubuntu)

Use the following example on Ubuntu and other distributions that use `Netplan`:

```yaml
#cloud-config
write_files:
  - path: /etc/netplan/99-custom.yaml
    content: |
      network:
        version: 2
        ethernets:
          eth1:
            addresses:
              - 10.0.0.5/24
            gateway4: 10.0.0.1
            nameservers:
              addresses: [8.8.8.8]
          eth2:
            dhcp4: true

runcmd:
  - netplan apply
```

#### For ifcfg (RHEL/CentOS)

Use the following example on RHEL, CentOS and other distributions that use the `ifcfg` scheme with `NetworkManager`:

```yaml
#cloud-config
write_files:
  - path: /etc/sysconfig/network-scripts/ifcfg-eth1
    content: |
      DEVICE=eth1
      BOOTPROTO=none
      ONBOOT=yes
      IPADDR=192.168.1.10
      PREFIX=24
      GATEWAY=192.168.1.1
      DNS1=8.8.8.8

runcmd:
  - nmcli connection reload
  - nmcli connection up eth1
```

#### For Alpine Linux

Use the following example on Alpine Linux and other distributions that use the traditional `/etc/network/interfaces` format:

```yaml
#cloud-config
write_files:
  - path: /etc/network/interfaces
    append: true
    content: |
      auto eth1
      iface eth1 inet static
          address 192.168.1.10
          netmask 255.255.255.0
          gateway 192.168.1.1

runcmd:
  - /etc/init.d/networking restart
```

## How to use Ansible to provision virtual machines?

[Ansible](https://docs.ansible.com/ansible/latest/index.html) is an automation tool that helps you to run tasks on remote servers via SSH. In this example, we will show you how to use Ansible to manage virtual machines in a demo-app project.

The following assumptions will be used:

- There is a frontend virtual machine in a demo-app project.
- A cloud user is set up on the virtual machine for SSH access.
- The SSH private key for the cloud user is stored in the /home/user/.ssh/id_rsa file on the Ansible server.

Ansible inventory file example:

```yaml
---
all:
  vars:
    ansible_ssh_common_args: '-o ProxyCommand="d8 v port-forward --stdio=true %h %h %p"'
    # Default user for SSH access.
    ansible_user: cloud
    # Path to private key.
    ansible_ssh_private_key_file: /home/user/.ssh/id_rsa
  hosts:
    # Host name in the format <VM name>.<project name>.
    frontend.demo-app:
```

To check the virtual machine's uptime value, use the following command:

```bash
ansible -m shell -a "uptime" -i inventory.yaml all
# frontend.demo-app | CHANGED | rc=0 >>
# 12:01:20 up 2 days, 4:59, 0 users, load average: 0.00, 0.00, 0.00
```

If you prefer not to use the inventory file, you can specify and pass all the parameters directly in the command line:

```bash
ansible -m shell -a "uptime" \
  -i "frontend.demo-app," \
  -e "ansible_ssh_common_args='-o ProxyCommand=\"d8 v port-forward --stdio=true %h %p %p\"'" \
  -e "ansible_user=cloud" \
  -e "ansible_ssh_private_key_file=/home/user/.ssh/id_rsa" \
  all
```

## How to automatically generate inventory for Ansible?

{% alert level="warning" %}
The `d8 v ansible-inventory` command requires `d8` version v0.27.0 or higher.
{% endalert %}

{% alert level="warning" %}
The command works only for virtual machines with the main cluster network (Main) connected.
{% endalert %}

Instead of manually creating an inventory file, you can use the `d8 v ansible-inventory` command, which automatically generates an Ansible inventory from virtual machines in the specified namespace. The command is compatible with the [ansible inventory script](https://docs.ansible.com/ansible/latest/user_guide/intro_inventory.html#inventory-scripts) interface.

The command includes only virtual machines with assigned IP addresses in the `Running` state. Host names are formatted as `<vmname>.<namespace>` (for example, `frontend.demo-app`).

If necessary, configure host variables through annotations (for example, SSH user):

```bash
d8 k -n demo-app annotate vm frontend provisioning.virtualization.deckhouse.io/ansible_user="cloud"
```

Use the command directly:

```bash
ANSIBLE_INVENTORY_ENABLED=yaml ansible -m shell -a "uptime" all -i <(d8 v ansible-inventory -n demo-app -o yaml)
```

{% alert level="info" %}
The `<(...)` construct is necessary because Ansible expects a file or script as the source of the host list. Simply specifying the command in quotes will not work — Ansible will try to execute the string as a script. The `<(...)` construct passes the command output as a file that Ansible can read.
{% endalert %}

Or save the inventory to a file:

```bash
d8 v ansible-inventory --list -o yaml -n demo-app > inventory.yaml
ansible -m shell -a "uptime" -i inventory.yaml all
```

## How to redirect traffic to a virtual machine?

The virtual machine operates within a Kubernetes cluster, so directing network traffic to it is similar to routing traffic to pods. To route network traffic to a virtual machine, Kubernetes uses a standard mechanism — the Service resource, which selects target objects using labels selectors.

1. Create a Service with the required settings.

   For example, consider a virtual machine with the label `vm: frontend-0`, an HTTP service exposed on ports 80 and 443, and SSH access on port 22:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: frontend-0
     namespace: dev
     labels:
       vm: frontend-0
   spec: ...
   ```

1. To route network traffic to the virtual machine's ports, create the following Service:

   This Service listens on ports 80 and 443 and forwards traffic to the target virtual machine’s ports 80 and 443. SSH access from outside the cluster is provided on port 2211.

   ```yaml
   apiVersion: v1
   kind: Service
   metadata:
     name: frontend-0-svc
     namespace: dev
   spec:
     type: LoadBalancer
     ports:
     - name: ssh
       port: 2211
       protocol: TCP
       targetPort: 22
     - name: http
       port: 80
       protocol: TCP
       targetPort: 80
     - name: https
       port: 443
       protocol: TCP
       targetPort: 443
     selector:
       vm: frontend-0
   ```

## How to increase the DVCR size?

To increase the disk size for DVCR, you must set a larger size in the `virtualization` module configuration than the current size.

1. Check the current DVCR size:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Example output:

   ```txt
    {"size":"58G","storageClass":"linstor-thick-data-r1"}
   ```

1. Set the size:

   ```shell
   d8 k patch mc virtualization \
     --type merge -p '{"spec": {"settings": {"dvcr": {"storage": {"persistentVolumeClaim": {"size":"59G"}}}}}}'
   ```

   Example output:

   ```txt
   moduleconfig.deckhouse.io/virtualization patched
   ```

1. Check the resizing:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Example output:

   ```txt
   {"size":"59G","storageClass":"linstor-thick-data-r1"}
   ```

1. Check the current status of the DVCR:

   ```shell
   d8 k get pvc dvcr -n d8-virtualization
   ```

   Example output:

   ```txt
   NAME STATUS VOLUME                                    CAPACITY    ACCESS MODES   STORAGECLASS           AGE
   dvcr Bound  pvc-6a6cedb8-1292-4440-b789-5cc9d15bbc6b  57617188Ki  RWO            linstor-thick-data-r1  7d
   ```

## How to create a golden image for Linux?

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

1. Set the VM run policy to [`runPolicy: AlwaysOnUnlessStoppedManually`](/modules/virtualization/stable/cr.html#virtualmachine-v1alpha2-spec-runpolicy). This is required to be able to shut down the VM.

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

1. Verify that `/etc/fstab` uses UUID or LABEL instead of device names (e.g., `/dev/sdX`). To check, run:

   ```bash
   blkid
   cat /etc/fstab
   ```

1. Clean cloud-init state, logs, and seed (recommended method):

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

1. Create a `VirtualImage` resource from the prepared VM disk:

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

   Alternatively, create a `ClusterVirtualImage` to make the image available at the cluster level for all projects:

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

1. Create a VM disk from the created image:

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

## How to restore the cluster if images from registry.deckhouse.io cannot be pulled after a license change?

After a license change on a cluster with `containerd v1` and removal of the outdated license, images from `registry.deckhouse.io` may stop being pulled. In that case, nodes still have the outdated config file `/etc/containerd/conf.d/dvcr.toml`, which is not removed automatically. Because of it, the `registry` module does not start, and without it DVCR does not work.

To fix this, apply a NodeGroupConfiguration (NGC) manifest: it will remove the file on the nodes. After the `registry` module starts, remove the manifest, since this is a one-time fix.

1. Save the manifest to a file (for example, `containerd-dvcr-remove-old-config.yaml`):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-dvcr-remove-old-config.sh
   spec:
     weight: 32 # Must be in range 32–90
     nodeGroups: ["*"]
     bundles: ["*"]
     content: |
       # Copyright 2023 Flant JSC
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #      http://www.apache.org/licenses/LICENSE-2.0
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.

       rm -f /etc/containerd/conf.d/dvcr.toml
   ```

1. Apply the manifest:

   ```bash
   d8 k apply -f containerd-dvcr-remove-old-config.yaml
   ```

1. Verify that the `registry` module is running:

   ```bash
   d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
   ```

   Example output when the `registry` module is running:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Delete the NGC manifest:

   ```bash
   d8 k delete -f containerd-dvcr-remove-old-config.yaml
   ```

For more information on migration, see [Migrating container runtime to containerd v2](/products/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/migrating.html).
