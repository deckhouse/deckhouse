<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

На данном этапе вы создали кластер, который состоит из **единственного** узла — master-узла. На master-узле по умолчанию запускаются только системные компоненты. Для полноценной работы платформы виртуализации необходимо добавить в кластер хотя бы один worker-узел, настроить хранилище для дисков виртуальных машин и включить модуль виртуализации.

## Добавление узлов к кластеру

Добавьте узел в кластер (подробнее о добавлении статического узла в кластер читайте [в документации](/products/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/bare-metal-node.html#добавление-узлов-в-bare-metal-кластере)):

- Подготовьте сервер, который будет worker-узлом кластера.

- Создайте [NodeGroup](/modules/node-manager/cr.html#nodegroup) с именем `worker`, выполнив на **master-узле** следующую команду:

  ```shell
  sudo -i d8 k create -f - << EOF
  apiVersion: deckhouse.io/v1
  kind: NodeGroup
  metadata:
    name: worker
  spec:
    nodeType: Static
    staticInstances:
      count: 1
      labelSelector:
        matchLabels:
          role: worker
  EOF
  ```
  
- Сгенерируйте SSH-ключ с пустой парольной фразой. Для этого выполните на **master-узле** следующую команду:

  ```shell
  ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
  ```

- Создайте в кластере ресурс [SSHCredentials](/modules/node-manager/cr.html#sshcredentials). Для этого выполните на **master-узле** следующую команду:

  ```shell
  sudo -i d8 k create -f - <<EOF
  apiVersion: deckhouse.io/v1alpha1
  kind: SSHCredentials
  metadata:
    name: caps
  spec:
    user: caps
    privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
  EOF
  ```

- Выведите публичную часть сгенерированного ранее SSH-ключа (он понадобится на следующем шаге). Для этого выполните на **master-узле** следующую команду:

  ```shell
  cat /dev/shm/caps-id.pub
  ```

- **На подготовленном worker-узле** создайте пользователя `caps`. Для этого выполните следующую команду, указав публичную часть SSH-ключа, полученную на предыдущем шаге (если у текущего пользователя недостаточно прав, выполняйте команды с `sudo`):

  ```shell
  export KEY='<SSH-PUBLIC-KEY>' # Укажите публичную часть SSH-ключа пользователя.
  useradd -m -s /bin/bash caps
  usermod -aG sudo caps
  echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
  mkdir /home/caps/.ssh
  echo $KEY | tee -a /home/caps/.ssh/authorized_keys
  chown -R caps:caps /home/caps
  chmod 700 /home/caps/.ssh
  chmod 600 /home/caps/.ssh/authorized_keys
  ```

- **В операционных системах семейства Astra Linux**, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя `caps`:

  ```shell
  pdpl-user -i 63 caps
  ```

- Создайте ресурс [StaticInstance](/modules/node-manager/cr.html#staticinstance) для добавляемого узла. Для этого выполните на **master-узле** следующую команду, указав IP-адрес добавляемого узла:

  ```shell
  export NODE=<NODE-IP-ADDRESS> # Укажите IP-адрес узла, который необходимо подключить к кластеру.
  sudo -i d8 k create -f - <<EOF
  apiVersion: deckhouse.io/v1alpha1
  kind: StaticInstance
  metadata:
    name: dvp-worker
    labels:
      role: worker
  spec:
    address: "$NODE"
    credentialsRef:
      kind: SSHCredentials
      name: caps
  EOF
  ```

- Дождитесь, пока все узлы кластера перейдут в состояние `Ready`.
  Выполните на **master-узле** следующую команду, чтобы получить список узлов кластера:

  ```shell
  sudo -i d8 k get no
  ```

## Настройка NFS-сервера

{% alert level="warning" %}
На этом этапе приведен пример настройки хранилища на основе внешнего NFS-сервера с установленным дистрибутивом на основе Debian/Ubuntu.
Если вы хотите использовать хранилище другого типа, ознакомьтесь с разделом [«Настройка хранилища»](../../documentation/admin/install/steps/storage.html).
{% endalert %}

Настройте хранилище, которое будет использоваться для хранения метрик компонентов кластера и дисков виртуальных машин.

1. Установите пакеты NFS-сервера (если они еще не установлены):

   ```bash
   sudo apt update
   sudo apt install nfs-kernel-server
   ```

1. Создайте каталог, который будет использоваться для хранения данных:

   ```bash
   sudo mkdir -p /srv/nfs/dvp
   ```

1. Установите права доступа:

   ```bash
   sudo chown -R nobody:nogroup /srv/nfs/dvp
   ```

1. Экспортируйте каталог с разрешением доступа для root-пользователей на клиентах. Для Linux-сервера это настраивается с помощью опции `no_root_squash`. Выполните следующую команду, чтобы добавить конфигурацию в файл `/etc/exports`:

   ```bash
   echo "/srv/nfs/dvp <SubnetCIDR>(rw,sync,no_subtree_check,no_root_squash)" | sudo tee -a /etc/exports
   ```

   Замените `<SubnetCIDR>` на подсеть, в которой находятся master- и worker-узлы (например, `10.128.0.0/24`).

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
   sudo mount -t nfs4 <IP-адрес-NFS-сервера>:/srv/nfs/dvp /mnt
   sudo umount /mnt
   ```

   Замените `<IP-адрес-NFS-сервера>` на IP-адрес вашего NFS-сервера.

### Настройка модуля csi-nfs

Для работы с NFS-хранилищем в кластере настройте модуль `csi-nfs` на master-узле. Модуль предоставляет CSI-драйвер и позволяет создавать StorageClass через ресурс [NFSStorageClass](/modules/csi-nfs/stable/cr.html#nfsstorageclass).

Для того чтобы настроить модуль, выполните следующие действия:

1. Включите модуль `csi-nfs`, выполнив на master-узле следующую команду:

   ```bash
   sudo -i d8 system module enable csi-nfs
   ```

1. Дождитесь, пока модуль перейдет в состояние `Ready`:

   ```bash
   sudo -i d8 k get module csi-nfs -w
   ```

1. Создайте ресурс [NFSStorageClass](/modules/csi-nfs/cr.html#nfsstorageclass), который описывает подключение к вашему NFS-серверу:

   ```bash
   sudo -i d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NFSStorageClass
   metadata:
     name: nfs-storage-class
   spec:
     connection:
       host: <IP-адрес-NFS-сервера>
       share: <экспортируемый-каталог> # Например, /srv/nfs/dvp
       nfsVersion: "4.1"
     reclaimPolicy: Delete
     volumeBindingMode: WaitForFirstConsumer
   EOF
   ```

   Параметры, которые нужно заменить:

  - `<IP-адрес-NFS-сервера>` — IP-адрес NFS-сервера, доступный из кластера;
  - `share` — экспортируемый каталог на NFS-сервере.

1. Проверьте, что NFSStorageClass создан успешно:

   ```bash
   sudo -i d8 k get nfsstorageclass
   ```

1. Установите созданный StorageClass как используемый по умолчанию для кластера:

   ```bash
   DEFAULT_STORAGE_CLASS=nfs-storage-class
   sudo -i d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
   ```

1. Проверьте, что StorageClass установлен как используемый по умолчанию:

   ```bash
   sudo -i d8 k get storageclass
   ```

   В колонке `DEFAULT` у `nfs-storage-class` должна быть отметка.

После этого все новые PVC, для которых не указан `storageClassName`, будут автоматически создаваться на NFS-хранилище.

## Настройка модуля виртуализации

Включите модуль `virtualization`. В параметре [.spec.settings.virtualMachineCIDRs](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) модуля укажите подсети, IP-адреса из которых будут назначаться виртуальным машинам:

```shell
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
    # Укажите подсети, из которых будут назначаться IP-адреса виртуальным машинам.
    - 10.66.10.0/24
    - 10.66.20.0/24
    - 10.66.30.0/24
  version: 1
EOF
```

{% alert level="warning" %}
Подсети блока `.spec.settings.virtualMachineCIDRs` не должны пересекаться с подсетями узлов кластера, подсетью сервисов или подсетью подов (`podCIDR`).
{% endalert %}
