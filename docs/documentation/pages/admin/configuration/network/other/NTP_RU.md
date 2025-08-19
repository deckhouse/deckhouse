---
title: Синхронизация времени на узлах
permalink: ru/admin/configuration/network/other/ntp.html
lang: ru
---

Для синхронизации времени на узлах кластера Deckhouse используется встроенное решение на основе [chrony](https://chrony-project.org/).
В решении используется протокол Network Time Protocol (NTP), который обеспечивает синхронизацию системных часов на узлах кластера с внешними NTP-серверами.
При необходимости вы можете отключить встроенный механизм и использовать собственные NTP-демоны.

## Включение встроенной синхронизации времени

Включите модуль [`chrony`](/modules/chrony/), чтобы включить синхронизацию времени:

```shell  
d8 platform module enable chrony
```

По умолчанию в качестве источника времени используется сервер `pool.ntp.org`. Указать список NTP-серверов можно с помощью параметра [`ntpServers`](/modules/chrony/configuration.html#parameters-ntpservers) конфигурации модуля `chrony`.

Пример конфигурации модуля с указанием NTP-серверов:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: chrony
spec:
  enabled: true
  settings:
    ntpServers:
      - ntp.ubuntu.com
      - time.google.com
  version: 1
```

## Использование собственных NTP-демонов

Чтобы отключить встроенный механизм синхронизации времени и использовать собственные NTP-демоны на узлах, выполните следующие шаги:

1. Отключите модуль [`chrony`](/modules/chrony/):

   ```shell
   d8 platform module disable chrony
   ```

   При успешном выполнении команды будет выведено сообщение о том, что модуль был отключён:

   ```console
   Module chrony disabled
   ```

2. Создайте ресурс [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration), чтобы включить NTP-демоны на узлах.

   Пример для `systemd-timesyncd`:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: enable-ntp-on-node.sh
   spec:
     weight: 100
     nodeGroups: ["*"]
     bundles: ["*"]
     content: |
       systemctl enable systemd-timesyncd
       systemctl start systemd-timesyncd
   EOF
   ```
