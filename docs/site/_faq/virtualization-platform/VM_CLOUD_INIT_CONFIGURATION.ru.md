---
title: Как использовать cloud-init для конфигурирования виртуальных машин?
section: vm_configuration
lang: ru
---

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

### Создание файла с нужными правами

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
