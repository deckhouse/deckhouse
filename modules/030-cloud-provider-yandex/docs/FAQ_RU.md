---
title: "Cloud provider — Yandex Cloud: FAQ"
---

## Настройка INTERNAL LoadBalancer

Для настройки INTERNAL LoadBalancer'а установите аннотацию для сервиса:

```yaml
yandex.cpi.flant.com/listener-subnet-id: SubnetID
```

Аннотация указывает, какой subnet будет слушать LoadBalancer.

## Резервирование публичного IP-адреса

Для использования в `externalIPAddresses` и `natInstanceExternalAddress` (также может быть использован для bastion-хоста) выполните следующую команду:

```shell
$ yc vpc address create --external-ipv4 zone=ru-central1-a
id: e9b4cfmmnc1mhgij75n7
folder_id: b1gog0h9k05lhqe5d88l
created_at: "2020-09-01T09:29:33Z"
external_ipv4_address:
  address: 178.154.226.159
  zone_id: ru-central1-a
  requirements: {}
reserved: true
```

## Проблемы `dhcpOptions` и пути их решения

Использование в настройках DHCP-серверов адресов DNS, отличающихся от предоставляемых Yandex Cloud, является временным решением. От него можно будет отказаться, когда Yandex Cloud введет услугу Managed DNS. Чтобы обойти ограничения, описанные ниже, рекомендуется использовать `stubZones` из модуля [`kube-dns`](../kube-dns/)

### Изменение параметров

Обратите внимание на следующие особенности:

1. При изменении данных параметров требуется выполнить `netplan apply` или аналог, форсирующий обновление DHCP lease.
2. Потребуется перезапуск всех подов hostNetwork (особенно `kube-dns`), чтобы перечитать новый `resolv.conf`.

### Особенности использования

При использовании опции `dhcpOptions` все DNS-запросы начнут идти через указанные DNS-серверы. Эти DNS-серверы **должны** разрешать внешние DNS-имена, а также при необходимости разрешать DNS-имена внутренних ресурсов.

**Не используйте** эту опцию, если указанные рекурсивные DNS-серверы не могут разрешать тот же список зон, что сможет разрешать рекурсивный DNS-сервер в подсети Yandex Cloud.

## Назначение произвольного StorageClass используемого по умолчанию

Чтобы назначить произвольный StorageClass используемым по умолчанию для ваших PVC, укажите его имя в параметре [defaultClusterStorageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass) модуля `global`.
Обратите внимание, что после этого аннотация `storageclass.kubernetes.io/is-default-class='true'` снимется со StorageClass'а, который ранее был указан как используемый по умолчанию.

```shell
d8 k edit mc global
```

## Изменение размера PVC

Размер существующего PersistentVolumeClaim (PVC), созданного с использованием StorageClass Yandex Cloud, можно увеличить без пересоздания PVC.

При выполнении операции последовательно увеличиваются размер диска в Yandex Cloud, размер PersistentVolume (PV) и размер файловой системы на диске.

{% alert level="warning" %}
Для выполнения операции необходимо остановить все поды, использующие PVC. Это приведёт к временному перерыву в работе приложений.

Перед увеличением PVC с критичными данными создайте резервную копию.
{% endalert %}

Не изменяйте размер соответствующего PV или диска непосредственно в консоли Yandex Cloud. Запускайте увеличение только изменением поля `spec.resources.requests.storage` объекта PVC.

Для StorageClass `network-ssd-nonreplicated` и `network-ssd-io-m3` новый размер должен быть кратен `93Gi`.

Чтобы увеличить размер PVC, выполните следующие действия:

1. Задайте переменные с пространством имён, именем PVC и новым размером:

   ```shell
   NAMESPACE=<ПРОСТРАНСТВО_ИМЁН>
   PVC_NAME=<ИМЯ_PVC>
   NEW_SIZE=<НОВЫЙ_РАЗМЕР>
   ```

   Например:

   ```shell
   NAMESPACE=production
   PVC_NAME=application-data
   NEW_SIZE=200Gi
   ```

