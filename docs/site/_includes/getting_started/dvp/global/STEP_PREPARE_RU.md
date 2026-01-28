<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/dvp/getting-started-access.js]{% endjavascript_asset_tag %}'></script>

## Установка ОС на master и worker узлы

Подготовьте узлы и установите на них все необходимые пакеты.

## Генерация SSH ключей для доступа к master и worker узлам

### На master узле

1. Сгенерируйте SSH-ключ с пустой парольной фразой. Для этого выполните на master-узле следующую команду:

   ```bash
   ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
   ```

1. Выведите публичную часть сгенерированного ранее SSH-ключа (он понадобится на следующем шаге). Для этого выполните на master-узле следующую команду:

   ```bash
   cat /dev/shm/caps-id.pub
   ```

### На worker узле

На подготовленном worker-узле создайте пользователя `caps`. Для этого выполните следующую команду, указав публичную часть SSH-ключа, полученную на предыдущем шаге (если у текущего пользователя недостаточно прав, выполняйте команды с `sudo`):

```bash
export KEY='<SSH_KEY>' # Укажите публичную часть SSH-ключа пользователя.
useradd -m -s /bin/bash caps
usermod -aG sudo caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $KEY | tee -a /home/caps/.ssh/authorized_keys
chown -R caps:caps /home/caps
chmod 700 /home/caps/.ssh
chmod 600 /home/caps/.ssh/authorized_keys
```

{% alert level="info" %}
В операционных системах семейства Astra Linux, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя `caps`:

```bash
pdpl-user -i 63 caps
```
{% endalert %}

## Настройка NFS-сервера

{% alert level="warning" %}
На этом этапе приведен пример настройки хранилища на основе внешнего NFS-сервера с установленным дистрибутивом на основе Debian/Ubuntu. Если вы хотите использовать хранилище другого типа, ознакомьтесь с разделом [«Настройка хранилища»](../../documentation/admin/install/steps/storage.html).
{% endalert %}

Настройте хранилище, которое будет использоваться для хранения метрик компонентов кластера и дисков виртуальных машин.

1. Установите пакеты NFS-сервера (если они еще не установлены):

   ```bash
   sudo apt update && sudo apt install nfs-kernel-server
   ```

1. Создайте каталог, который будет использоваться для хранения данных:

   ```bash
   sudo mkdir -p <NFS_SHARE>
   ```

1. Установите права доступа:

   ```bash
   sudo chown -R nobody:nogroup <NFS_SHARE>
   ```

1. Экспортируйте каталог с разрешением доступа для root-пользователей на клиентах. Для Linux-сервера это настраивается с помощью опции `no_root_squash`. Выполните следующую команду, чтобы добавить конфигурацию в файл `/etc/exports`:

   ```bash
   echo "<NFS_SHARE> <SUBNET_CIDR>(rw,sync,no_subtree_check,no_root_squash)" | sudo tee -a /etc/exports
   ```
   
1. Примените изменения конфигурации:

   ```bash
   sudo exportfs -ra
   ```

1. Перезапустите службу NFS:

   ```bash
   sudo systemctl restart nfs-kernel-server
   ```

1. Выполните следующие команды на master- и worker-узлах, чтобы убедиться в успешном монтировании каталога:

   ```bash
   sudo apt update && sudo apt install -y nfs-common
   sudo mount -t nfs4 <NFS_HOST>:<NFS_SHARE> /mnt
   sudo umount /mnt
   ```
