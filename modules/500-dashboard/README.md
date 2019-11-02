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
    * Используется если не включена внешняя аутентификация `externalAuthentication`.
* `ingressClass` — класс ingress контроллера, который используется для dashboard.
    * Опциональный параметр, по-умолчанию используется глобальное значение `modules.ingressClass`.
* `accessLevel` — уровень доступа в dashboard при отсутсвии внешней аутентификации `externalAuthentication`. Возможные значения описаны в [user-authz](../140-user-authz/README.md).
  * По-умолчанию: `User`.
* `allowScale` — если указать данный параметр в `true`, то в Kubernetes Dashboard появится возможность скейлить deployment и statefulset.
* `https` — выбираем, какой типа сертификата использовать для dashboard.
    * При использовании этого параметра полностью переопределяются глобальные настройки `global.modules.https`.
    * `mode` — режим работы HTTPS:
        * `Disabled` — в данном режиме dashboard будет работать только по http;
        * `CertManager` — dashboard будет работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
        * `CustomCertificate` — dashboard будет работать по https используя сертификат из namespace `d8-system`;
        * `OnlyInURI` — dashboard будет работать по http (подразумевая, что перед ними стоит внешний https балансер, который терминирует https) и все ссылки в `user-authn` будут генерироваться с https схемой.
    * `certManager`
      * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для dashboard (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По-умолчанию `letsencrypt`.
    * `customCertificate`
      * `secretName` - указываем имя secret'а в namespace `d8-system`, который будет использоваться для dashboard (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
        * По-умолчанию `false`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* `externalAuthentication` - параметры для подключения внешней аутентификации (используется механизм Nginx Ingress [external-auth](https://kubernetes.github.io/ingress-nginx/examples/auth/external-auth/), работающей на основе модуля Nginx [auth_request](http://nginx.org/en/docs/http/ngx_http_auth_request_module.html).
     * `authURL` - URL сервиса аутентификации. Если пользователь прошел аутентификацию, сервис должен возвращать код ответа HTTP 200.
     * `authSignInURL` - URL, куда будет перенаправлен пользователь для прохождения аутентификации (если сервис аутентификации вернул код ответа HTTP отличный от 200).
     * `useBearerTokens` – dashboard должен работать с Kubernetes API от имени пользователя (сервис аутентификации при этом должен обязательно возвращать в своих ответах HTTP-заголовок Authorization, в котором должен быть bearer-token – именно под этим токеном dashboard будет производить запросы к API-серверу Kubernetes).
         * Значение по-умолчанию: `false`.
         * Важно! Из соображений безопасности этот режим работает только если https.mode (глобальный, или в модуле) не установлен в значение Disabled.
### Пример конфига
```yaml
dashboard: |
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
  externalAuthentication:
    authURL: "https://<applicationDomain>/auth"
    authSignInURL: "https://<applicationDomain>/sign-in"
    authResponseHeaders: "Authorization"
```
