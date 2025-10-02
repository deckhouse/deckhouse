---
title: "FAQ"
weight: 70
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

## How to use Ansible to provision virtual machines?

[Ansible](https://docs.ansible.com/ansible/latest/index.html) is an automation tool that helps you to run tasks on remote servers via SSH. In this example, we will show you how to use Ansible to manage virtual machines in a demo-app project.

The following assumptions will be used:

- There is a frontend virtual machine in a demo-app project.
- A cloud user is set up on the virtual machine for SSH access.
- The SSH private key for the cloud user is stored in the ./tmp/demo file on the Ansible server.

Ansible inventory file example:

```yaml
---
all:
  vars:
    ansible_ssh_common_args: '-o ProxyCommand="d8 v port-forward --stdio=true %h %h %p"'
    # Default user for SSH access.
    ansible_user: cloud
    # Path to private key.
    ansible_ssh_private_key_file: ./tmp/demo
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
  -e "ansible_ssh_private_key_file=./tmp/demo" \
  all
```

## How to redirect traffic to a virtual machine?

Since the virtual machine runs in a Kubernetes cluster, the forwarding of network traffic to it is done similarly to the forwarding of traffic to the pods.

To do this, you just need to create a service with the required settings.

1. Suppose we have a virtual machine with http service published on port 80 and the following set of labels:

    ```yaml
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualMachine
    metadata:
      name: web
      labels:
        vm: web
    spec: ...
    ```

1. In order to direct network traffic to port 80 of the virtual machine - let's create a service:

    ```yaml
    apiVersion: v1
    kind: Service
    metadata:
      name: svc-1
    spec:
      ports:
        - name: http
          port: 8080
          protocol: TCP
          targetPort: 80
      selector:
        app: old
    ```

    We can change virtual machine label values on the fly, i.e. changing labels does not require restarting the virtual machine, which means that we can configure network traffic redirection from different services dynamically:

    Let's imagine that we have created a new service and want to redirect traffic to our virtual machine from it:

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

    By changing the labels on the virtual machine, we will redirect network traffic from the `svc-2` service to it:

    ```yaml
    metadata:
      labels:
        app: old
    ```

## How to increase the DVCR size?

To increase the disk size for DVCR, you must set a larger size in the `virtualization` module configuration than the current size.

1. Check the current DVCR size:

    ```shell
    d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
    ```

    Example output:

    ```console
    {"size":"58G","storageClass":"linstor-thick-data-r1"}
    ```

1. Set the size:

    ```shell
    d8 k patch mc virtualization \
      --type merge -p '{"spec": { "settings": { "dvcr": { "storage": { "persistentVolumeClaim": {"size": "59G"}}}}}}'
    ```

   Example output:

    ```console
    moduleconfig.deckhouse.io/virtualization patched
    ```

1. Check the resizing:

    ```shell
    d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
    ```

   Example output:

    ```console
    {"size":"59G","storageClass":"linstor-thick-data-r1"}

1. Check the current status of the DVCR:

    ```shell
    d8 k get pvc dvcr -n d8-virtualization
    ```

   Example output:

    ```console
    NAME STATUS VOLUME                                    CAPACITY    ACCESS MODES   STORAGECLASS           AGE
    dvcr Bound  pvc-6a6cedb8-1292-4440-b789-5cc9d15bbc6b  57617188Ki  RWO            linstor-thick-data-r1  7d
    ```
