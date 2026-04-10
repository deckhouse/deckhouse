---
title: "Deckhouse Virtualization Platform"
permalink: ru/virtualization-platform/documentation/faq.html
lang: ru
---

## Работа с виртуальными машинами

### Установка и настройка ОС

#### Как установить ОС в виртуальной машине из ISO-образа?

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

#### Как предоставить файл ответов Windows (Sysprep)?

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

#### Как создать golden image для Linux?

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

## Конфигурирование виртуальных машин

### Как использовать cloud-init для конфигурирования виртуальных машин?

[Cloud-init](https://cloudinit.readthedocs.io/) применяется для первичной настройки гостевой ОС при первом запуске. Конфигурация задаётся в YAML и начинается с директивы `#cloud-config`.

{% alert level="warning" %}
Для образов, рассчитанных на cloud-init (в том числе официальных cloud-образов дистрибутивов), конфигурацию cloud-init нужно передать явно: иначе на части дистрибутивов не поднимается сеть, и ВМ оказывается недоступна по сети даже при подключении основной сети (Main).

Кроме того, в cloud-образах по умолчанию отключена возможность входа в систему — необходимо добавить SSH-ключи для пользователя по умолчанию либо создать нового пользователя с SSH-доступом, иначе доступ к виртуальной машине будет невозможен.
{% endalert %}

#### Обновление и установка пакетов

Пример `cloud-config` для обновления системы и установки пакетов из списка:

```yaml
#cloud-config
# Обновить списки пакетов
package_update: true
# Обновить установленные пакеты до последних версий
package_upgrade: true
# Список пакетов для установки
packages:
  - nginx
  - curl
  - htop
# Команды для выполнения после установки пакетов
runcmd:
  - systemctl enable --now nginx.service
```

#### Создание пользователя

Пример `cloud-config` для создания локального пользователя с паролем и SSH-ключом:

```yaml
#cloud-config
# Список пользователей для создания
users:
  - name: cloud                    # Имя пользователя
    passwd: "$6$rounds=4096$saltsalt$..."  # Хеш пароля (SHA-512)
    lock_passwd: false            # Не блокировать учётную запись
    sudo: ALL=(ALL) NOPASSWD:ALL  # Права sudo без запроса пароля
    shell: /bin/bash              # Оболочка по умолчанию
    ssh-authorized-keys:          # SSH-ключи для доступа
      - ssh-ed25519 AAAAC3NzaC... your-public-key ...
# Разрешить аутентификацию по паролю через SSH
ssh_pwauth: true
```

Чтобы получить хеш пароля для поля `passwd`, выполните команду:

```shell
mkpasswd --method=SHA-512 --rounds=4096
```

#### Создание файла с нужными правами

Пример `cloud-config` для создания файла с заданными правами доступа:

```yaml
#cloud-config
# Список файлов для создания
write_files:
  - path: /opt/scripts/start.sh    # Путь к файлу
    content: |                     # Содержимое файла
      #!/bin/bash
      echo "Starting application"
    owner: cloud:cloud            # Владелец файла (пользователь:группа)
    permissions: '0755'           # Права доступа (восьмеричный формат)
```

#### Настройка диска и файловой системы

Пример `cloud-config` для разметки диска, создания файловой системы и монтирования:

```yaml
#cloud-config
# Настройка разметки диска
disk_setup:
  /dev/sdb:                        # Устройство диска
    table_type: gpt                # Тип таблицы разделов (gpt или mbr)
    layout: true                   # Автоматически создать разделы
    overwrite: false               # Не перезаписывать существующие разделы

# Настройка файловых систем
fs_setup:
  - label: data                    # Метка файловой системы
    filesystem: ext4               # Тип файловой системы
    device: /dev/sdb1              # Устройство раздела
    partition: auto                # Автоматически определить раздел

# Монтирование файловых систем
mounts:
  # [устройство, точка_монтирования, тип_ФС, опции, dump, pass]
  - ["/dev/sdb1", "/mnt/data", "ext4", "defaults", "0", "2"]
```

#### Настройка сетевых интерфейсов для дополнительных сетей

{% alert level="warning" %}
Настройки, описанные в этом разделе, применяются только для дополнительных сетей. Основная сеть (Main) настраивается автоматически через cloud-init и не требует ручной конфигурации.
{% endalert %}

Если к виртуальной машине подключены дополнительные сети, их необходимо настроить вручную через cloud-init: конфигурационные файлы создаются в `write_files`, применение настроек — в `runcmd`.

Подробнее о подключении дополнительных сетей к виртуальной машине см. в разделе [Дополнительные сетевые интерфейсы](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#дополнительные-сетевые-интерфейсы).

##### Для systemd-networkd

Пример `cloud-config` для дистрибутивов, использующих `systemd-networkd` (Debian, CoreOS и др.):

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

##### Для Netplan (Ubuntu)

Пример `cloud-config` для Ubuntu и других систем, использующих `Netplan`:

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

##### Для ifcfg (RHEL/CentOS)

Пример `cloud-config` для RHEL-совместимых дистрибутивов, использующих схему `ifcfg` и `NetworkManager`:

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

##### Для Alpine Linux

Пример `cloud-config` для дистрибутивов, использующих традиционный формат `/etc/network/interfaces` (Alpine и аналоги):

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

### Как использовать Ansible для конфигурирования виртуальных машин?

[Ansible](https://docs.ansible.com/ansible/latest/index.html) — это инструмент автоматизации, который позволяет выполнять задачи на удаленных серверах с использованием протокола SSH. В данном примере мы рассмотрим, как использовать Ansible для управления виртуальными машинами расположенными в проекте `demo-app`.

В рамках примера предполагается, что:

- в неймспейсе `demo-app` есть ВМ `frontend`;
- в ВМ есть пользователь `cloud` с доступом по SSH;
- на машине, где запускается Ansible, приватный SSH-ключ хранится в файле `/home/user/.ssh/id_rsa`.

1. Создайте файл `inventory.yaml`:

   ```yaml
   ---
   all:
     vars:
       ansible_ssh_common_args: '-o ProxyCommand="d8 v port-forward --stdio=true %h %p"'
       # Пользователь по умолчанию, для доступа по SSH.
       ansible_user: cloud
       # Путь к приватному ключу.
       ansible_ssh_private_key_file: /home/user/.ssh/id_rsa
     hosts:
       # Название узла в формате <название ВМ>.<название проекта>.
       frontend.demo-app:

   ```

1. Проверьте значение `uptime` виртуальной машины:

   ```bash
   ansible -m shell -a "uptime" -i inventory.yaml all

   # frontend.demo-app | CHANGED | rc=0 >>
   # 12:01:20 up 2 days,  4:59,  0 users,  load average: 0.00, 0.00, 0.00
   ```

Если вы не хотите использовать файл inventory, передайте все параметры прямо в командной строке:

```bash
ansible -m shell -a "uptime" \
  -i "frontend.demo-app," \
  -e "ansible_ssh_common_args='-o ProxyCommand=\"d8 v port-forward --stdio=true %h %p\"'" \
  -e "ansible_user=cloud" \
  -e "ansible_ssh_private_key_file=/home/user/.ssh/id_rsa" \
  all
```

### Как автоматически сгенерировать inventory для Ansible?

{% alert level="warning" %}
Для использования команды `d8 v ansible-inventory` требуется версия `d8` v0.27.0 или выше.

Команда работает только для виртуальных машин, у которых подключена основная сеть кластера (Main).
{% endalert %}

Вместо ручного создания inventory-файла можно использовать команду `d8 v ansible-inventory`, которая автоматически генерирует инвентарь Ansible из виртуальных машин в указанном неймспейсе. Команда совместима с интерфейсом [ansible inventory script](https://docs.ansible.com/ansible/latest/user_guide/intro_inventory.html#inventory-scripts).

Команда включает в инвентарь только виртуальные машины с назначенными IP-адресами в состоянии `Running`. Имена хостов формируются в формате `<vmname>.<namespace>` (например, `frontend.demo-app`).

1. При необходимости задайте переменные хоста через аннотации (например, пользователя для SSH):

   ```bash
   d8 k -n demo-app annotate vm frontend provisioning.virtualization.deckhouse.io/ansible_user="cloud"
   ```

1. Запустите Ansible с динамически сформированным инвентарём:

   ```bash
   ANSIBLE_INVENTORY_ENABLED=yaml ansible -m shell -a "uptime" all -i <(d8 v ansible-inventory -n demo-app -o yaml)
   ```

{% alert level="info" %}
Конструкция `<(...)` необходима, потому что Ansible ожидает файл или скрипт в качестве источника списка хостов. Простое указание команды в кавычках не сработает — Ansible попытается выполнить строку как скрипт. Конструкция `<(...)` передаёт вывод команды как файл, который Ansible может прочитать.
{% endalert %}

1. Либо сохраните инвентарь в файл и выполните проверку:

   ```bash
   d8 v ansible-inventory --list -o yaml -n demo-app > inventory.yaml
   ansible -m shell -a "uptime" -i inventory.yaml all
   ```

### Как перенаправить трафик на виртуальную машину?

Виртуальная машина функционирует в кластере Kubernetes, поэтому сетевой трафик направляется к ней по аналогии с направлением трафика к подам. Для маршрутизации сетевого трафика на виртуальную машину применяется стандартный механизм Kubernetes — ресурс Service, который выбирает целевые объекты по лейблам (label selector).

1. Создайте сервис с требуемыми настройками.

   В качестве примера приведена виртуальная машина с меткой `vm: frontend-0`, HTTP-сервисом, опубликованным на портах 80 и 443, и открытым SSH на порту 22:

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

1. Чтобы направить сетевой трафик на порты виртуальной машины, создайте сервис:

   Следующий сервис обеспечивает доступ к виртуальной машине: он слушает порты 80 и 443 и перенаправляет трафик на соответствующие порты целевой виртуальной машины. SSH-доступ извне предоставляется по порту 2211:

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

## Управление платформой

### Как увеличить размер DVCR?

Размер тома DVCR задаётся в ModuleConfig модуля `virtualization` (`spec.settings.dvcr.storage.persistentVolumeClaim.size`). Новое значение должно быть больше текущего.

1. Проверьте текущий размер DVCR:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Пример вывода:

   ```console
    {"size":"58G","storageClass":"linstor-thick-data-r1"}
   ```

1. Увеличьте `size` через `patch` (подставьте нужное значение):

   ```shell
   d8 k patch mc virtualization \
     --type merge -p '{"spec": {"settings": {"dvcr": {"storage": {"persistentVolumeClaim": {"size":"59G"}}}}}}'
   ```

   Пример вывода:

   ```console
   moduleconfig.deckhouse.io/virtualization patched
   ```

1. Убедитесь, что в ModuleConfig отображается новый размер:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Пример вывода:

   ```console
   {"size":"59G","storageClass":"linstor-thick-data-r1"}
   ```

1. Проверьте текущее состояние DVCR:

   ```shell
   d8 k get pvc dvcr -n d8-virtualization
   ```

   Пример вывода:

   ```console
   NAME STATUS VOLUME                                    CAPACITY    ACCESS MODES   STORAGECLASS           AGE
   dvcr Bound  pvc-6a6cedb8-1292-4440-b789-5cc9d15bbc6b  57617188Ki  RWO            linstor-thick-data-r1  7d
   ```

### Как восстановить кластер, если после смены лицензии образы из registry.deckhouse.io не загружаются?

После смены лицензии на кластере с `containerd v1` и удаления устаревшей лицензии образы из `registry.deckhouse.io` могут перестать загружаться. При этом на узлах остаётся устаревший файл конфигурации `/etc/containerd/conf.d/dvcr.toml`, который не удаляется автоматически. Из-за него не запускается модуль `registry`, без которого не работает DVCR.

Манифест NodeGroupConfiguration (NGC) после применения удалит файл на узлах. После запуска модуля `registry` манифест нужно удалить, так как это разовое исправление.

1. Сохраните манифест в файл (например, `containerd-dvcr-remove-old-config.yaml`):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-dvcr-remove-old-config.sh
   spec:
     weight: 32 # Должен быть в диапазоне 32–90
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

1. Примените сохранённый манифест:

   ```bash
   d8 k apply -f containerd-dvcr-remove-old-config.yaml
   ```

1. Проверьте, что модуль `registry` запущен:

   ```bash
   d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
   ```

   Пример вывода при успешном запуске:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Удалите разовый манифест NodeGroupConfiguration:

   ```bash
   d8 k delete -f containerd-dvcr-remove-old-config.yaml
   ```

Подробнее о миграции см. в статье [Миграция container runtime на containerd v2](/products/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/migrating.html).
