---
title: "Cloud provider — Yandex Cloud: FAQ"
---

## Как настроить INTERNAL LoadBalancer?

Для настройки INTERNAL LoadBalancer'а установите одну из следующих аннотаций для сервиса:

1.
   ```yaml
   yandex.cpi.flant.com/listener-subnet-id: SubnetID
   ```

   Аннотация указывает, какой subnet будет слушать LoadBalancer.
   
2.
   ```yaml
   yandex.cpi.flant.com/loadbalancer-internal: ""
   ```

   LoadBalancer будет слушать первую из доступных subnet.

Аннотация указывает, какой subnet будет слушать LoadBalancer.

## Как зарезервировать публичный IP-адрес?

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

Использование в настройках DHCP-серверов адресов DNS, отличающихся от предоставляемых Yandex Cloud, является временным решением. От него можно будет отказаться, когда Yandex Cloud введет услугу Managed DNS. Чтобы обойти ограничения, описанные ниже, рекомендуется использовать `stubZones` из модуля [`kube-dns`](../042-kube-dns/)

### Изменение параметров

Обратите внимание на следующие особенности:

1. При изменении данных параметров требуется выполнить `netplan apply` или аналог, форсирующий обновление DHCP lease.
2. Потребуется перезапуск всех подов hostNetwork (особенно `kube-dns`), чтобы перечитать новый `resolv.conf`.

### Особенности использования

При использовании опции `dhcpOptions` все DNS-запросы начнут идти через указанные DNS-серверы. Эти DNS-серверы **должны** разрешать внешние DNS-имена, а также при необходимости разрешать DNS-имена внутренних ресурсов.

**Не используйте** эту опцию, если указанные рекурсивные DNS-серверы не могут разрешать тот же список зон, что сможет разрешать рекурсивный DNS-сервер в подсети Yandex Cloud.

## Как назначить произвольный StorageClass используемым по умолчанию?

Чтобы назначить произвольный StorageClass используемым по умолчанию, выполните следующие шаги:

1. Добавьте на StorageClass аннотацию `storageclass.kubernetes.io/is-default-class='true'`:

   ```shell
   kubectl annotate sc $STORAGECLASS storageclass.kubernetes.io/is-default-class='true'
   ```

2. Укажите имя StorageClass'а в параметре [storageClass.default](configuration.html#parameters-storageclass-default) в настройках модуля `cloud-provider-yandex`. Обратите внимание, что после этого аннотация `storageclass.kubernetes.io/is-default-class='true'` снимется со StorageClass'а, который ранее был указан в настройках модуля как используемый по умолчанию.

   ```shell
   kubectl edit mc cloud-provider-yandex
   ```

## Добавление CloudStatic-узлов в кластер

В метаданные виртуальных машин, которые вы хотите включить в кластер в качестве узлов, добавьте (Изменить ВМ -> Метадата) ключ `node-network-cidr` со значением `nodeNetworkCIDR` для кластера.

`nodeNetworkCIDR` кластера можно узнать, воспользовавшись следующей командой:

```shell
kubectl -n kube-system get secret d8-provider-cluster-configuration -o json | jq --raw-output '.data."cloud-provider-cluster-configuration.yaml"' | base64 -d | grep '^nodeNetworkCIDR'
```

## Как создать кластер в новом VPC и развернуть bastion-хост для доступа к узлам?

1. Выполните bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config.yml
   ```

2. Создайте bastion-хост:

   ```shell
   yc compute instance create \
   --name bastion \
   --hostname bastion \
   --create-boot-disk image-family=ubuntu-2204-lts,image-folder-id=standard-images,size=20,type=network-hdd \
   --memory 2 \
   --cores 2 \
   --core-fraction 100 \
   --ssh-key ~/.ssh/id_rsa.pub \
   --zone ru-central1-a \
   --public-address 178.154.226.159
   ```

3. Продолжите установку кластера, указав данные bastion-хоста. На вопрос про кэш Terraform ответьте `y`:

   ```shell
   dhctl bootstrap --ssh-bastion-host=178.154.226.159 --ssh-bastion-user=yc-user \
   --ssh-user=ubuntu --ssh-agent-private-keys=/tmp/.ssh/id_rsa --config=/config.yml --resources=/resources.yml
   ```
