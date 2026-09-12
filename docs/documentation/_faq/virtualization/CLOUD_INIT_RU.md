---
title: Как использовать cloud-init для конфигурирования виртуальных машин?
subsystems:
- virtualization
lang: ru
---

[Cloud-init](https://cloudinit.readthedocs.io/) применяется для первичной настройки гостевой ОС при первом запуске. Конфигурация задаётся в YAML и начинается с директивы `#cloud-config`.

{% alert level="warning" %}
Для образов, рассчитанных на cloud-init (в том числе официальных cloud-образов дистрибутивов), конфигурацию cloud-init нужно передать явно. Иначе на части дистрибутивов не поднимается сеть, и машина остаётся недоступной даже при подключённой основной сети (Main).

Кроме того, в cloud-образах по умолчанию отключён вход в систему. Добавьте SSH-ключи пользователю по умолчанию либо создайте нового пользователя с SSH-доступом, иначе к машине не подключиться.
{% endalert %}

#### Обновление и установка пакетов

Пример `cloud-config` для обновления системы и установки пакетов из списка:

```yaml
#cloud-config
# Обновить списки пакетов.
package_update: true
# Обновить установленные пакеты до последних версий.
package_upgrade: true
# Список пакетов для установки.
packages:
  - nginx
  - curl
  - htop
# Команды для выполнения после установки пакетов.
runcmd:
  - systemctl enable --now nginx.service
```

#### Создание пользователя

Пример `cloud-config` для создания локального пользователя с паролем и SSH-ключом:

```yaml
#cloud-config
# Список пользователей для создания.
users:
    # Имя пользователя.
  - name: cloud
    # Хеш пароля.
    passwd: "<PASSWORD_HASH>"
    # Не блокировать учётную запись.
    lock_passwd: false
    # Права sudo без запроса пароля.
    sudo: ALL=(ALL) NOPASSWD:ALL
    # Оболочка по умолчанию.
    shell: /bin/bash
    # SSH-ключи для доступа.
    ssh-authorized-keys:
      - <SSH_PUBLIC_KEY>
# Разрешить аутентификацию по паролю через SSH.
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
# Список файлов для создания.
write_files:
    # Путь к файлу.
  - path: /opt/scripts/start.sh
    # Содержимое файла.
    content: |
      #!/bin/bash
      echo "Starting application"
    # Владелец файла, пользователь и группа.
    owner: cloud:cloud
    # Права доступа в восьмеричном формате.
    permissions: '0755'
```

#### Настройка диска и файловой системы

Пример `cloud-config` для разметки диска, создания файловой системы и монтирования:

```yaml
#cloud-config
# Настройка разметки диска.
disk_setup:
  # Устройство диска.
  /dev/sdb:
    # Тип таблицы разделов, gpt или mbr.
    table_type: gpt
    # Автоматически создать разделы.
    layout: true
    # Не перезаписывать существующие разделы.
    overwrite: false

# Настройка файловых систем.
fs_setup:
    # Метка файловой системы.
  - label: data
    # Тип файловой системы.
    filesystem: ext4
    # Устройство раздела.
    device: /dev/sdb1
    # Автоматически определить раздел.
    partition: auto

# Монтирование файловых систем.
mounts:
  # [устройство, точка_монтирования, тип_ФС, опции, dump, pass]
  - ["/dev/sdb1", "/mnt/data", "ext4", "defaults", "0", "2"]
```

#### Настройка сетевых интерфейсов для дополнительных сетей

{% alert level="warning" %}
Настройки, описанные в этом разделе, применяются только для дополнительных сетей. Основная сеть (Main) настраивается автоматически через cloud-init и не требует ручной конфигурации.
{% endalert %}

Дополнительные сети настраиваются вручную через cloud-init. Конфигурационные файлы создаёт блок `write_files`, а применяет настройки блок `runcmd`.

Подробнее о подключении дополнительных сетей к виртуальной машине см. в разделе [Дополнительные сетевые интерфейсы](user/network/virtualization/vm-additional-interfaces.html#дополнительные-сетевые-интерфейсы).

Ниже приведены примеры для распространённых способов настройки сети в гостевой ОС:

{% tabs cloudinit-net %}

{% tab "systemd-networkd" %}

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

{% endtab %}

{% tab "Netplan (Ubuntu)" %}

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

{% endtab %}

{% tab "ifcfg (RHEL/CentOS)" %}

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

{% endtab %}

{% tab "Alpine Linux" %}

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

{% endtab %}

{% endtabs %}
