---
title: "Модуль kube-dns"
---

Модуль устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes.

**Note!!** Модуль удаляет ранее установленны kubeadm'ом Deployment, ConfigMap и RBAC для CoreDNS.

## Конфигурация

### Включение модуля

Модуль по умолчанию **включен**.

### Параметры

* `upstreamNameservers` — список IP-адресов рекурсивных DNS-серверов, которые CoreDNS будет использовать для резолва внешних доменов.
  * Формат — список строк.
  * По умолчанию, список из `/etc/resolv.conf`.
* `hosts` — статически список хостов в стиле `/etc/hosts`.
  * Формат — список ассоциативных массивов с ключами `domain` и `ip`.
  * Опциональный параметр.
* `stubZones` — список дополнительных зон для обслуживания CoreDNS.
  * `zone` — зона CoreDNS.
      * Пример: `consul.local:53`
  * `upstreamNameservers` — список IP-адресов рекурсивных DNS-серверов, которые CoreDNS будет использовать для резолва доменов в этой зоне.
* `enableLogs` - позволяет включить логирование в CoreDNS:
  * Формат - true или false
  * По умолчанию, false
* `clusterDomainAliases` — <a name="clusterDomainAliases"></a> список алиасов домена кластера, резолвятся наравне с `global.discovery.clusterDomain`

### Пример конфигурации

```yaml
kubeDns: |
  upstreamNameservers:
  - 8.8.8.8
  - 8.8.4.4
  hosts:
  - domain: one.example.com
    ip: 192.168.0.1
  - domain: two.another.example.com
    ip: 10.10.0.128
  stubZones:
  - zone: consul.local:53
    upstreamNameservers:
    - 10.150.0.1
  enableLogs: true
  clusterDomainAliases:
  - foo.bar
  - baz.qux
```
