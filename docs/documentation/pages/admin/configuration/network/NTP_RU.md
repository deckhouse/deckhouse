---
title: Синхронизация времени на узлах
permalink: ru/admin/network/ntp.html
lang: ru
---

Для синхронизации времени на узлах кластера Deckhouse используется встроенное решение на основе [chrony](https://chrony-project.org/).
Используя протокол Network Time Protocol (NTP),
DKP обеспечивает синхронизацию системных часов на узлах кластера с внешними NTP-серверами.
При необходимости вы можете отключить встроенный механизм и использовать собственные NTP-демоны.

## Включение встроенной синхронизации времени

Чтобы включить синхронизацию времени с настройками по умолчанию,
примените ресурс ModuleConfig, указав список NTP-серверов.
Пример конфигурации с сервером `pool.ntp.org`:

```shell
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: chrony
spec:
  enabled: true
  settings:
    ntpServers:
      - pool.ntp.org
  version: 1
EOF
```

## Использование NTP-демонов

1. Чтобы отключить встроенный механизм синхронизации времени и использовать собственные NTP-демоны на узлах,
   отключите модуль `chrony`:

   ```shell
   d8 k -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module disable chrony
   ```

   При успешном выполнении команды будет выведено сообщение о том, что модуль был отключён:

   ```console
   Module chrony disabled
   ```

1. Создайте ресурс [ресурс NodeGroupConfiguration](../../reference/cr/nodegroupconfiguration.html),
   чтобы включить NTP-демоны на узлах.
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
