{%- include getting_started/dvp/global/partials/gs_scripts.liquid step='prepare' -%}

Подготовьте узлы кластера: настройте NFS-сервер и worker-узел до установки платформы.

## Настройка NFS-сервера

Для настройки NFS выполните следующие действия:

1. Настройте NFS-сервер для хранения дисков ВМ. Выполните следующие команды на **NFS-сервере**:

   {% tabs dvp-nfs-server %}
   {% tab "ОС на базе Ubuntu" %}
   ```bash
   sudo apt update
   sudo apt install nfs-kernel-server
   sudo mkdir -p <NFS_SHARE>
   sudo chown -R nobody:nogroup <NFS_SHARE>
   echo "<NFS_SHARE> <INTERNAL_NETWORK_CIDRS>(rw,sync,no_subtree_check,no_root_squash)" | sudo tee -a /etc/exports
   sudo exportfs -ra
   sudo systemctl restart nfs-kernel-server
   ```
   {% endtab %}
   {% tab "Для CentOS, Rocky Linux, ALT Linux, РОСА Сервер, РЕД ОС, МОС ОС" %}
   ```bash
   sudo dnf install -y nfs-utils
   sudo mkdir -p <NFS_SHARE>
   sudo chown -R nobody:nobody <NFS_SHARE>
   echo "<NFS_SHARE> <INTERNAL_NETWORK_CIDRS>(rw,sync,no_subtree_check,no_root_squash)" | sudo tee -a /etc/exports
   sudo exportfs -ra
   sudo systemctl enable --now nfs-server
   sudo systemctl restart nfs-server
   ```
   {% endtab %}
   {% endtabs %}

1. Проверьте доступ к NFS с **master-узла**. Выполните следующие команды на **master-узле**:

   {% tabs dvp-nfs-master %}
   {% tab "ОС на базе Ubuntu" %}
   ```bash
   sudo apt update
   sudo apt install nfs-common
   sudo mkdir -p /mnt/dvp-nfs-test
   sudo mount -t nfs4 <NFS_HOST>:<NFS_SHARE> /mnt/dvp-nfs-test
   ls /mnt/dvp-nfs-test
   sudo umount /mnt/dvp-nfs-test
   sudo rmdir /mnt/dvp-nfs-test
   ```
   {% endtab %}
   {% tab "Для CentOS, Rocky Linux, ALT Linux, РОСА Сервер, РЕД ОС, МОС ОС" %}
   ```bash
   sudo dnf install -y nfs-utils
   sudo mkdir -p /mnt/dvp-nfs-test
   sudo mount -t nfs4 <NFS_HOST>:<NFS_SHARE> /mnt/dvp-nfs-test
   ls /mnt/dvp-nfs-test
   sudo umount /mnt/dvp-nfs-test
   sudo rmdir /mnt/dvp-nfs-test
   ```
   {% endtab %}
   {% endtabs %}

   Если монтирование не удаётся, проверьте, что NFS-сервер доступен с master-узла, его IP не совпадает с IP master- и worker-узлов, а запись в `/etc/exports` разрешает доступ из подсети узлов кластера.

## Подготовка worker-узла

{% alert level="info" %}
В браузере автоматически сгенерировалась пара SSH-ключей для пользователя `caps`, необходимых для дальнейшей настройки кластера. Сохраните их: они могут понадобиться, если вы захотите добавить дополнительные worker-узлы.

Публичный ключ:

```text
<CAPS_SSH_PUBLIC_KEY>
```

Приватный ключ:

```text
<CAPS_SSH_PRIVATE_KEY>
```
{% endalert %}

{% alert level="warning" %}
При развёртывании тестового окружения на виртуальных машинах (ВМ) включите nested virtualization на гипервизоре для **worker-узла**. См. [требования к установке](/products/virtualization-platform/gs/bm/step1.html#hardware-and-software-requirements).
{% endalert %}

Для дальнейшей настройки создайте пользователя `caps`, выполнив следующие команды на **worker-узле** :

{% tabs dvp-caps-worker %}
{% tab "ОС на базе Ubuntu" %}
```bash
export KEY='<CAPS_SSH_PUBLIC_KEY>'
sudo useradd -m -s /bin/bash caps
sudo usermod -aG sudo caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
sudo mkdir -p /home/caps/.ssh
echo "$KEY" | sudo tee -a /home/caps/.ssh/authorized_keys
sudo chown -R caps:caps /home/caps
sudo chmod 700 /home/caps/.ssh
sudo chmod 600 /home/caps/.ssh/authorized_keys
```
{% endtab %}
{% tab "Для CentOS, Rocky Linux, ALT Linux, РОСА Сервер, РЕД ОС, МОС ОС" %}
```bash
export KEY='<CAPS_SSH_PUBLIC_KEY>'
sudo useradd -m -s /bin/bash caps
sudo usermod -aG wheel caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
sudo mkdir -p /home/caps/.ssh
echo "$KEY" | sudo tee -a /home/caps/.ssh/authorized_keys
sudo chown -R caps:caps /home/caps
sudo chmod 700 /home/caps/.ssh
sudo chmod 600 /home/caps/.ssh/authorized_keys
```
{% endtab %}
{% endtabs %}
**В Astra Linux** с Parsec задайте максимальный уровень целостности для `caps`:
```bash
sudo pdpl-user -i 63 caps
```

Теперь узлы кластера готовы к установке Deckhouse Virtualization Platform.
