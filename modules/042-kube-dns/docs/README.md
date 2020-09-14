---
title: "Модуль kube-dns"
---

Модуль устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes.

**Note!!** Модуль удаляет ранее установленны kubeadm'ом Deployment, ConfigMap и RBAC для CoreDNS.

## Конфигурация

### Включение модуля

Модуль по-умолчанию **включен**.

### Параметры

* `clusterDomain` - домен для сервисов в кластере.
  * Формат — строка.
  * При отсутствии параметра модуль попытается получить его из существующей инсталляции CoreDNS.
* `upstreamNameservers` — список IP-адресов рекурсивных DNS-серверов, которые CoreDNS будет использовать для резолва внешних доменов.
  * Формат — список строк.
  * По-умолчанию, список из `/etc/resolv.conf`.
* `hosts` — статически список хостов в стиле `/etc/hosts`.
  * Формат — список ассоциативных массивов с ключами `domain` и `ip`.
  * Опциональный параметр.
* `enableLogs` - позволяет включить логирование в CoreDNS:
  * Формат - true или false
  * По-умолчанию, false

### Пример конфигурации

```yaml
kubeDns: |
  clusterDomain: cluster.local
  upstreamNameservers:
  - 8.8.8.8
  - 8.8.4.4
  hosts:
  - domain: one.example.com
    ip: 192.168.0.1
  - domain: two.another.example.com
    ip: 10.10.0.128
```