2. Проверьте текущее состояние PVC:

   ```shell
   d8 k -n "$NAMESPACE" get pvc "$PVC_NAME" \
     -o custom-columns='NAME:.metadata.name,STATUS:.status.phase,STORAGECLASS:.spec.storageClassName,VOLUME:.spec.volumeName,REQUESTED:.spec.resources.requests.storage,CAPACITY:.status.capacity.storage,VOLUME_MODE:.spec.volumeMode'
   ```

   Пример вывода:

   ```console
   NAME               STATUS   STORAGECLASS   VOLUME                                     REQUESTED   CAPACITY   VOLUME_MODE
   application-data   Bound    network-ssd    pvc-a1111111-b222-c333-d444-e55555555555   100Gi       100Gi      Filesystem
   ```

   PVC должен находиться в состоянии `Bound`.

   Поле `REQUESTED` содержит размер, указанный в спецификации PVC, а поле `CAPACITY` — фактически предоставленный размер.

3. Проверьте, что StorageClass разрешает увеличение томов:

   ```shell
   STORAGE_CLASS=$(d8 k -n "$NAMESPACE" get pvc "$PVC_NAME" \
     -o jsonpath='{.spec.storageClassName}')
   ```

   ```shell
   d8 k get storageclass "$STORAGE_CLASS" \
     -o custom-columns='NAME:.metadata.name,PROVISIONER:.provisioner,ALLOW_VOLUME_EXPANSION:.allowVolumeExpansion'
   ```

   Пример вывода:

   ```console
   NAME          PROVISIONER             ALLOW_VOLUME_EXPANSION
   network-ssd   yandex.csi.flant.com    true
   ```

   Для увеличения PVC поле `ALLOW_VOLUME_EXPANSION` должно иметь значение `true`.

4. Получите список подов, использующих PVC:

   ```shell
   d8 k -n "$NAMESPACE" get pods -o json | jq -r \
     --arg pvc "$PVC_NAME" '
       .items[]
       | select(any(.spec.volumes[]?; .persistentVolumeClaim.claimName == $pvc))
       | [
           .metadata.name,
           (.metadata.ownerReferences[0].kind // "-"),
           (.metadata.ownerReferences[0].name // "-"),
           (.spec.nodeName // "-")
         ]
       | @tsv
     '
   ```

   Также список подов отображается в поле `Used By` в описании PVC:

   ```shell
   d8 k -n "$NAMESPACE" describe pvc "$PVC_NAME"
   ```

   Определите рабочие нагрузки, управляющие найденными подами, например Deployment или StatefulSet.

   {% alert level="warning" %}
   Не удаляйте поды по одному. Управляющий контроллер может сразу создать новый под, и том останется подключённым к узлу.

   Если PVC используют несколько рабочих нагрузок, необходимо одновременно остановить всех потребителей.
   {% endalert %}

5. Сохраните текущее количество реплик рабочих нагрузок.

   Для Deployment:

   ```shell
   d8 k -n "$NAMESPACE" get deployment <ИМЯ_DEPLOYMENT> \
     -o custom-columns='NAME:.metadata.name,REPLICAS:.spec.replicas'
   ```

   Для StatefulSet:

   ```shell
   d8 k -n "$NAMESPACE" get statefulset <ИМЯ_STATEFULSET> \
     -o custom-columns='NAME:.metadata.name,REPLICAS:.spec.replicas'
   ```

6. Масштабируйте до нуля все рабочие нагрузки, использующие PVC.

   Для Deployment:

   ```shell
   d8 k -n "$NAMESPACE" scale deployment \
     <ИМЯ_DEPLOYMENT_1> <ИМЯ_DEPLOYMENT_2> \
     --replicas=0
   ```

   Для StatefulSet:

   ```shell
   d8 k -n "$NAMESPACE" scale statefulset \
     <ИМЯ_STATEFULSET_1> <ИМЯ_STATEFULSET_2> \
     --replicas=0
   ```

   Для рабочих нагрузок другого типа остановите создание подов способом, предусмотренным соответствующим контроллером или оператором.

7. Убедитесь, что все поды, использующие PVC, удалены:

   ```shell
   d8 k -n "$NAMESPACE" get pods -o json | jq -r \
     --arg pvc "$PVC_NAME" '
       .items[]
       | select(any(.spec.volumes[]?; .persistentVolumeClaim.claimName == $pvc))
       | .metadata.name
     '
   ```

   Команда не должна возвращать поды.

8. Увеличьте запрошенный размер PVC:

   ```shell
   d8 k -n "$NAMESPACE" patch pvc "$PVC_NAME" \
     --type merge \
     -p "{\"spec\":{\"resources\":{\"requests\":{\"storage\":\"$NEW_SIZE\"}}}}"
   ```

   Также размер можно изменить с помощью редактора:

   ```shell
   d8 k -n "$NAMESPACE" edit pvc "$PVC_NAME"
   ```

   Укажите новое значение поля `spec.resources.requests.storage`:

   ```yaml
   spec:
     resources:
       requests:
         storage: 200Gi
   ```

