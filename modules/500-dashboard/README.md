Модуль dashboard
=======

Модуль устанавливает [dashboard](https://github.com/kubernetes/dashboard).

Конфигурация
------------

### Что нужно настраивать?
Обязательных настроек нет.

### Настройка Gitlab если используем
Регистрируем в гитлаб новое приложение. Для этого идем Admin area -> Applications -> New application
Redirect URI(Callback url) устанавливаем вида https://dashboard.example.com/oauth2/callback

### Параметры
* `gitlabBaseUrl` — url gitlab для авторизации (https://git.example.com)
* `oauth2ProxyClientId`  — `Application Id` в Admin Area -> Applications gitlab
* `oauth2ProxyClientSecret` — `Secret` в `Admin Area -> Applications gitlab`
* `oauth2ProxyCookieSecret` —  генерируется автоматически если есть `gitlabBaseUrl`
* `password` — пароль для http-авторизации, используется если не задан `gitlabBaseUrl` (генерируется автоматически)
* `certificateForIngress` — выбираем, какой типа сертификата использовать для dashboard.
    * `certmanagerClusterIssuerName` — указываем, какой ClusterIssuer использовать для dashboard (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По-умолчанию `letsencrypt`.
    * `customCertificateSecretName` — указываем имя secret'а в namespace `antiopa`, который будет использоваться для dashboard (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets).
        * По-умолчанию `false`.
        * При указании этого параметра не забудьте выставить `certmanagerClusterIssuerName` в значение `false`.
    * Если вы хотите отключить https, то оба параметра необходимо выставить в `false`.
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
