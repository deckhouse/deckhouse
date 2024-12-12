---
title: Настройка ПО безопасности для работы с Deckhouse
permalink: ru/security_software_setup.html
lang: ru
---

Если узлы кластера Kubernetes анализируются сканерами безопасности (антивирусными средствами), то может потребоваться их настройка для исключения ложноположительных срабатываний.

Deckhouse Kubernetes Platform (DKP) использует следующие директории при работе ([скачать в csv](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

## SIEM - ПО для сбора и анализа информации о событиях безопасности

SIEM (Security information and event management) — класс программных продуктов, предназначенных для сбора и анализа информации о событиях безопасности.

### KUMA (Kaspersky Unified Monitoring and Analysis Platform)

KUMA объединяет продукты «Лаборатории Касперского» и сторонних поставщиков в единую систему информационной безопасности и является ключевым компонентом на пути реализации комплексного защитного подхода, способного обезопасить от актуальных киберугроз корпоративную и индустриальную среду, а также наиболее эксплуатируемый злоумышленниками стык IT/OT-систем.

#### Описание настроек

{% alert level="warning" %}
Для работы с KUMA должен быть **обязательно включён** модуль [log-shipper](./modules/log-shipper/).
{% endalert %}

Для отправки данных [в KUMA](https://go.kaspersky.com/ru-kuma) необходимо настроить на стороне DKP следующие ресурсы:

- [`ClusterLogDestination`](/modules/log-shipper/cr.html#clusterlogdestination);
- [`ClusterLoggingConfig`](/modules/log-shipper/cr.html#clusterloggingconfig).

{% alert level="info" %}
На стороне KUMA должны быть настроены необходимые ресурсы для приёма событий.
{% endalert %}

Ниже приведены примеры конфигурации отправки файла аудита `/var/log/kube-audit/audit.log` в различных форматах.

#### Отправка логов в формате JSON по UDP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-udp-json
spec:
  type: Socket
  socket:
    address: IP_ADDRESS:PORT # Заменить при настройке
    mode: UDP
    encoding:
      codec: "JSON"
---
apiVersion: deckhouse.io/v1alpha1
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

#### Отправка логов в формате JSON по TCP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-tcp-json
spec:
  type: Socket
  socket:
    address: IP_ADDRESS:PORT # Заменить при настройке
    mode: TCP
    tcp:
      verifyCertificate: false
      verifyHostname: false
    encoding:
      codec: "JSON"
---
apiVersion: deckhouse.io/v1alpha1
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

#### Отправка логов в формате CEF по TCP

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
    address: IP_ADDRESS:PORT # Заменить при настройке
    mode: TCP
    tcp:
      verifyCertificate: false
      verifyHostname: false
    encoding:
      codec: "CEF"
---
apiVersion: deckhouse.io/v1alpha1
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

#### Отправка логов в формате Syslog по TCP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-tcp-syslog
spec:
  type: Socket
  socket:
    address: IP_ADDRESS:PORT # Заменить при настройке
    mode: TCP
    tcp:
      verifyCertificate: false
      verifyHostname: false
    encoding:
      codec: "Syslog"
---
apiVersion: deckhouse.io/v1alpha1
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

#### Отправка логов в Kafka

{% alert level="info" %}
При условии, что Kafka настроена на приём данных.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-kafka
spec:
  type: Kafka
  kafka:
    bootstrapServers:
      - kafka-address:9092 # Заменить при настройке на актуальное значение
    topic: k8s-logs
---
apiVersion: deckhouse.io/v1alpha1
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

## ПО безопасности

В разделе приведены примеры настроек для различного ПО безопасности.

### KESL (Kaspersky Endpoint Security for Linux)

Далее приведены рекомендации по настройке Kaspersky Endpoint Security for Linux (KESL) для обеспечения корректной работы с платформой Deckhouse Kubernetes Platform, независимо от выбранной редакции.

Для обеспечения совместимости с DKP на стороне KESL необходимо отключить следующие задачи:

- `Firewall_Management (ID: 12)`.
- `Web Threat Protection (ID: 14)`.
- `Network Threat protection (ID: 17)`.
- `Web Control (ID: 26)`.

{% alert level="info" %}
Список задач может отличаться в будущих версиях KESL.
{% endalert %}

Убедитесь, что узлы Kubernetes соответствуют минимальным требованиям к ресурсам, указанным для [DKP](https://deckhouse.ru/products/kubernetes-platform/guides/production.html#требования-к-ресурсам) и [KESL](https://support.kaspersky.com/KES4Linux/12.1.0/ru-RU/197642.htm).

При совместной эксплуатации KESL и DKP может потребоваться оптимизация производительности согласно [рекомендациям Kaspersky](https://support.kaspersky.com/KES4Linux/12.1.0/ru-RU/206054.htm).
