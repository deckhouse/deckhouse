---
title: "FAQ: Конфигурирование ВМ"
permalink: ru/virtualization-platform/documentation/faq/vm-configuration.html
lang: ru
---

## Как использовать cloud-init для конфигурирования виртуальных машин?

[Cloud-init](https://cloudinit.readthedocs.io/) применяется для первичной настройки гостевой ОС при первом запуске. Конфигурация задаётся в YAML и начинается с директивы `#cloud-config`.

{% alert level="warning" %}
Для образов, рассчитанных на cloud-init (в том числе официальных cloud-образов дистрибутивов), конфигурацию cloud-init нужно передать явно: иначе на части дистрибутивов не поднимается сеть, и ВМ оказывается недоступна по сети даже при подключении основной сети (Main).

Кроме того, в cloud-образах по умолчанию отключена возможность входа в систему — необходимо добавить SSH-ключи для пользователя по умолчанию либо создать нового пользователя с SSH-доступом, иначе доступ к виртуальной машине будет невозможен.
{% endalert %}

### Обновление и установка пакетов

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

### Создание пользователя

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

### Настройка диска и файловой системы

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

### Настройка сетевых интерфейсов для дополнительных сетей

{% alert level="warning" %}
Настройки, описанные в этом разделе, применяются только для дополнительных сетей. Основная сеть (Main) настраивается автоматически через cloud-init и не требует ручной конфигурации.
{% endalert %}

Если к виртуальной машине подключены дополнительные сети, их необходимо настроить вручную через cloud-init: конфигурационные файлы создаются в `write_files`, применение настроек — в `runcmd`.

Подробнее о подключении дополнительных сетей к виртуальной машине см. в разделе [Дополнительные сетевые интерфейсы](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#дополнительные-сетевые-интерфейсы).

#### Для systemd-networkd

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

#### Для Netplan (Ubuntu)

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

#### Для ifcfg (RHEL/CentOS)

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

#### Для Alpine Linux

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

## Как использовать Ansible для конфигурирования виртуальных машин?

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

## Как автоматически сгенерировать inventory для Ansible?

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

## Как перенаправить трафик на виртуальную машину?

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
