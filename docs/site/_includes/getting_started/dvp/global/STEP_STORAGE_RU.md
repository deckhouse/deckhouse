{% alert level="warning" %}
На этом этапе приведен пример настройки хранилища на основе внешнего NFS-сервера с установленным дистрибутивом на основе Debian/Ubuntu.
Если вы хотите использовать хранилище другого типа, ознакомьтесь с разделом [«Настройка хранилища»](../../documentation/admin/install/steps/storage.html).
{% endalert %}

Настройте хранилище, которое будет использоваться для хранения метрик компонентов кластера и дисков виртуальных машин.

## Настройка NFS-сервера

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

1. Экспортируйте каталог с правами, позволяющими доступ root-клиентам. Для Linux-сервера это делается через опцию `no_root_squash`. Добавьте следующую строку в файл `/etc/exports`:

   ```bash
   echo "/srv/nfs/dvp <SubnetCIDR>(rw,sync,no_subtree_check,no_root_squash)" | sudo tee -a /etc/exports
   ```

   Замените `<SubnetCIDR>` на подсеть, в которой находятся master- и worker-узлы (например, `192.168.1.0/24`). 
   
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

## Настройка модуля csi-nfs

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

1. Проверьте, что поды драйвера запущены в пространстве имён `d8-csi-nfs`:

   ```bash
   sudo -i d8 k -n d8-csi-nfs get pod -owide -w
   ```

1. Создайте ресурс NFSStorageClass, который описывает подключение к вашему NFS-серверу:

   ```bash
   sudo -i d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NFSStorageClass
   metadata:
     name: nfs-storage-class
   spec:
     connection:
       host: <IP-адрес-NFS-сервера>
       share: /srv/nfs/dvp
       nfsVersion: "4.1"
     reclaimPolicy: Delete
     volumeBindingMode: WaitForFirstConsumer
   EOF
   ```

   Параметры, которые нужно заменить:

   - `<IP-адрес-NFS-сервера>` — IP-адрес NFS-сервера, доступный из кластера;
   - `share` — экспортируемый каталог на NFS-сервере (в примере `/srv/nfs/dvp`).

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
