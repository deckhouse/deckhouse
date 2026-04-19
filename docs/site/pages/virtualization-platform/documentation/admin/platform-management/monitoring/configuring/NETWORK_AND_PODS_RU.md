---
title: "Настройка мониторинга сетевого взаимодействия и узлов кластера"
permalink: ru/virtualization-platform/documentation/admin/platform-management/monitoring/configuring/network-and-pods.html
lang: ru
---

## Мониторинг сетевого взаимодействия

DVP может выполнять мониторинг сетевого взаимодействия между всеми узлами кластера, а также между узлами кластера и внешними хостами. При настроенном мониторинге, каждый узел два раза в секунду отправляет ICMP-пакеты на все другие узлы кластера (и на опциональные внешние узлы) и экспортирует данные в систему мониторинга.

Модуль `monitoring-ping` отслеживает любые изменения поля `.status.addresses` узла. Если они обнаружены, срабатывает хук, который собирает полный список имен узлов и их адресов, и передает в DaemonSet, который заново создает поды. Таким образом, `ping` проверяет всегда актуальный список узлов.

{% alert level="warning" %}
Модуль `monitoring-ping` должен быть включен.
{% endalert %}

### Добавление дополнительных IP-адресов для мониторинга

Для добавления дополнительных IP-адресов мониторинга используйте параметр [`externalTargets`](/modules/monitoring-ping/configuration.html#parameters-externaltargets) модуля `monitoring-ping`.

Пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-ping
spec:
  version: 1
  enabled: true
  settings:
    externalTargets:
    - name: google-primary
      host: 8.8.8.8
    - name: yaru
      host: ya.ru
    - host: youtube.com
```

> Поле `name` используется в Grafana для отображения связанных данных. Если поле `name` не указано, используется обязательное поле `host`.

## Мониторинг узлов кластера

Чтобы включить мониторинг узлов кластера, необходимо включить модуль `monitoring-kubernetes`, если он не включен. Включить мониторинг кластера можно в [веб-интерфейсе Deckhouse](/modules/console/stable/), или с помощью следующей команды:

```shell
d8 system module enable monitoring-kubernetes
```

Аналогично можно включить модули [`monitoring-kubernetes-control-plane`](/modules/monitoring-kubernetes-control-plane/) и [`extended-monitoring`](/modules/extended-monitoring/).
