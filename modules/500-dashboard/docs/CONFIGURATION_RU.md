---
title: "Модуль dashboard: настройки"
---

Обязательных настроек нет.

## Параметры

* `ingressClass` — класс Ingress-контроллера, который используется для dashboard.
  * Опциональный параметр, по умолчанию используется глобальное значение `modules.ingressClass`.
* `auth` — опции, связанные с аутентификацией или авторизацией в приложении:
  * `externalAuthentication` — параметры для подключения внешней аутентификации (используется механизм Nginx Ingress [external-auth](https://kubernetes.github.io/ingress-nginx/examples/auth/external-auth/), работающий на основе модуля Nginx [auth_request](http://nginx.org/en/docs/http/ngx_http_auth_request_module.html):
    * `authURL` — URL сервиса аутентификации. Если пользователь прошел аутентификацию, сервис должен возвращать код ответа HTTP 200.
    * `authSignInURL` — URL, куда будет перенаправлен пользователь для прохождения аутентификации (если сервис аутентификации вернул код ответа HTTP, отличный от 200).
    * `useBearerTokens` – Токены авторизации. dashboard должен работать с Kubernetes API от имени пользователя (сервис аутентификации при этом должен обязательно возвращать в своих ответах HTTP-заголовок Authorization, в котором должен быть bearer-token – именно под этим токеном dashboard будет производить запросы к API-серверу Kubernetes).
      * Значение по умолчанию: `false`.
      * **Важно!** Из соображений безопасности этот режим работает только если `https.mode` (глобальный или в модуле) не установлен в значение `Disabled`.
  * `password` — пароль HTTP-авторизации для пользователя `admin` (генерируется автоматически, но можно менять).
    * Используется, если не включен параметр `externalAuthentication`.
  * `whitelistSourceRanges` — массив CIDR, которым разрешено проходить аутентификацию для доступа в dashboard.
  * `allowScale` — активация возможности скейлить Deployment и StatefulSet из web-интерфейса.
    * Используется, если не включен параметр `externalAuthentication`.
* `https` — тип сертификата, используемого для dashboard.
  * При использовании этого параметра полностью переопределяются глобальные настройки `global.modules.https`.
  * `mode` — режим работы HTTPS:
    * `Disabled` — в данном режиме dashboard будет работать только по HTTP;
    * `CertManager` — dashboard будет работать по HTTPS и заказывать сертификат с помощью ClusterIssuer, заданном в параметре `certManager.clusterIssuerName`;
    * `CustomCertificate` — dashboard будет работать по HTTPS, используя сертификат из пространства имен `d8-system`;
    * `OnlyInURI` — dashboard будет работать по HTTP (подразумевая, что перед ними стоит внешний HTTPS балансер, который терминирует HTTPS) и все ссылки в `user-authn` будут генерироваться с HTTPS-схемой.
  * `certManager`
    * `clusterIssuerName` — тип используемого ClusterIssuer (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
      * По умолчанию `letsencrypt`.
  * `customCertificate`
    * `secretName` — имя Secret'а в пространстве имен `d8-system`, который будет использоваться для dashboard (данный Secret должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
      * По умолчанию `false`.
* `nodeSelector` — аналогично параметру Kubernetes `spec.nodeSelector` у Pod'ов.
  * Если ничего не указано или указано `false` — будет [использоваться автоматика](../../#выделение-узлов-под-определенный-вид-нагрузки).
* `tolerations` — аналогично параметру Kubernetes `spec.tolerations` у Pod'ов.
  * Если ничего не указано или указано `false` — будет [использоваться автоматика](../../#выделение-узлов-под-определенный-вид-нагрузки).
* `accessLevel` — уровень доступа в dashboard, если отключен модуль `user-authn` и не включена внешняя аутентификация (`externalAuthentication`). Возможные значения описаны [в user-authz](../../modules/140-user-authz/).
  * По умолчанию: `User`.
  * В случае использования модуля `user-authn` или другой внешней аутентификации (`externalAuthentication`) права доступа необходимо настраивать при помощи модуля `user-authz`.
