---
title: "Модуль user-authn: Custom Resources"
---

## DexAuthenticator

При появлении объекта DexAuthenticator в неймспейсе будут созданы:
* Deployment с oauth2-proxy и redis
* Service, ведущий на Deployment с oauth2-proxy
* Ingress, который принимает запросы по адресу `https://<applicationDomain>/dex-authenticator` и отправляет их в сторону сервиса
* Secret'ы, необходимые для доступа к dex

**Важно!** При перезапуске pod'а с oauth2-proxy при помощи refresh token'а будут получены и сохранены в память (redis) актуальные access token и id token.

### Параметры
* `applicationDomain` — внешний адрес вашего приложения, с которого пользовательский запрос будет перенаправлен для авторизации в dex.
    * Формат — строка с адресом (пример: `my-app.kube.my-domain.com`, обязательно НЕ указывать HTTP схему.
* `sendAuthorizationHeader` — флаг, который отвечает за отправку конечному приложению header'а `Authorization: Bearer`.
     * Включать только если ваше приложение умеет этот header обрабатывать.
* `keepUsersLoggedInFor` — отвечает за то, как долго пользовательская сессия будет считаться активной, если пользователь бездействует (указывается с суффиксом s, m или h).
    * По-умолчанию — 7 дней (`168h`).
* `applicationIngressCertificateSecretName` — имя secret'а с TLS-сертификатом (от домена `applicationDomain`), который используется в Ingress объекте вашего приложения. Secret должен обязательно находится в том же неймспейсе, что и DexAuthenticator.
* `applicationIngressClassName` — имя Ingress класса, который будет использоваться в ingress-объекте (должно совпадать с именем ingress класса для `applicationDomain`).
* `allowedGroups` — группы, пользователям которых разрешено проходить аутентификацию. Дополнительно, опция помогает ограничить список групп до тех, которые несут для приложения полезную информацию (для примера у пользователя 50+ групп, но приложению grafana мы хотим передать только определенные 5). 
    * По умолчанию разрешены все группы.
* `whitelistSourceRanges` — список CIDR, которым разрешено проходить аутентификацию. 
    * Если параметр не указан, аутентификацию разрешено проходить без ограничения по IP-адресу.

### Примеры
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexAuthenticator
metadata:
  name: my-cool-app # поды аутентификатора будут иметь префикс my-cool-app
  namespace: my-cool-namespace # неймспейс, в котором будет развернут dex-authenticator
spec:
  applicationDomain: "my-app.kube.my-domain.com" # домен, на котором висит ваше приложение
  sendAuthorizationHeader: false # отправлять ли `Authorization: Bearer` header приложению, полезно в связке с auth_request в nginx
  applicationIngressCertificateSecretName: "ingress-tls" # имя секрета с tls сертификатом
  applicationIngressClassName: "nginx"
  keepUsersLoggedInFor: "720h"
  allowedGroups:
  - everyone
  - admins
  whitelistSourceRanges:
  - 1.1.1.1
  - 192.168.0.0/24
```
{% endraw %}

После появления CR `DexAuthenticator` в кластере, в указанном namespace'е появятся необходимые deployment, service, ingress, secret.
Чтобы подключить своё приложение к dex, достаточно будет добавить в Ingress-ресурс вашего приложения следующие аннотации:

{% raw %}
```yaml
annotations:
  nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
  nginx.ingress.kubernetes.io/auth-url: https://my-cool-app-dex-authenticator.my-cool-namespace.svc.{{ домен вашего кластера, например | cluster.local }}/dex-authenticator/auth
  nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
```
{% endraw %}

#### Настройка ограничений на основе CIDR

В DexAuthenticator нет встроенной системы управления разрешением аутентификации на основе IP адреса пользователя. Вместо этого вы можете воспользоваться аннотациями для Ingress-ресурсов:

* Если нужно ограничить доступ по IP и оставить прохождение аутентификации в dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:
```yaml
nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1`
```
* Если вы хотите, чтобы пользователи из указанных сетей были освобождены от прохождения аутентификации в dex, а пользователи из остальных сетей были обязаны аутентифицироваться в dex - добавьте следующую аннотацию:
```yaml
nginx.ingress.kubernetes.io/satisfy: "any"
```

## DexClient

Позволяет приложениям, поддерживающим DC-аутентификацию взаимодействовать с dex.

### Параметры
* `redirectURIs` — список адресов, на которые допустимо редиректить dex'у после успешного прохождения аутентификации.
* `trustedPeers` — id клиентов, которым позволена cross аутентификация. [Подробнее тут](https://developers.google.com/identity/protocols/CrossClientAuth).
* `allowedGroups` — список групп, участникам которых разрешено подключаться к этому клиенту;
    * По умолчанию разрешено всем группам.

### Примеры
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexClient
metadata:
  name: myname
  namespace: mynamespace
spec:
  redirectURIs:
  - https://app.example.com/callback
  - https://app.example.com/callback-reserve
  allowedGroups:
  - Everyone
  - admins
  trustedPeers:
  - opendistro-sibling
```
{% endraw %}

## User

Содержит информацию о статическом пользователе.

### Параметры

* `userID` — имя пользователя
* `email` — e-mail пользователя
* `password` — хэшированный пароль пользователя
  * Для получения хэшированного пароля можно воспользоваться командой `echo "$password" | htpasswd -inBC 10 "" | tr -d ':\n' | sed 's/$2y/$2a/'`
* `groups` — массив групп, в которых у пользователя есть членство

### Примеры
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: User
metadata:
  name: admin
spec:
  email: admin@yourcompany.com
  password: $2a$10$etblbZ9yfZaKgbvysf1qguW3WULdMnxwWFrkoKpRH1yeWa5etjjAa
  userID: some-unique-user-id
  groups:
  - Everyone
  - admins
```
{% endraw %}