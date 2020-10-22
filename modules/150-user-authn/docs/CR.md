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

## DexProvider

Описывает конфигурацию подключения стороннего провайдера. С его помощью можно гибко настроить интеграцию вашего каталога учетных записей с Kubernetes.

### Параметры
* `type` — тип внешнего провайдера, в данный момент поддерживается 6 типов: `Github`, `Gitlab`, `BitbucketCloud`, `Crowd`, `OIDC`, `LDAP`;
* `displayName` — имя провайдера, которое будет отображено на странице выбора провайдера для аутентификации (если настроен всего один – эта страница не будет показана);
* `github` – параметры провайдера Github (можно указывать только если `type: Github`:
    * `clientID` — ID организации на Github;
    * `clientSecret` — secret организации на Github;
    * `orgs` — массив названий организаций в Github;
        * `name` — название организации;
        * `teams` — массив команд, допустимых для приема из Github'а, токен пользователя будет содержать объединенное множество команд из Github'а и команд из этого списка (если множество окажется пустым, авторизация будет считаться не успешной), если параметр не указан, токен пользователя будет содержать все команды из Github'а;
    * `teamNameField` — данная опция отвечает за формат команд, которые будут получены из github. Может быть одним из трех вариантов: `name` (default), `slug`, `both`.
        * Если в организации `acme` есть группа `Site Reliability Engineers`, то в случае:
        * `name` будет получена группа с именем `['acme:Site Reliability Engineers']`;
        * `slug` будет получена группа с именем `['acme:site-reliability-engineers']`;
        * `both` будут получены группы с именами `['acme:Site Reliability Engineers', 'acme:site-reliability-engineers']`.
    * `useLoginAsID` — данная опция позволяет вместо использования внутреннего github id, использовать имя пользователя.
* `gitlab` – параметры провайдера Gitlab (можно указывать только если `type: Gitlab`:
    * `clientID` — ID приложения созданного в Gitlab (Application ID);
    * `clientSecret` — secret приложения созданного в Gitlab (Secret);
    * `baseURL` — адрес Gitlab'а (например: `https://fox.flant.com`);
    * `groups` — массив групп, допустимых для приема из Gitlab'а, токен пользователя будет содержать объединенное множество групп из Gitlab'а и групп из этого списка (если множество окажется пустым, авторизация будет считаться не успешной), если параметр не указан, токен пользователя будет содержать все группы из Gitlab'а;
        * Массив групп Gitlab содержит пути групп (path), а не их имена.
    * `useLoginAsID` — данная опция позволяет вместо использования внутреннего gitlab id, использовать имя пользователя.
* `crowd` – параметры провайдера Crowd (можно указывать только если `type: Crowd`:
    * `baseURL` – адрес Crowd'а (например: `https://crowd.example.com/crowd`);
    * `clientID` – ID приложения созданного в Crowd (Application Name);
    * `clientSecret` – пароль приложения созданного в Crowd (Password);
    * `groups` – массив групп, допустимых для приема из Crowd'а, токен пользователя будет содержать объединенное множество групп из Crowd'а и групп из этого списка (если множество окажется пустым, авторизация будет считаться не успешной), если параметр не указан, токен пользователя будет содержать все группы из Crowd'а;
    * `usernamePrompt` – строка, которая будет отображаться возле поля для имени пользователя в форме ввода логина и пароля.
        * По-умолчанию `Crowd username`.
    * `enableBasicAuth` – включает возможность basic авторизации для kubernetes api server, в качестве credentials для basic авторизации указываются логин и пароль пользователя из приложения, созданного в Crowd (возможно включить при указании только одного провайдера с типом Crowd), работает ТОЛЬКО при включенном `publishAPI`, полученные от Crowd данные авторизации и групп сохраняются в кэш на 10 секунд;
* `bitbucketCloud` – параметры провайдера Bitbucket Cloud (можно указывать только если `type: BitbucketCloud`);
    * `clientID` — ID приложения созданного в Bitbucket Cloud (Key);
    * `clientSecret` — secret приложения созданного в Bitbucket Cloud (Secret);
    * `teams` — массив комманд, допустимых для приема из Bitbucket Cloud'а, токен пользователя будет содержать объединенное множество комманд из Bitbucket Cloud'а и комманд из этого списка (если множество окажется пустым, авторизация будет считаться не успешной), если параметр не указан, токен пользователя будет содержать все комманды из  Bitbucket Cloud'а;
        * Токен будет содержать команды пользователя в claim'е `groups`, как и у других провайдеров.
    * `includeTeamGroups` — при включении данной опции в список команд будут включены все группы команды, в которых состоит пользователь.
        * По-умолчанию `false`.
        * Пример групп пользователя с включенной опцией:
          ```yaml
          groups=["my_team", "my_team/administrators", "my_team/members"]
          ```
* `oidc` — параметры провайдера OIDC (можно указывать только если `type: OIDC`):
    * `issuer` — адрес провайдера (пример: `https://accounts.google.com`);
    * `clientID` – ID приложения, созданного в OIDC провайдере;
    * `clientSecret` – пароль приложения, созданного в OIDC провайдере;
    * `basicAuthUnsupported` — включение этого параметра означает, что dex для общения с провайдером будет использовать POST запросы вместо добавления токена в Basic Authorization header (в большинстве случаев dex сам определяет, какой запрос ему нужно сделать, но иногда включение этого параметра может помочь);
        * По-умолчанию `false`.
    * `insecureSkipEmailVerified` — при включении данной опции dex перестает обращать внимание на информацию о том, подтвержден e-mail пользователя или нет (как именно подтверждается e-mail решает сам провайдер, в ответе от провайдера приходит лишь информация, подтвержден e-mail или нет);
        * По-умолчанию `false`.
    * `getUserInfo` — запрашивать ли дополнительные данные об успешно подключенном пользователе, подробнее о механизме можно прочитать [здесь](https://openid.net/specs/openid-connect-core-1_0.html#UserInfo));
        * По-умолчанию `false`.
    * `scopes` — список [полей](https://github.com/dexidp/website/blob/main/content/docs/custom-scopes-claims-clients.md) для включения в ответ при запросе токена;
        * По-умолчанию `["openid", "profile", "email", "groups", "offline_access"]`
    * `userIDKey` — [Claim](https://openid.net/specs/openid-connect-core-1_0.html#Claims), который будет использован для получения ID пользователя;
        * По-умолчанию `sub`.
    * `userNameKey` — [Claim](https://openid.net/specs/openid-connect-core-1_0.html#Claims), который будет использован для получения имени пользователя;
        * По-умолчанию `name`.
    * `promptType` — параметр указывает на то, должен ли issuer запрашивать подтверждение и давать подсказки при аутентификации. По умолчанию будет запрошено подтверждение при первой аутентификации. Допустимые значения могут изменяться в зависимости то issuer'а.
        * По-умолчанию `consent`.
* `ldap` – параметры провайдера LDAP:
    * `host` — адрес (и опционально порт) для LDAP-сервера;
    * `rootCAData` — CA, используемый для валидации TLS;
    * `insecureNoSSL` — при включении данной опции подключение к каталогу LDAP происходит по не защищенному порту;
        * По-умолчанию `false`.
    * `insecureSkipVerify` — при включении данной опции не происходит проверка подлинности ответа от провайдера с помощью `rootCAData`;
        * По-умолчанию `false`.
    * `bindDN` — путь до сервис-аккаунта приложения в LDAP.
        * Пример: `uid=seviceaccount,cn=users,dc=example,dc=com`
    * `bindPW` — пароль для сервис-аккаунта приложения в LDAP;
    * `startTLS` — использовать ли [STARTTLS](https://www.digitalocean.com/community/tutorials/how-to-encrypt-openldap-connections-using-starttls) для шифрования;
        * По-умолчанию `false`.
    * `usernamePrompt` – строка, которая будет отображаться возле поля для имени пользователя в форме ввода логина и пароля.
        * По-умолчанию `LDAP username`.
    * `userSearch` — настройки фильтров пользователей, которые помогают сначала отфильтровать директории, в которых будет производиться поиск пользователей, а затем найти пользователя по полям (его имени, адресу электронной почты или отображаемому имени), [подробнее о процессе фильтрации можно прочитать в документации](https://github.com/dexidp/dex/blob/3b7292a08fd2c61900f5e6c67f3aa2ee81827dea/Documentation/connectors/ldap.md#example-mapping-a-schema-to-a-search-config):
        * `baseDN` — откуда будет начат поиск пользователей.
            * Пример: `cn=users,dc=example,dc=com`
        * `filter` — опциональное поле, которое позволяет добавить фильтр для директории с пользователями.
            * Пример: `(objectClass=person)`
        * `username` — имя атрибута из которого будет получен username пользователя.
            * Пример: `uid`
            * Обязательный параметр
        * `idAttr` — имя атрибута из которого будет получен идентификатор пользователя.
            * Пример: `uid`
            * Обязательный параметр
        * `emailAttr` — имя атрибута из которого будет получен email пользователя.
            * Пример: `mail`
            * Обязательный параметр
        * `nameAttr` — атрибут отображаемого имени пользователя.
            * Пример: `name`
    * `groupSearch` — настройки фильтра для поиска групп для указанного пользователя, [подробнее о процессе фильтрации можно прочитать в документации](https://github.com/dexidp/dex/blob/3b7292a08fd2c61900f5e6c67f3aa2ee81827dea/Documentation/connectors/ldap.md#example-mapping-a-schema-to-a-search-config):
        * `baseDN` — откуда будет начат поиск групп.
            * Пример: `cn=groups,dc=freeipa,dc=example,dc=com`
        * `filter` — опциональное поле, которое позволяет добавить фильтр для директории с группами.
            * Пример: `(objectClass=group)`
        * `nameAttr` — имя атрибута, в котором хранится уникальное имя группы.
            * Пример: `name`
            * Обязательный параметр
        * `userMatchers` — список сопоставлений атрибута имени юзера с именем группы.
            * `userAttr` — имя атрибута, в котором хранится имя пользователя.
                * Пример: `uid`
            * `groupAttr` — имя атрибута, в котором хранятся имена пользователей, состоящих в группе.
                * Пример: `member`

### Примеры
#### Github

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: github
spec:
  type: Github
  displayName: My Company Github
  github:
    clientID: plainstring
    clientSecret: plainstring
```
#### GitLab
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: gitlab
spec:
  type: Gitlab
  displayName: Dedicated Gitlab
  gitlab:
    baseURL: https://gitlab.example.com
    clientID: plainstring
    clientSecret: plainstring
    groups:
    - administrators
    - users
```

#### Atlassian Crowd
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: crowd
spec:
  type: Crowd
  displayName: Crowd
  crowd:
    baseURL: https://crowd.example.com/crowd
    clientID: plainstring
    clientSecret: plainstring
    enableBasicAuth: true
    groups:
    - administrators
    - users
```

#### Bitbucket Cloud
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: gitlab
spec:
  type: BitbucketCloud
  displayName: Bitbucket
  bitbucketCloud:
    clientID: plainstring
    clientSecret: plainstring
    includeTeamGroups: true
    teams:
    - administrators
    - users
```
#### OIDC (OpenID Connect)
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: okta
spec:
  type: OIDC
  displayName: My Company Okta
  oidc:
    issuer: https://my-company.okta.com
    clientID: plainstring
    clientSecret: plainstring
    insecureSkipEmailVerified: true
    getUserInfo: true
```
#### LDAP
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: active-directory
spec:
  type: LDAP
  displayName: Active Directory
  ldap:
    host: ad.example.com:636
    insecureSkipVerify: true

    bindDN: cn=Administrator,cn=users,dc=example,dc=com
    bindPW: admin0!

    usernamePrompt: Email Address

    userSearch:
      baseDN: cn=Users,dc=example,dc=com
      filter: "(objectClass=person)"
      username: userPrincipalName
      idAttr: DN
      emailAttr: userPrincipalName
      nameAttr: cn

    groupSearch:
      baseDN: cn=Users,dc=example,dc=com
      filter: "(objectClass=group)"
      userMatchers:
      - userAttr: DN
        groupAttr: member
      nameAttr: cn
```

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