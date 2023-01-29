---
title: "Cloud provider — Yandex Cloud: FAQ"
---

## Как настроить INTERNAL LoadBalancer?

Для настройки INTERNAL LoadBalancer'а установите аннотацию для сервиса:

```yaml
yandex.cpi.flant.com/listener-subnet-id: SubnetID
```

Аннотация указывает, какой subnet будет слушать LoadBalancer.

## Как зарезервировать публичный IP-адрес?

Для использования в `externalIPAddresses` и `natInstanceExternalAddress` выполните следующую команду:

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

Использование в настройках DHCP-серверов адресов DNS, отличающихся от предоставляемых Yandex Cloud, является временным решением. От него можно будет отказаться, когда Yandex Cloud введёт Managed DNS услугу. Чтобы обойти ограничения, описанные ниже, рекомендуется использовать `stubZones` из модуля [`kube-dns`](../042-kube-dns/)

### Изменение параметров

Обратите внимание на следующие особенности:

1. При изменении данных параметров требуется выполнить `netplan apply` или аналог, форсирующий обновление DHCP lease.
2. Потребуется перезапуск всех hostNetwork Pod'ов (особенно `kube-dns`), чтобы перечитать новый `resolv.conf`.

### Особенности использования

При использовании опции `dhcpOptions`, все DNS-запросы начнут идти через указанные DNS-серверы. Эти DNS-серверы **должны** разрешать внешние DNS-имена, а также, при необходимости, разрешать DNS-имена внутренних ресурсов.

**Не используйте** эту опцию, если указанные рекурсивные DNS-серверы не могут разрешать тот же список зон, что сможет разрешать рекурсивный DNS в подсети Yandex Cloud.
