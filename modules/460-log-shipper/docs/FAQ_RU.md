---
title: "The log-shipper module: FAQ"
---

## Как добавить авторизацию в ресурс _ClusterLogDestination_?

Чтобы добавить параметры авторизации в ресурс [ClusterLogDestination](cr.html#clusterlogdestination), необходимо:
- изменить [протокол](cr.html#clusterlogdestination-v1alpha1-spec-loki-endpoint) подключения к Loki на HTTPS;
- добавить секцию [auth](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth), в которой:
  - параметр [strategy](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth-strategy) установить в `Bearer`;
  - в параметре [token](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth-token) указать токен `log-shipper-token` из пространства имен `d8-log-shipper`.

Пример:

- Ресурс _ClusterLogDestination_ без авторизации:

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

- Получите токен `log-shipper-token` из пространства имен `d8-log-shipper`:

  ```bash
  kubectl -n d8-log-shipper get secret log-shipper-token -o jsonpath='{.data.token}' | base64 -d
  ```

- Ресурс _ClusterLogDestination_ с авторизацией:

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
