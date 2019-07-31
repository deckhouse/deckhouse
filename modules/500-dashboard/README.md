Модуль dashboard
=======

Модуль устанавливает [dashboard](https://github.com/kubernetes/dashboard).

Модуль работает только если включен https.

Конфигурация
------------

### Что нужно настраивать?
Обязательных настроек нет.

### Параметры
* `password` — пароль для http-авторизации для пользователя `admin` (генерируется автоматически, но можно менять)
    * Используется если не включен модуль `user-authn`.
* `certificateForIngress` — выбираем, какой типа сертификата использовать для dashboard.
    * `certmanagerClusterIssuerName` — указываем, какой ClusterIssuer использовать для dashboard (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По-умолчанию `letsencrypt`.
    * `customCertificateSecretName` — указываем имя secret'а в namespace `antiopa`, который будет использоваться для dashboard (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets).
        * По-умолчанию `false`.
        * При указании этого параметра не забудьте выставить `certmanagerClusterIssuerName` в значение `false`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфига
```yaml
dashboard: |
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```
