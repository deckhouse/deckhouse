---
title: "FAQ: Работа с ВМ"
permalink: ru/virtualization-platform/documentation/faq/vm-operations.html
lang: ru
---

## Установка и настройка ОС

### Как установить ОС в виртуальной машине из ISO-образа?

Ниже приведён типовой сценарий установки гостевой ОС Windows из ISO-образа. Перед началом разместите ISO-образ на HTTP-ресурсе, доступном из кластера.

1. Создайте пустой [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) для установки ОС:

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

1. Создайте ресурсы [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) для ISO-образа ОС Windows и дистрибутива драйверов `VirtIO`:

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

1. Запустите виртуальную машину:

   ```bash
   d8 v start win-vm
   ```

1. Подключитесь к консоли ВМ и завершите установку ОС и драйверов `VirtIO` при помощи графического установщика.

   Подключение по VNC:

   ```bash
   d8 v vnc -n default win-vm
   ```

1. После завершения установки перезагрузите виртуальную машину.

1. Для дальнейшей работы снова подключитесь по VNC:

   ```bash
   d8 v vnc -n default win-vm
   ```

### Как предоставить файл ответов Windows (Sysprep)?

Автоматическая установка Windows выполняется с файлом ответов (`unattend.xml` или `autounattend.xml`).

В примере ниже файл ответов:

- задаёт русский язык интерфейса и раскладку;
- подключает драйверы `VirtIO` для этапа установки (порядок устройств в `blockDeviceRefs` у ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) должен совпадать с путями в файле);
- создаёт разметку диска для установки с EFI;
- создаёт пользователя `cloud` (администратор, пароль `cloud`) и пользователя `user` (пароль `user`).

{% offtopic title="Пример содержимого файла autounattend.xml..." %}

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

1. Сохраните файл ответов в `autounattend.xml` (воспользуйтесь примером из блока выше или измените его под свои требования).

1. Создайте секрет с типом `provisioning.virtualization.deckhouse.io/sysprep`:

   ```bash
   d8 k create secret generic sysprep-config --type="provisioning.virtualization.deckhouse.io/sysprep" --from-file=./autounattend.xml
   ```

1. Создайте виртуальную машину, которая в процессе установки будет использовать файл ответов. Укажите в спецификации `provisioning` с типом `SysprepRef`. При необходимости добавьте в спецификацию другие файлы в формате Base64, необходимые для успешного выполнения скриптов внутри файла ответов:

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

### Как создать golden image для Linux?

Golden image — это предварительно настроенный образ виртуальной машины, который можно использовать для быстрого создания новых ВМ с уже установленным программным обеспечением и настройками.

1. Создайте виртуальную машину, установите на неё необходимое программное обеспечение и выполните все требуемые настройки.

1. Установите и настройте `qemu-guest-agent` (рекомендуется):

   - Для RHEL/CentOS:

   ```bash
   yum install -y qemu-guest-agent
   ```

   - Для Debian/Ubuntu:

   ```bash
   apt-get update
   apt-get install -y qemu-guest-agent
   ```

1. Включите и запустите сервис:

   ```bash
   systemctl enable qemu-guest-agent
   systemctl start qemu-guest-agent
   ```

1. Установите политику запуска ВМ [runPolicy: AlwaysOnUnlessStoppedManually](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-runpolicy) — это потребуется, чтобы ВМ можно было выключить.

1. Подготовьте образ. Очистите неиспользуемые блоки файловой системы:

   ```bash
   fstrim -v /
   fstrim -v /boot
   ```

1. Очистите сетевые настройки:

   - Для RHEL:

   ```bash
   nmcli con delete $(nmcli -t -f NAME,DEVICE con show | grep -v ^lo: | cut -d: -f1)
   rm -f /etc/sysconfig/network-scripts/ifcfg-eth*
   ```

   - Для Debian/Ubuntu:

   ```bash
   rm -f /etc/network/interfaces.d/*
   ```

1. Очистите системные идентификаторы:

   ```bash
   echo -n > /etc/machine-id
   rm -f /var/lib/dbus/machine-id
   ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Удалите SSH host keys:

   ```bash
   rm -f /etc/ssh/ssh_host_*
   ```

1. Очистите systemd journal:

   ```bash
   journalctl --vacuum-size=100M --vacuum-time=7d
   ```

1. Очистите кеш пакетных менеджеров:

   - Для RHEL:

   ```bash
   yum clean all
   ```

   - Для Debian/Ubuntu:

   ```bash
   apt-get clean
   ```

1. Очистите временные файлы:

   ```bash
   rm -rf /tmp/*
   rm -rf /var/tmp/*
   ```

1. Очистите логи:

   ```bash
   find /var/log -name "*.log" -type f -exec truncate -s 0 {} \;
   ```

1. Очистите историю команд:

   ```bash
   history -c
   ```

   Для RHEL: выполните сброс и восстановление контекстов SELinux (выберите один из вариантов):

   - Вариант 1: Проверка и восстановление контекстов немедленно:

     ```bash
     restorecon -R /
     ```

   - Вариант 2: Запланировать `relabel` при следующей загрузке:

     ```bash
     touch /.autorelabel
     ```

1. Проверьте, что в `/etc/fstab` указаны UUID или `LABEL`, а не имена вида `/dev/sdX`:

   ```bash
   blkid
   cat /etc/fstab
   ```

1. Сбросьте состояние cloud-init (логи и seed):

   ```bash
   cloud-init clean --logs --seed
   ```

1. Выполните финальную синхронизацию и очистку буферов:

   ```bash
   sync
   echo 3 > /proc/sys/vm/drop_caches
   ```

1. Выключите виртуальную машину:

   ```bash
   poweroff
   ```

1. Создайте ресурс [VirtualImage](/modules/virtualization/cr.html#virtualimage), указав исходный ресурс [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) подготовленной ВМ:

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

   Либо создайте ресурс [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), чтобы образ был доступен на уровне кластера для всех проектов:

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

1. Создайте новый ресурс [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) из полученного образа:

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

После выполнения всех шагов у вас будет Golden image, который можно использовать для быстрого создания новых виртуальных машин с предустановленным программным обеспечением и настройками.
