---
title: "Deckhouse Virtualization Platform"
permalink: ru/virtualization-platform/documentation/faq.html
lang: ru
---

## Установка ОС в виртуальной машине из ISO-образа

Рассмотрим пример установки ОС из ISO-образа ОС Windows.
Для этого загрузите и опубликуйте его на каком-либо HTTP-сервисе, доступном из кластера.

1. Создайте пустой диск для установки ОС:

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

1. Создайте ресурсы с ISO-образами ОС Windows и драйверами virtio:

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

1. Создайте виртуальную машину:

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

1. После создания ресурса запустите ВМ:

    ```bash
    d8 v vnc -n default win-vm
    ```

1. К ней необходимо подключиться и с помощью графического установщика и выполнить установку ОС и драйверов `virtio`:

   ```console
   d8 v vnc -n default win-vm
   ```

1. После окончания установки перезагрузите виртуальную машину.

1. Для продолжения работы с виртуальной машиной также используйте команду:

   ```bash
   d8 v vnc -n default win-vm
   ```

## Предоставление файла ответов Windows (Sysprep)

Чтобы выполнить автоматическую установку Windows, создайте файл ответов (обычно именуются unattend.xml или autounattend.xml).
Для примера возьмем файл, позволяющий:
- Добавить русский язык и раскладку;
- Указать расположение virtio драйверов, необходимых для установки (поэтому важен порядок дисковых устройств в спецификации ВМ);
- Разметить диски для установки Windows на ВМ c EFI;
- Создать в группе администраторов пользователя *cloud* с паролем *cloud*;
- Создать непривилегированного пользователя *user* с паролем *user*.

{% offtopic title="autounattend.xml" %}

```xml
<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State">
  <settings pass="offlineServicing"></settings>
  <settings pass="windowsPE">
    <component name="Microsoft-Windows-International-Core-WinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS">
      <SetupUILanguage>
        <UILanguage>ru-RU</UILanguage>
      </SetupUILanguage>
      <InputLocale>0409:00000409;0419:00000419</InputLocale>
      <SystemLocale>en-US</SystemLocale>
      <UILanguage>ru-RU</UILanguage>
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

Создайте секрет из этого xml файла:

```bash
d8 k create secret generic sysprep-config --type="provisioning.virtualization.deckhouse.io/sysprep" --from-file=./autounattend.xml
```

Затем можно создать виртуальную машину, которая в процессе установки будет использовать файл ответов.
Чтобы предоставить виртуальной машине Windows файл ответов, необходимо указать provisioning с типом SysprepRef.
Вы также можете указать здесь другие файлы в формате base64 (customize.ps1, id_rsa.pub,...),
необходимые для успешного выполнения скриптов внутри файла ответов.

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

## Перенаправление трафика на виртуальную машину

Виртуальная машина функционирует в кластере Kubernetes, поэтому направление сетевого трафика осуществляется аналогично направлению трафика на поды.

В качестве примера приведена виртуальная машина с HTTP-сервисом, опубликованным на порте 80, и следующим набором меток:

1. Создайте сервис с требуемыми настройками. В качестве примера приведена виртуальная машина с HTTP-сервисом, опубликованным на порте 80, и следующим набором меток:

    ```yaml
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualMachine
    metadata:
      name: web
      labels:
        vm: web
    spec: ...
    ```

1. Чтобы направить сетевой трафик на 80-й порт виртуальной машины, создайте сервис:

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

## Изменение меток виртуальной машины без необходимости перезапуска

Можно изменять метки виртуальной машины без необходимости перезапуска, что позволяет настраивать перенаправление сетевого трафика между различными сервисами в реальном времени.

Предположим, что был создан новый сервис и требуется перенаправить трафик на виртуальную машину от этого сервиса:

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

При изменении метки на виртуальной машине, трафик с сервиса `svc-2` будет перенаправлен на виртуальную машину:

```yaml
metadata:
labels:
  app: old
```
