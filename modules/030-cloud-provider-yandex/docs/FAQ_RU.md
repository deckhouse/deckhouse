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

Размер существующего PVC можно увеличить путем изменения значения параметра `spec.resources.requests.storage` без остановки и пересоздания использующего его пода.

После изменения значения `spec.resources.requests.storage` CSI-драйвер последовательно:

- увеличивает размер диска в Yandex Cloud;
- обновляет размер связанного PersistentVolume;
- выполняет расширение файловой системы на узле, к которому подключён том.

Во время операции под продолжает работать, а смонтированный том остаётся доступным приложению. После завершения увеличения новый размер файловой системы становится доступен внутри контейнера без перезапуска пода.

{% alert level="info" %}
Уменьшение размера PVC не поддерживается.
{% endalert %}

Чтобы увеличить PVC, выполните следующие действия:

1. Получите имя StorageClass, используемого PVC:

   ```shell
   d8 k -n <НЕЙМСПЕЙС> get pvc <ИМЯ_PVC> \
     -o jsonpath='{.spec.storageClassName}{"\n"}'
   ```

   Где:

   - `<НЕЙМСПЕЙС>` — неймспейс, в котором находится PVC;
   - `<ИМЯ_PVC>` — имя PVC, размер которого необходимо увеличить.

   Например:

   ```shell
   d8 k -n production get pvc application-data \
     -o jsonpath='{.spec.storageClassName}{"\n"}'
   ```

   Пример вывода команды:

   ```console
   network-ssd
   ```

   В этом примере PVC `application-data` использует StorageClass `network-ssd`.

1. Убедитесь, что StorageClass разрешает увеличение томов:

   ```shell
   d8 k get storageclass <ИМЯ_STORAGECLASS> \
     -o jsonpath='{.allowVolumeExpansion}{"\n"}'
   ```

   Где `<ИМЯ_STORAGECLASS>` — имя StorageClass, полученное на предыдущем шаге.

   Например:

   ```shell
   d8 k get storageclass network-ssd \
     -o jsonpath='{.allowVolumeExpansion}{"\n"}'
   ```

   Пример вывода команды:

   ```console
   true
   ```

1. Проверьте текущее состояние и размер PVC:

   ```shell
   d8 k -n <НЕЙМСПЕЙС> get pvc <ИМЯ_PVC>
   ```

   Например:

   ```shell
   d8 k -n production get pvc application-data
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME               STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS
   application-data   Bound    pvc-65e92674-077c-4b4f-b65d-19e92f04e103   20Gi       RWO            network-ssd
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Убедитесь, что:

   - PVC находится в состоянии `Bound`;
   - в поле `CAPACITY` указан текущий размер PVC;
   - в поле `STORAGECLASS` указан StorageClass, проверенный на предыдущем шаге.

1. Увеличьте размер PVC:

   ```shell
   d8 k -n <НЕЙМСПЕЙС> edit pvc <ИМЯ_PVC>
   ```

   Например:

   ```shell
   d8 k -n production edit pvc application-data
   ```

   В поле `spec.resources.requests.storage` укажите новый размер PVC:

   ```yaml
   spec:
     resources:
       requests:
         storage: 30Gi
   ```

   В этом примере размер PVC увеличивается до 30Gi.

   Сохраните изменения и закройте редактор.

   {% alert level="warning" %}
   Для StorageClass `network-ssd-nonreplicated` и `network-ssd-io-m3` [размер должен быть кратен 93Gi](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-disktype).
   {% endalert %}

1. Дождитесь увеличения PVC:

   ```shell
   d8 k -n <НЕЙМСПЕЙС> get pvc <ИМЯ_PVC> --watch
   ```

   Где:

   - `<НЕЙМСПЕЙС>` — неймспейс, в котором находится PVC;
   - `<ИМЯ_PVC>` — имя PVC, размер которого увеличивается.

   Например:

   ```shell
   d8 k -n production get pvc application-data --watch
   ```

   Во время увеличения в поле `CAPACITY` может отображаться прежний размер:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME               STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS
   application-data   Bound    pvc-65e92674-077c-4b4f-b65d-19e92f04e103   20Gi       RWO            network-ssd
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Операция завершена, когда в поле `CAPACITY` отображается новый размер PVC:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME               STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS
   application-data   Bound    pvc-65e92674-077c-4b4f-b65d-19e92f04e103   30Gi       RWO            network-ssd
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Проверьте события PVC:

   ```shell
   d8 k -n <НЕЙМСПЕЙС> describe pvc <ИМЯ_PVC>
   ```

   Например:

   ```shell
   d8 k -n production describe pvc application-data
   ```

   Во время увеличения могут появиться следующие события:

   ```console
   ExternalExpanding
   Resizing
   FileSystemResizeRequired
   ```

   Об успешном увеличении файловой системы свидетельствует событие:

   ```console
   FileSystemResizeSuccessful
   ```

   Например:

   ```console
   Normal  FileSystemResizeSuccessful  kubelet  MountVolume.NodeExpandVolume succeeded for volume "pvc-65e92674-077c-4b4f-b65d-19e92f04e103"
   ```

1. Проверьте размер файловой системы внутри пода:

   ```shell
   d8 k -n <НЕЙМСПЕЙС> exec <ИМЯ_ПОДА> -- \
     df -hT <ТОЧКА_МОНТИРОВАНИЯ>
   ```

   Где:

   - `<НЕЙМСПЕЙС>` — неймспейс, в котором находится под;
   - `<ИМЯ_ПОДА>` — имя пода, использующего PVC;
   - `<ТОЧКА_МОНТИРОВАНИЯ>` — путь внутри контейнера, в который смонтирован PVC.

   Например:

   ```shell
   d8 k -n production exec application-0 -- \
     df -hT /data
   ```

   Пример вывода:

   ```console
   Filesystem   Type   Size    Used   Avail   Use%   Mounted on
   /dev/vde     ext4   29.4G   22M    29.4G    1%    /data
   ```

   {% alert level="info" %}
   Фактический размер файловой системы может быть немного меньше размера PVC из-за служебных данных файловой системы.
   {% endalert %}

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
