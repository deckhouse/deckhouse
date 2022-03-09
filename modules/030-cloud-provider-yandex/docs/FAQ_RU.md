---
title: "Cloud provider — Yandex.Cloud: FAQ"
---

## Как настроить INTERNAL LoadBalancer?

Установить аннотацию для сервиса:
```
yandex.cpi.flant.com/listener-subnet-id: SubnetID
```
Аннотация указывает, какой Subnet будет слушать LB.

## Как зарезервировать публичный IP-адрес?

Для использования в `externalIPAddresses` и `natInstanceExternalAddress`.

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

Использование в настройках DHCP серверов DNS, отличающихся от предоставляемых yandex облаком, является временным решением, пока Yandex.Cloud не введёт Managed DNS услугу. Чтобы обойти ограничения, описанные ниже, рекомендуется использовать `stubZones` из модуля [`kube-dns`]({{"/modules/042-kube-dns/" | true_relative_url }} )

### Изменение параметров

1. При изменении данных параметров требуется выполнить `netplan apply` или аналог, форсирующий обновление DHCP lease.
2. Потребуется перезапуск всех hostNetwork Pod'ов (особенно `kube-dns`), чтобы перечитать новый `resolv.conf`.

### Особенности использования

При использовании опции все DNS запросы начнут идти через указанные DNS сервера. Эти DNS **обязаны** отвечать на DNS запросы во внешний интернет, плюс, по желанию, предоставлять резолв интранет ресурсов. **Не используйте** эту опцию, если указанные рекурсивные DNS не могут резолвить тот же список зон, что сможет резолвить рекурсивный DNS в подсети Yandex.Cloud.
