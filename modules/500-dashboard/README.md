Модуль dashboard
=======

Модуль устанавливает [dashboard](https://github.com/kubernetes/dashboard).

В случае работы модуля по http, он работает с минимальными правами с ролью `User` из модуля: [user-authz](../140-user-authz/README.md).

Если модуль `user-authz` отключен, то dashboard не выкатывается.

Конфигурация
------------

### Что нужно настраивать?
Обязательных настроек нет.

### Параметры
* `password` — пароль для http-авторизации для пользователя `admin` (генерируется автоматически, но можно менять)
    * Используется если не включен модуль `user-authn`.
* `ingressClass` — класс ingress контроллера, который используется для dashboard.
    * Опциональный параметр, по-умолчанию используется глобальное значение `modules.ingressClass`.
* `https` — выбираем, какой типа сертификата использовать для dashboard.
    * `mode` — режим работы HTTPS:
        * `Disabled` — в данном режиме dashboard будет работать только по http;
        * `CertManager` — dashboard будет работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
        * `CustomCertificate` — dashboard будут работать по https используя сертификат из namespace `antiopa`;
        * `UriOnly` — dashboard будет работать по http (подразумевая, что перед ними стоит внешний https балансер, который терминирует https) и все ссылки в `user-authn` будут генерироваться с https схемой.
    * `certManager`
      * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для dashboard (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По-умолчанию `letsencrypt`.
    * `customCertificate`
      * `secretName` - указываем имя secret'а в namespace `antiopa`, который будет использоваться для dashboard (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
        * По-умолчанию `false`.
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
