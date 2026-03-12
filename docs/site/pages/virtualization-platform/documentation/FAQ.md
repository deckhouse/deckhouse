---
title: "Deckhouse Virtualization Platform"
permalink: en/virtualization-platform/documentation/faq.html
---

## Installing OS in a virtual machine from an ISO image

Let's look at an example of installing an OS from an ISO image of Windows OS.

To do this, download and publish it on any HTTP service accessible from the cluster.

1. Create an empty disk for OS installation:

    ```bash
    d8 k apply -f -<<EOF
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualDisk
    metadata:
    name: win-disk
    namespace: default
    spec:
    persistentVolumeClaim:
    size: 100Gi
    storageClassName: local-path
    EOF
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
    bootloader:EFI
    CPU:
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

1. After creating the resource, start the virtual machine:

    ```bash
    d8 v vnc -n default win-vm
    ```

1. Connect to it using the graphical installer and complete the OS and `virtio` driver installation:

    ```console
    d8 v vnc -n default win-vm
    ```

1. After the installation is complete, restart the virtual machine.

1. To continue working with it, use the following command:

   ```bash
   d8 v vnc -n default win-vm
   ```

## Providing a Windows answer file (Sysprep)

To perform an unattended installation of Windows,
create answer file (usually named unattend.xml or autounattend.xml).
For example, let's take a file that allows you to:

- Add English language and keyboard layout
- Specify the location of the virtio drivers needed for the installation
  (hence the order of disk devices in the VM specification is important)
- Partition the disks for installing windows on a VM with EFI
- Create an user with name *cloud* and the password *cloud* in the Administrators group
- Create a non-privileged user with name *user* and the password *user*

{% offtopic title="autounattend.xml" %}

```xml
<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State">
  <settings pass="offlineServicing"></settings>
  <settings pass="windowsPE">
    <component name="Microsoft-Windows-International-Core-WinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <SetupUILanguage>
        <UILanguage>ru-EN</UILanguage>
      </SetupUILanguage>
      <InputLocale>0409:00000409;0419:00000419</InputLocale>
      <SystemLocale>en-US</SystemLocale>
      <UILanguage>ru-En</UILanguage>
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
      <InputLocale>0409:00000409;0419:00000419</InputLocale>
      <SystemLocale>en-US</SystemLocale>
      <UILanguage>ru-RU</UILanguage>
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

{% endofftopic %}

Create a secret from this xml file:

```bash
d8 k create secret generic sysprep-config --type="provisioning.virtualization.deckhouse.io/sysprep" --from-file=./autounattend.xml
```

Then you can create a virtual machine that will use an answer file during installation.
To provide the Windows virtual machine with the answer file,
you need to specify provisioning with the type SysprepRef.
You can also specify here other files in Base64 format (customize.ps1, id_ed25519.pub, ...)
that you need to successfully execute scripts inside the answer file.

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

## Using cloud-init to configure virtual machines

Cloud-Init is a tool for automatically configuring virtual machines on first boot. The configuration is written in YAML format and must start with the `#cloud-config` header.

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

## How to use Ansible to provision virtual machines?

[Ansible](https://docs.ansible.com/ansible/latest/index.html) is an automation tool that helps you to run tasks on remote servers via SSH. In this example, we will show you how to use Ansible to manage virtual machines in a `demo-app` project.

The following assumptions will be used:

- There is a frontend virtual machine in a `demo-app` project.
- A `cloud` user is set up on the virtual machine for SSH access.
- The SSH private key for the `cloud` user is stored in the `/home/user/.ssh/id_rsa` file on the Ansible server.

Ansible `inventory` file example:

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
```

Example output:

```console
frontend.demo-app | CHANGED | rc=0 >>
12:01:20 up 2 days, 4:59, 0 users, load average: 0.00, 0.00, 0.00
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

## Automatic Ansible inventory generation

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

## Redirecting traffic to a virtual machine

The virtual machine operates within a Kubernetes cluster, so directing network traffic to it is similar to routing traffic to pods. To route network traffic to a virtual machine, Kubernetes uses a standard mechanism — the Service resource, which selects target objects using labels selectors.

1. Create a Service with the required settings:

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

## Changing virtual machine labels without having to restart

You can change the labels of a virtual machine without having to restart it, which allows you to configure real-time redirection of network traffic between different services.

Let's assume that a new service has been created and you want to redirect traffic to the virtual machine from this service:

```yaml
apiVersion: v1
kind: Service
metadata:
name: svc-2
spec:
ports:
- name: http
port: 8080
protocol: TCP
targetPort: 80
selector:
app: new
```

When you change the label on a virtual machine, traffic from the `svc-2` service will be redirected to the virtual machine:

```yaml
metadata:
labels:
app: old
```
