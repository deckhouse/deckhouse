---
title: "Deckhouse Virtualization Platform"
permalink: en/virtualization-platform/documentation/faq.html
---

## Installing OS in a virtual machine from an ISO image

Let's look at an example of installing an OS from an ISO image of Windows OS.

To do this, download and publish it on any HTTP service accessible from the cluster.

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
You can also specify here other files in base64 format (customize.ps1, id_rsa.pub, ...)
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

## Redirecting traffic to a virtual machine

The virtual machine runs in a Kubernetes cluster, so directing network traffic is similar to directing traffic to pods.

1. Create a service with the required settings. As an example, here is a virtual machine with an HTTP service published on port 80 and the following set of labels:

    ```yaml
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualMachine
    metadata:
    name: web
    labels:
    vm: web
    spec: ...
    ```

1. To forward network traffic to port 80 of the virtual machine, create a service:

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
