---
title: "Модуль deckhouse-web: настройки"
---

## Параметры

* `ingressClass` — класс ingress-контроллера web-интерфейса документации.
    * Опциональный параметр, по умолчанию используется глобальное значение `modules.ingressClass`.
* `auth` — опции, связанные с аутентификацией и авторизацией доступа к web-интерфейсу документации:
    * `externalAuthentication` - параметры для подключения внешней аутентификации (используется механизм Nginx Ingress [external-auth](https://kubernetes.github.io/ingress-nginx/examples/auth/external-auth/), работающей на основе модуля Nginx [auth_request](http://nginx.org/en/docs/http/ngx_http_auth_request_module.html).
         * `authURL` - URL сервиса аутентификации. Если пользователь прошел аутентификацию, сервис должен возвращать код ответа HTTP 200.
         * `authSignInURL` - URL, куда будет перенаправлен пользователь для прохождения аутентификации (если сервис аутентификации вернул код ответа HTTP отличный от 200).
    * `password` — пароль для http-авторизации для пользователя `admin` (генерируется автоматически, но можно менять)
         * Используется если не включен параметр `externalAuthentication`.
* `https` — выбираем, какой типа сертификата использовать для web-интерфейса документации.
    * При использовании этого параметра полностью переопределяются глобальные настройки `global.modules.https`.
    * `mode` — режим работы HTTPS:
        * `Disabled` — в данном режиме доступ к web-интерфейсу документации будет только по HTTP;
        * `CertManager` — доступ по HTTPS с заказом сертификата согласно clusterIssuer заданного в параметре `certManager.clusterIssuerName`;
        * `CustomCertificate` — доступ по HTTPS, с использованием сертификата из namespace `d8-system`;
        * `OnlyInURI` — web-интерфейс документации будет доступен по HTTP (подразумевая, что перед ним стоит внешний HTTPS-балансер, который терминирует HTTPS) и все ссылки в `user-authn` будут генерироваться с HTTPS-схемой.
    * `certManager`
      * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для заказа SSL-сертификата (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По умолчанию `letsencrypt`.
    * `customCertificate`
      * `secretName` - указываем имя secret'а в namespace `d8-system`, который будет использоваться для web-интерфейса документации (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
        * По умолчанию `false`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример

```yaml
deckhouseWeb: |
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
