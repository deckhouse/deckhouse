---
title: "Управление NTP"
permalink: ru/virtualization-platform/documentation/admin/platform-management/network/ntp.html
lang: ru
---

## Настройка синхронизации времени на узлах

Для настройки синхронизации времени на узлах можно использовать модуль chrony или заменить его на собственные NTP-демоны.

Чтобы включить модуль kube-dns с настройками по умолчанию, примените ресурс `ModuleConfig` указав свои NTP-сервера для синхронизации. Пример конфигурации с NTP-сервером по умолчанию:

```yaml
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

Чтобы запретить использование chrony и использовать NTP-демоны на узлах, выключите модуль:

```shell
d8 k -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module disable chrony

# Module chrony disabled
```

Создайте [NodeGroupConfiguration](todo) custom step, чтобы включить NTP-демоны на узлах (пример для systemd-timesyncd):

```yaml
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