9. Проверьте события PVC:

   ```shell
   d8 k -n "$NAMESPACE" describe pvc "$PVC_NAME"
   ```

   Во время увеличения могут появляться следующие события:

   ```console
   ExternalExpanding
   Resizing
   FileSystemResizeRequired
   ```

   Событие `FileSystemResizeRequired` означает, что диск увеличен, но для завершения операции необходимо подключить том к узлу и увеличить файловую систему.

   Текущее состояние PVC также можно отслеживать командой:

   ```shell
   d8 k -n "$NAMESPACE" get pvc "$PVC_NAME" --watch
   ```

10. Запустите один под, использующий PVC.

    Например, временно установите одну реплику для одного Deployment:

    ```shell
    d8 k -n "$NAMESPACE" scale deployment <ИМЯ_DEPLOYMENT> \
      --replicas=1
    ```

    Или для StatefulSet:

    ```shell
    d8 k -n "$NAMESPACE" scale statefulset <ИМЯ_STATEFULSET> \
      --replicas=1
    ```

    Дождитесь перехода пода в состояние `Running`:

    ```shell
    d8 k -n "$NAMESPACE" get pods --watch
    ```

    При подключении тома kubelet и CSI-драйвер должны увеличить файловую систему.

11. Убедитесь, что увеличение файловой системы завершено:

    ```shell
    d8 k -n "$NAMESPACE" describe pvc "$PVC_NAME"
    ```

    Об успешном завершении операции свидетельствует событие:

    ```console
    FileSystemResizeSuccessful
    ```

    Например:

    ```console
    MountVolume.NodeExpandVolume succeeded for volume "pvc-a1111111-b222-c333-d444-e55555555555"
    ```

12. Сравните запрошенный и фактически предоставленный размеры PVC:

    ```shell
    d8 k -n "$NAMESPACE" get pvc "$PVC_NAME" \
      -o jsonpath='requested={.spec.resources.requests.storage}{" capacity="}{.status.capacity.storage}{"\n"}'
    ```

    После завершения операции размеры должны совпадать:

    ```console
    requested=200Gi capacity=200Gi
    ```

13. Проверьте размер файловой системы внутри пода:

    ```shell
    d8 k -n "$NAMESPACE" exec <ИМЯ_ПОДА> -- \
      df -hT <ТОЧКА_МОНТИРОВАНИЯ>
    ```

    Например:

    ```shell
    d8 k -n "$NAMESPACE" exec application-6b75db85cf-4rj2k -- \
      df -hT /var/lib/application
    ```

    Пример вывода:

    ```console
    Filesystem     Type  Size  Used Avail Use% Mounted on
    /dev/vdb       ext4  197G   81G  107G  44% /var/lib/application
    ```

    Фактически доступный размер файловой системы может быть немного меньше размера PVC из-за служебных данных файловой системы.

14. После успешного увеличения файловой системы восстановите исходное количество реплик всех рабочих нагрузок:

    ```shell
    d8 k -n "$NAMESPACE" scale deployment <ИМЯ_DEPLOYMENT> \
      --replicas=<ИСХОДНОЕ_КОЛИЧЕСТВО>
    ```

    ```shell
    d8 k -n "$NAMESPACE" scale statefulset <ИМЯ_STATEFULSET> \
      --replicas=<ИСХОДНОЕ_КОЛИЧЕСТВО>
    ```

15. Проверьте состояние подов и доступность приложения:

    ```shell
    d8 k -n "$NAMESPACE" get pods
    ```

### Диагностика

Если увеличение PVC не начинается, проверьте:

* остановлены ли все поды, использующие PVC;
* не создаёт ли контроллер новые поды;
* разрешено ли увеличение томов в StorageClass;
* соответствует ли новый размер ограничениям выбранного типа диска;
* достаточно ли квоты на дисковое пространство в Yandex Cloud.

Проверьте, остался ли том подключённым к узлу:

```shell
PV_NAME=$(d8 k -n "$NAMESPACE" get pvc "$PVC_NAME" \
  -o jsonpath='{.spec.volumeName}')
```

