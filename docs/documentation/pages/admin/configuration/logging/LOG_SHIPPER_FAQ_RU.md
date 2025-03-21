---
title: FAQ
permalink: ru/admin/configuration/logging/log-shipper/faq.html
lang: ru
---

## Как добавить авторизацию в ресурс ClusterLogDestination?

Чтобы добавить параметры авторизации в ресурс ClusterLogDestination (#TODO ссылка на CR), выполните следующее:

1. Измените протокол подключения к `loki` на HTTPS (#TODO ссылка на CR).
1. Добавьте в конфигурацию секцию `auth` (#TODO ссылка на CR), в которой:
   - для параметра `strategy` (#TODO ссылка на CR) установите значение `Bearer`;
   - для параметра `token` (#TODO ссылка на CR) укажите токен `log-shipper-token` из пространства имён `d8-log-shipper`.

Пример:

Ресурс ClusterLogDestination без авторизации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki
spec:
  type: Loki
  loki:
    endpoint: "http://loki.d8-monitoring:3100"
```

Получите токен `log-shipper-token` из пространства имён `d8-log-shipper` с помощью следующей команды:

```bash
sudo -i d8 k -n d8-log-shipper get secret log-shipper-token -o jsonpath='{.data.token}' | base64 -d
```

Ресурс ClusterLogDestination с авторизацией:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki
spec:
  type: Loki
  loki:
    endpoint: "https://loki.d8-monitoring:3100"
    auth:
      strategy: "Bearer"
      token: <log-shipper-token>
    tls:
      verifyHostname: false
      verifyCertificate: false
```
