---
title: "Модуль dashboard: конфигурация"
---

Обязательных настроек нет.

## Параметры
* `ingressClass` — класс ingress контроллера, который используется для dashboard.
    * Опциональный параметр, по умолчанию используется глобальное значение `modules.ingressClass`.
* `accessLevel` — уровень доступа в dashboard при отсутствии внешней аутентификации `externalAuthentication`. Возможные значения описаны в [user-authz](/modules/140-user-authz/).
    * По-умолчанию: `User`.
* `auth` — опции, связанные с аутентификацией или авторизацией в приложении:
    * `externalAuthentication` - параметры для подключения внешней аутентификации (используется механизм Nginx Ingress [external-auth](https://kubernetes.github.io/ingress-nginx/examples/auth/external-auth/), работающей на основе модуля Nginx [auth_request](http://nginx.org/en/docs/http/ngx_http_auth_request_module.html).
         * `authURL` - URL сервиса аутентификации. Если пользователь прошел аутентификацию, сервис должен возвращать код ответа HTTP 200.
         * `authSignInURL` - URL, куда будет перенаправлен пользователь для прохождения аутентификации (если сервис аутентификации вернул код ответа HTTP отличный от 200).
         * `useBearerTokens` – dashboard должен работать с Kubernetes API от имени пользователя (сервис аутентификации при этом должен обязательно возвращать в своих ответах HTTP-заголовок Authorization, в котором должен быть bearer-token – именно под этим токеном dashboard будет производить запросы к API-серверу Kubernetes).
             * Значение по умолчанию: `false`.
             * Важно! Из соображений безопасности этот режим работает только если https.mode (глобальный, или в модуле) не установлен в значение Disabled.
    * `password` — пароль для http-авторизации для пользователя `admin` (генерируется автоматически, но можно менять)
         * Используется если не включен параметр `externalAuthentication`.
    * `whitelistSourceRanges` — массив CIDR, которым разрешено проходить аутентификацию для доступа в dashboard.
    * `allowScale` — если указать данный параметр в `true`, то в Kubernetes Dashboard появится возможность скейлить deployment и statefulset.
         * Используется если не включен параметр `externalAuthentication`.
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
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