```shell
d8 k get volumeattachments.storage.k8s.io -o json | jq -r \
  --arg pv "$PV_NAME" '
    .items[]
    | select(.spec.source.persistentVolumeName == $pv)
    | [
        .metadata.name,
        .spec.nodeName,
        (.status.attached | tostring)
      ]
    | @tsv
  '
```

Если команда возвращает объект VolumeAttachment со значением `true`, том всё ещё подключён к узлу.

Если размер блочного устройства увеличился, но файловая система сохранила прежний размер, результаты `lsblk` и `df` будут отличаться:

```console
$ lsblk
NAME  SIZE TYPE MOUNTPOINTS
vdb   200G disk /var/lib/application

$ df -h /var/lib/application
Filesystem  Size  Used Avail Use% Mounted on
/dev/vdb    100G   70G   30G  70% /var/lib/application
```

В этом случае убедитесь, что запущен один под, использующий PVC, и проверьте события PVC:

```shell
d8 k -n "$NAMESPACE" describe pvc "$PVC_NAME"
```

При необходимости проверьте журналы CSI-пода на узле, где запущен потребитель PVC:

```shell
d8 k get pods -A -o wide | grep -E 'csi-node|yandex-csi'
```

```shell
d8 k -n <ПРОСТРАНСТВО_ИМЁН_CSI> logs <ИМЯ_CSI_ПОДА> \
  -c <ИМЯ_КОНТЕЙНЕРА> \
  --since=30m
```

{% alert level="warning" %}
Не запускайте `resize2fs`, `xfs_growfs` и другие команды изменения файловой системы вручную, пока не определены тип файловой системы, правильное блочное устройство, наличие разделов и состояние операции CSI.

Использование неподходящей команды или неправильного устройства может привести к повреждению файловой системы.
{% endalert %}

Если PVC использует режим `Block`, CSI-драйвер увеличивает только блочное устройство. Приложение должно самостоятельно обнаружить и обработать изменение его размера.

## Добавление CloudStatic-узлов в кластер

В метаданные виртуальных машин, которые вы хотите включить в кластер в качестве узлов, добавьте (Изменить ВМ -> Метадата) ключ `node-network-cidr` со значением `nodeNetworkCIDR` для кластера.

`nodeNetworkCIDR` кластера можно узнать, воспользовавшись следующей командой:

```shell
d8 k -n kube-system get secret d8-provider-cluster-configuration -o json | jq --raw-output '.data."cloud-provider-cluster-configuration.yaml"' | base64 -d | grep '^nodeNetworkCIDR'
```

## Создание кластера в новом VPC и развертывание bastion-хост для доступа к узлам

1. Выполните bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config.yml
   ```

1. Создайте bastion-хост:

   ```shell
   yc compute instance create \
   --name bastion \
   --hostname bastion \
   --create-boot-disk image-family=ubuntu-2404-lts,image-folder-id=standard-images,size=20,type=network-hdd \
   --memory 2 \
   --cores 2 \
   --core-fraction 100 \
   --ssh-key ~/.ssh/<SSH_PUBLIC_KEY_FILE> \
   --zone ru-central1-a \
   --public-address 178.154.226.159
   ```

   > Замените здесь `<SSH_PUBLIC_KEY_FILE>` на имя вашего публичного ключа. Например, для ключа с RSA-шифрованием это будет `id_rsa.pub`, а для ключа с ED25519-шифрованием `id_ed25519.pub`.

1. Продолжите установку кластера, указав данные bastion-хоста. На вопрос про кеш Terraform ответьте `y`:

   ```shell
   dhctl bootstrap --ssh-bastion-host=178.154.226.159 --ssh-bastion-user=yc-user \
   --ssh-user=ubuntu --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> --config=/config.yml
   ```

   > Замените здесь `<SSH_PRIVATE_KEY_FILE>` на имя вашего приватного ключа. Например, для ключа с RSA-шифрованием это может быть `id_rsa`, а для ключа с ED25519-шифрованием — `id_ed25519`.

## Длительное переключение на заказ узлов в менее приоритетных группах

Если переключение на заказ узлов в менее приоритетных группах занимает много времени (например, когда для групп узлов с preemptible-инстансами установлен наивысший приоритет и при недоступности таких инстансов заказ узлов из других групп происходит очень долго), воспользуйтесь [инструкцией](/products/kubernetes-platform/documentation/v1/faq.html#что-делать-если-переключение-на-заказ-узлов-в-менее-приоритетных).
