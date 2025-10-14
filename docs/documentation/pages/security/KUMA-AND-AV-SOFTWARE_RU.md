---
title: Интеграция с KUMA и антивирусным ПО
permalink: security/kuma-and-av-software.html
lang: ru
---

Deckhouse Platform Certified Security Edition поддерживает интеграцию с [Kaspersky Unified Monitoring and Analysis Platform (KUMA)](https://lp.kaspersky.com/ru/kuma/),
единой системой мониторинга и анализа от «Лаборатории Касперского».
В рамках интеграции события безопасности и журналы аудита из кластера передаются в KUMA для дальнейшего анализа.

## Отправка логов в KUMA

Для отправки логов в систему KUMA настройте сбор и доставку логов на стороне Deckhouse Platform Certified Security Edition,
используя следующие ресурсы:

- [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) — задаёт параметры хранилища логов;
- [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) — задаёт параметры сбора логов из кластера.

{% alert level="info" %   }
На стороне KUMA настройте соответствующие ресурсы для приёма событий.
{% endalert %}

### Примеры конфигурации ClusterLogDestination и ClusterLoggingConfig

- Отправка логов в формате JSON по UDP:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-udp-json
  spec:
    type: Socket
    socket:
      address: IP_ADDRESS:PORT # Замените при настройке.
      mode: UDP
      encoding:
        codec: "JSON"
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    type: File
    file:
      include:
      - /var/log/kube-audit/audit.log
    destinationRefs:
      - kuma-udp-json
  ```

- Отправка логов в формате JSON по TCP:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-tcp-json
  spec:
    type: Socket
    socket:
      address: IP_ADDRESS:PORT # Замените при настройке.
      mode: TCP
      tcp:
        verifyCertificate: false
        verifyHostname: false
      encoding:
        codec: "JSON"
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    type: File
    file:
      include:
      - /var/log/kube-audit/audit.log
    destinationRefs:
      - kuma-tcp-json
  ```

- Отправка логов в формате CEF по TCP:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-tcp-cef
  spec:
    type: Socket
    socket:
      extraLabels:
        cef.name: d8
        cef.severity: "1"
      address: IP_ADDRESS:PORT # Замените при настройке.
      mode: TCP
      tcp:
        verifyCertificate: false
        verifyHostname: false
      encoding:
        codec: "CEF"
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    type: File
    file:
      include:
      - /var/log/kube-audit/audit.log
    logFilter:
      - field: userAgent
        operator: Regex
        values: [ "kubelet.*" ]
    destinationRefs:
      - kuma-tcp-cef
  ```

- Отправка логов в формате Syslog по TCP:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-tcp-syslog
  spec:
    type: Socket
    socket:
      address: IP_ADDRESS:PORT # Замените при настройке.
      mode: TCP
      tcp:
        verifyCertificate: false
        verifyHostname: false
      encoding:
        codec: "Syslog"
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    type: File
    file:
      include:
      - /var/log/kube-audit/audit.log
    logFilter:
      - field: userAgent
        operator: Regex
        values: [ "kubelet.*" ]
    destinationRefs:
      - kuma-tcp-syslog
  ```

- Отправка логов в Apache Kafka:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-kafka
  spec:
    type: Kafka
    kafka:
      bootstrapServers:
        - kafka-address:9092 # Замените при настройке.
      topic: k8s-logs
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    destinationRefs:
      - kuma-kafka
    file:
      include:
        - /var/log/kube-audit/audit.log
    logFilter:
      - field: userAgent
        operator: Regex
        values:
          - kubelet.*
    type: File
  ```

## Исключения при антивирусном сканировании узлов

Если на узлах кластера Deckhouse Platform Certified Security Edition используются антивирусные средства (например, Kaspersky Endpoint Security (KESL)),
вам может понадобиться исключить из анализа служебные директории Deckhouse, чтобы избежать ложных срабатываний.

Перечень служебных директорий Deckhouse ([также доступен в CSV](/products/kubernetes-platform/documentation/v1/deckhouse-directories.csv)):

| Директория | Назначение |
| ---------- | ---------- |
| `/etc/cni/` (любой узел) | Файлы настройки модуля CNI |
| `/etc/kubernetes` (любой узел) | Манифесты статических подов, файлы сертификатов PKI |
| `/mnt/kubernetes-data` (master-узел) | Существует только в кластерах, развёрнутых в облаке, когда используется отдельный диск для базы данных etcd |
| `/mnt/vector-data` (любой узел, модуль `log-shipper`) | Служебные данные статуса отправленных журналов |
| `/opt/cni/bin/` (любой узел) | Исполняемые файлы модуля CNI |
| `/opt/deckhouse/bin/` (любой узел) | Исполняемые файлы, необходимые для работы Deckhouse |
| `/var/lib/bashible` (любой узел) | Файлы настройки узла |
| `/var/lib/containerd` (любой узел) | Используется для хранения данных, связанных с работой CRI (например, containerd). Содержит слои образов контейнеров, снимки файловых систем контейнеров метаинформацию, логи и другую информацию контейнеров |
| `/var/lib/deckhouse/` (master-узел) | Файлы модулей Deckhouse, динамически загружаемых из хранилища образов |
| `/var/lib/etcd` (master-узел) | База данных etcd |
| `/var/lib/kubelet/` (любой узел) | Файлы настройки kubelet |
| `/var/lib/upmeter` (master-узел, модуль `upmeter`) | База данных [модуля `upmeter`](/modules/upmeter/) |
| `/var/log/containers` (любой узел) | Журналы контейнеров (при использовании containerd) |
| `/var/log/pods/` (любой узел) | Журналы всех контейнеров подов, запущенных на узле |

### Рекомендации по настройке KESL

Для корректной работы Deckhouse Platform Certified Security Edition при наличии установленного решения KESL выполните следующие шаги:

1. Отключите следующие задачи на стороне KESL:

   - `Firewall Management (ID: 12)`;
   - `Web Threat Protection (ID: 14)`;
   - `Network Threat Protection (ID: 17)`;
   - `Web Control (ID: 26)`.

   > В будущих версиях KESL список задач может отличаться.

1. Убедитесь, что ресурсы узлов соответствуют требованиям. В том числе для [KESL](https://support.kaspersky.com/KES4Linux/12.1.0/ru-RU/197642.htm).

1. Для оптимизации производительности следуйте [официальным рекомендациям «Лаборатории Касперского»](https://support.kaspersky.com/KES4Linux/12.1.0/ru-RU/206054.htm).
