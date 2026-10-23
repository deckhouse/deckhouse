---
title: "Модуль user-authn: примеры конфигурации"
description: "Примеры настройки провайдеров аутентификации, DexAuthenticator, DexClient, локальных пользователей, парольной политики и 2FA в модуле user-authn."
---

## Пример конфигурации модуля

В примере добавляется способ прямого доступа к API-серверу для kubeconfig, который пользователи генерируют в веб-интерфейсе Deckhouse.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    kubeconfigGenerator:
    - id: direct
      masterURI: https://192.0.2.10:6443
      description: "Direct access to kubernetes API"
```

{% endraw %}

## Примеры настройки провайдера

В примерах ниже `<DEX_DOMAIN>` — домен Dex. Это значение [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate), в котором `%s` заменено на `dex`, например `dex.example.com`.

### Проверка подключения провайдера

На странице провайдера в веб-интерфейсе Deckhouse доступно действие «Проверить подключение». Оно создаёт ресурс [DexProviderCheck](cr.html#dexprovidercheck) и ждёт, пока контроллер `user-authn` запишет результат проверки в его `status`.

Проверка выполняется, чтобы убедиться в следующем:

- указанный [DexProvider](cr.html#dexprovider) существует и включён;
- эндпоинт Dex discovery внутри кластера доступен;
- эндпоинт внешнего провайдера доступен.

Для OIDC-провайдеров дополнительно читаются discovery-документ и JWKS. Для LDAP-провайдеров проверяется доступность TCP/TLS/StartTLS и, если включён Kerberos, наличие секрета с keytab. Также проверяется, что провайдер принимает учётные данные клиента или сервисной учётной записи (GitHub, GitLab, Bitbucket Cloud, Crowd, OIDC, bind в LDAP), и выводятся предупреждения об отключённой проверке TLS и о сертификатах, срок действия которых скоро истекает. Проверка не выполняет полный сценарий входа и не проверяет пароль тестового пользователя.

Чтобы выполнить проверку без веб-интерфейса, создайте DexProviderCheck:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProviderCheck
metadata:
  name: check-github
spec:
  providerName: github
```

Затем посмотрите его статус:

```shell
d8 k get dexproviderchecks check-github -o yaml
```

### Ограничение идентичностей провайдера

Пользователь, вошедший через DexProvider, получает email и группы, которые сообщает провайдер. Email служит именем пользователя в Kubernetes. Поэтому провайдер даёт своим пользователям все права, уже выданные в кластере этим email и группам.

Чтобы ограничить их, используйте [`spec.allowedIdentities`](cr.html#dexprovider-v1-spec-allowedidentities). Списки `emails` и `emailDomains` ограничивают email, и для любого другого email Dex отказывает во входе. Список `groups` ограничивает группы. Dex оставляет в токене только перечисленные группы и отказывает во входе, если не осталось ни одной.

Ограничения действуют для провайдера любого типа в дополнение к фильтрам групп этого типа (`oidc.allowedGroups`, `gitlab.groups`, `crowd.groups`, `bitbucketCloud.teams` без `includeTeamGroups`, `github.orgs[].teams` в каждой организации, `saml.allowedGroups` вместе с `filterGroups: true`). Имена групп сравниваются точно. Вложенность объектов Group список не расширяет, так как токен внешнего провайдера содержит только те группы, которые сообщил провайдер.

Dex проверяет ограничения при входе, при каждом запросе с refresh-токеном и при базовой аутентификации в API-сервере через LDAP-провайдер. Refresh-токен, который не проходит ограничения, Dex отзывает. Поэтому сужение ограничений завершает текущие сессии исключённых пользователей, и им нужно войти заново. Для провайдеров Crowd и OIDC basic-auth-proxy проверяет учётные данные напрямую у провайдера, поэтому при базовой аутентификации ограничения не действуют.

Те же ограничения определяют, кто может создать или изменить провайдер. Нужно иметь право назначать все роли, уже выданные email и группам, которые провайдер может сообщить. Провайдер без ограничения по email может сообщить email пользователя с ролью SuperAdmin, поэтому создать или изменить его может только SuperAdmin. Провайдер Crowd или OIDC с `enableBasicAuth: true` считается провайдером без ограничения по email, потому что базовая аутентификация через него выполняется в обход ограничений. Создать такой провайдер или включить `enableBasicAuth` в существующем может только SuperAdmin.

Пример провайдера, ограниченного доменом email и группой:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: contractors
spec:
  type: OIDC
  displayName: Contractors
  oidc:
    issuer: https://idp.contractor.example
    clientID: dex
    clientSecret: secret
  allowedIdentities:
    emailDomains: [contractor.example]
    groups: [contractors]
```

Без проверки выполняются: смена `clientSecret` или `bindPW`, изменение `displayName`, выключение провайдера, сужение ограничений, повторное применение того же манифеста и удаление провайдера. Любое другое изменение проверяется как подключение нового провайдера, в том числе включение выключенного провайдера.

Если проверка не пройдена, в ошибке перечислены роли, которые провайдер мог бы получить, например:

```text
dexproviders.deckhouse.io "corp": the provider can assert identities that already carry roles [user-authz:super-admin]; the requester's can-assign range is basic<=ClusterAdmin and does not cover them. Narrow spec.allowedIdentities (emails, emailDomains, groups) or ask a SuperAdmin
```

### GitHub

В примере представлены настройки провайдера для интеграции с GitHub.

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: github
spec:
  type: Github
  displayName: My Company GitHub
  # Опционально: временно отключить провайдер, не удаляя CR
  # enabled: false
  github:
    clientID: plainstring
    clientSecret: plainstring
```

В организации GitHub зарегистрируйте OAuth-приложение. Для этого перейдите в `Settings` → `Developer settings` → `OAuth Apps` → `New OAuth App` и укажите в `Authorization callback URL` адрес `https://<DEX_DOMAIN>/callback`.

Полученные `Client ID` и `Client Secret` укажите в [DexProvider](cr.html#dexprovider).

Если в организации ограничен доступ сторонних приложений, запросите одобрение в `Settings` → `Applications` → `Authorized OAuth Apps` → `<APPLICATION_NAME>` → `Send Request` и попросите владельца организации подтвердить запрос.

### GitLab

В примере представлены настройки провайдера для интеграции с GitLab.

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: gitlab
spec:
  type: Gitlab
  displayName: Dedicated GitLab
  gitlab:
    baseURL: https://gitlab.example.com
    clientID: plainstring
    clientSecret: plainstring
    groups:
    - administrators
    - users
```

В GitLab создайте новое приложение.

Для этого:

* **self-hosted** — перейдите в `Admin area` → `Application` → `New application`, укажите в `Redirect URI (Callback URL)` адрес `https://<DEX_DOMAIN>/callback` и в `Scopes` выберите `read_user` и `openid`;
* **cloud GitLab.com** — под главной учётной записью проекта перейдите в `User Settings` → `Application` → `New application`, укажите в `Redirect URI (Callback URL)` адрес `https://<DEX_DOMAIN>/callback` и в `Scopes` выберите `read_user` и `openid`;
* **GitLab 16 и выше** — при создании приложения включите опцию `Trusted`/`Trusted applications are automatically authorized on GitLab OAuth flow`.

Полученные `Application ID` и `Secret` укажите в [DexProvider](cr.html#dexprovider).

### Atlassian Crowd

В примере представлены настройки провайдера для интеграции с Atlassian Crowd.

```yaml
apiVersion: deckhouse.io/v1
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
    groups:
    - administrators
    - users
```

В соответствующем проекте Atlassian Crowd создайте новое `Generic`-приложение. Для этого перейдите в `Applications` → `Add application`.

Полученные `Application Name` и `Password` укажите в [DexProvider](cr.html#dexprovider).

Группы Crowd указывайте в DexProvider в нижнем регистре.

### Bitbucket Cloud

В примере представлены настройки провайдера для интеграции с Bitbucket.

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: bitbucket
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

В меню команды Bitbucket создайте новый OAuth consumer. Для этого перейдите в `Settings` → `OAuth consumers` → `New application`, укажите в `Callback URL` адрес `https://<DEX_DOMAIN>/callback` и разрешите доступ для `Account: Read` и `Workspace membership: Read`.

Полученные `Key` и `Secret` укажите в [DexProvider](cr.html#dexprovider).

### OIDC (OpenID Connect)

Аутентификация через OIDC-провайдера требует регистрации клиента (или создания приложения). Сделайте это по документации вашего провайдера (например, [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration) или [Blitz](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html)).

Полученные `clientID` и `clientSecret` укажите в [DexProvider](cr.html#dexprovider).

#### Keycloak

После выбора `realm` для настройки, добавления пользователя в [Users](https://www.keycloak.org/docs/latest/server_admin/index.html#assembly-managing-users_server_administration_guide) и создания клиента в разделе [Clients](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide) с включенной [аутентификацией](https://www.keycloak.org/docs/latest/server_admin/index.html#capability-config), которая необходима для генерации `clientSecret`, выполните следующие шаги:

* Создайте в разделе [Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes) `scope` с именем `groups`, и назначьте ему предопределённое сопоставление `groups` («Client scopes» → «Client scope details» → «Mappers» → «Add predefined mappers»).
* В созданном ранее клиенте добавьте данный `scope` [во вкладке Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) («Clients → «Client details» → «Client Scopes» → «Add client scope»).
* В полях «Valid redirect URIs», «Valid post logout redirect URIs» и «Web origins» [конфигурации клиента](https://www.keycloak.org/docs/latest/server_admin/#general-settings) укажите `https://<DEX_DOMAIN>/*`.

В примере представлены настройки провайдера для интеграции с Keycloak:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: keycloak
spec:
  type: OIDC
  displayName: My Company Keycloak
  oidc:
    issuer: https://keycloak.my-company.com/realms/myrealm # Используйте имя вашего realm
    clientID: plainstring
    clientSecret: plainstring
    insecureSkipEmailVerified: true
    getUserInfo: true
    scopes:
      - openid
      - profile
      - email
      - groups
```

Если в Keycloak не используется подтверждение учетных записей по email, для корректной работы с ним в качестве провайдера аутентификации внесите изменения в настройку [`Client scopes`](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) одним из следующих способов:

* Удалите сопоставление `Email verified` («Client Scopes» → «Email» → «Mappers»).
  Это необходимо для корректной обработки значения `true` в поле [`insecureSkipEmailVerified`](cr.html#dexprovider-v1-spec-oidc-insecureskipemailverified) и правильной выдачи прав пользователям с неподтвержденным email.

* Если отредактировать или удалить сопоставление `Email verified` невозможно, создайте отдельный Client Scope с именем `email_dkp` (или любым другим) и добавьте в него два сопоставления:
  * `email`: «Client Scopes» → `email_dkp` → «Add mapper» → «From predefined mappers» → `email`;
  * `email verified`: «Client Scopes» → `email_dkp` → «Add mapper» → «By configuration» → «Hardcoded claim». Укажите следующие поля:
    * «Name»: `email verified`;
    * «Token Claim Name»: `emailVerified`;
    * «Claim value»: `true`;
    * «Claim JSON Type»: `boolean`.
  
  После этого в клиенте, зарегистрированном для кластера Deckhouse Platform, в разделе «Clients» для `Client scopes` замените значение `email` на `email_dkp`.

  В ресурсе DexProvider укажите параметр `insecureSkipEmailVerified: true` и в поле `.spec.oidc.scopes` замените название Client Scope на `email_dkp`, следуя примеру:

  ```yaml
      scopes:
        - openid
        - profile
        - email_dkp
        - groups
  ```

{% alert level="warning" %}
При `insecureSkipEmailVerified: true` Dex принимает неподтверждённые email-адреса. Права выдаются по email, поэтому используйте этот параметр, только если провайдер не позволяет пользователям указывать произвольный email.
{% endalert %}

#### Okta

В примере представлены настройки провайдера для интеграции с Okta:

```yaml
apiVersion: deckhouse.io/v1
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
    getUserInfo: true
```

#### Blitz Identity Provider

При [регистрации приложения](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html) в Blitz Identity Provider укажите `https://<DEX_DOMAIN>/callback` в качестве URL для перенаправления пользователя после авторизации.

В примере представлены настройки провайдера для интеграции с Blitz Identity Provider:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: blitz
spec:
  displayName: Blitz Identity Provider
  oidc:
    basicAuthUnsupported: false
    claimMapping:
      email: email
      groups: your_claim # Claim для получения групп пользователя, группы пользователя настраиваются на стороне провайдера Blitz Identity Provider
    clientID: clientID
    clientSecret: clientSecret
    getUserInfo: true
    insecureSkipVerify: false
    issuer: https://yourdomain.idblitz.ru/blitz
    promptType: consent 
    scopes:
    - profile
    - openid
    userIDKey: sub
    userNameKey: email
  type: OIDC
```

Чтобы корректно отрабатывал выход из приложений (происходил отзыв токена и требовалась повторная авторизация), нужно установить `login` в значении параметра `promptType`.

Для обеспечения гранулированного доступа пользователя к приложениям необходимо:

* добавить параметр `allowedUserGroups` в `ModuleConfig` нужного приложения;
* добавить группы к пользователю (наименования групп должны совпадать как на стороне Blitz, так и на стороне Deckhouse).

Пример для Prometheus:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  settings:
    auth:
      allowedUserGroups:
        - adm-grafana-access
        - grafana-access
```

### LDAP

В примере представлены настройки провайдера для интеграции с Active Directory:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: active-directory
spec:
  type: LDAP
  displayName: Active Directory
  ldap:
    host: ad.example.com:636
    rootCAData: |
      -----BEGIN CERTIFICATE-----
      MIIFaDC...
      -----END CERTIFICATE-----

    bindDN: cn=dex-readonly,cn=users,dc=example,dc=com
    bindPW: <PASSWORD>

    usernamePrompt: Email Address

    enableBasicAuth: true

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

Для настройки аутентификации создайте в LDAP сервисную учётную запись с правами только на чтение. Укажите её DN и пароль в параметрах `bindDN` и `bindPW` объекта [DexProvider](cr.html#dexprovider). Пароль в `bindPW` указывается в открытом виде. Хешированные пароли не поддерживаются.

Если в LDAP разрешено анонимное чтение, `bindDN` и `bindPW` можно не указывать.

#### Настройка базовой аутентификации

Чтобы включить доступ к Kubernetes API с использованием базовой аутентификации (Basic Authentication) по учетным записям LDAP:

1. Убедитесь, что в конфигурации модуля `control-plane-manager` включен параметр [`apiserver.publishAPI`](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi).
1. Установите [`enableBasicAuth: true`](cr.html#dexprovider-v1-spec-ldap-enablebasicauth) в DexProvider типа LDAP.

{% alert level="info" %}
В кластере может быть только один провайдер аутентификации с включённым параметром [`enableBasicAuth`](cr.html#dexprovider-v1-spec-ldap-enablebasicauth).
{% endalert %}

После настройки пользователи смогут обращаться к Kubernetes API с помощью `kubectl`, используя свой логин и пароль в LDAP.

Пример kubeconfig для пользователя:

```yaml
apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: https://api.example.com
    # Путь к CA сертификату или insecure-skip-tls-verify: true
    certificate-authority: /path/to/ca.crt
users:
- name: ldap-user
  user:
    username: janedoe@example.com
    password: userpassword
contexts:
- name: default
  context:
    cluster: my-cluster
    user: ldap-user
current-context: default
```

#### Kerberos (SPNEGO) SSO для LDAP

Dex поддерживает аутентификацию без отображения формы ввода логина/пароля, которая реализуется с помощью механизма Kerberos (SPNEGO) для LDAP‑коннектора. При использовании этого механизма браузер, доверяющий хосту Dex, отправляет `Authorization: Negotiate …`, Dex валидирует Kerberos‑билет по keytab, пропускает форму вводу логина/пароля, сопоставляет principal с LDAP‑именем, получает группы и завершает OIDC‑поток.

Минимальный пример (расширение спецификации LDAP‑провайдера):

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: active-directory
spec:
  type: LDAP
  displayName: Active Directory
  ldap:
    host: ad.example.com:636
    bindDN: cn=dex-readonly,cn=users,dc=example,dc=com
    bindPW: <PASSWORD>
    userSearch:
      baseDN: cn=Users,dc=example,dc=com
      username: sAMAccountName
      idAttr: uid
      emailAttr: mail
      nameAttr: cn
    groupSearch:
      baseDN: cn=Users,dc=example,dc=com
      nameAttr: cn
      userMatchers:
      - userAttr: uid
        groupAttr: memberUid
    kerberos:
      enabled: true
      keytabSecretName: dex-kerberos-keytab   # Секрет в неймспейсе `d8-user-authn` с ключом 'krb5.keytab'.
      expectedRealm: EXAMPLE.COM              # Опционально, проверка realm (без учёта регистра).
      usernameFromPrincipal: SAMAccountName   # Localpart|SAMAccountName|UserPrincipalName
      fallbackToPassword: false               # По умолчанию false; если true — при отсутствии/ошибке заголовка `Authorization: Negotiate` будет показана форма ввода логина/пароля.
```

Примечания:

* Секрет `dex-kerberos-keytab` должен находиться в неймспейсе `d8-user-authn` и содержать ключ `krb5.keytab`.
* Один под Dex может обслуживать несколько LDAP+Kerberos провайдеров. У каждого — свой keytab. `krb5.conf` не требуется (Dex проверяет билеты офлайн по keytab).
* В DexProvider с `enabled: true` должен быть указан `keytabSecretName`, иначе объект не сохранится. Чтобы выключить Kerberos и оставить его настройки, укажите `enabled: false`.

### SAML

В примере представлены настройки провайдера для интеграции с SAML 2.0 Identity Provider (например, AD FS, Okta, Keycloak).

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: saml-provider
spec:
  type: SAML
  displayName: Корпоративный SAML
  saml:
    ssoURL: https://saml-idp.example.com/saml/sso
    rootCAData: |
      -----BEGIN CERTIFICATE-----
      MIIFaDC...
      -----END CERTIFICATE-----
    entityIssuer: https://dex.example.com/callback
    ssoIssuer: https://saml-idp.example.com
    usernameAttr: name
    emailAttr: email
    groupsAttr: groups
    nameIDPolicyFormat: persistent
```

Для настройки SAML Identity Provider:

1. Зарегистрируйте Dex как Service Provider (SP) у вашего провайдера идентификации со следующими параметрами:
   - **ACS URL (Assertion Consumer Service)**: `https://<DEX_DOMAIN>/callback`
   - **Entity ID**: `https://<DEX_DOMAIN>/callback`
   - **Формат идентификатора имени**: `persistent` или `emailAddress`

1. Настройте сопоставление атрибутов в провайдере идентификации для отправки атрибутов `email`, `name` (имя пользователя) и `groups` в SAML assertion.

1. Экспортируйте сертификат подписи провайдера идентификации и укажите его в поле `rootCAData` ресурса DexProvider.

{% alert level="info" %}
SAML не поддерживает refresh-токены. Dex кеширует данные пользователя из первого SAML-утверждения (assertion) и возвращает их при последующих запросах с refresh-токеном. Время жизни сессии ограничивается параметром модуля [`refreshTokenAbsoluteLifetime`](configuration.html#parameters-refreshtokenabsolutelifetime).
{% endalert %}

## Настройка OAuth2-клиента в Dex для подключения приложения

Этот вариант настройки подходит приложениям, которые могут выполнять OAuth2-аутентификацию самостоятельно, без OAuth2-прокси.
Чтобы такие приложения могли работать с Dex, используется [DexClient](cr.html#dexclient).

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
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

Списки разрешённых групп и адресов электронной почты в DexClient проверяются при входе и при каждом обновлении refresh-токена. Изменение списков запрещает дальнейшее обновление сессий, которые больше им не соответствуют; таким пользователям нужно войти заново.

После создания такого ресурса в Dex будет зарегистрирован клиент с идентификатором (**clientID**) `dex-client-myname@mynamespace`.

Пароль доступа к клиенту (**clientSecret**) сохранится в секрете:
{% raw %}

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dex-client-myname
  namespace: mynamespace
type: Opaque
data:
  clientSecret: c2VjcmV0
```

{% endraw %}

### Предоставление приложению доступа к Kubernetes API

Ресурс [DexClient](cr.html#dexclient) или [DexAuthenticator](cr.html#dexauthenticator) можно настроить для получения токенов, которые принимает API-сервер Kubernetes. Для этого установите на ресурсе одну из следующих аннотаций со значением `"true"`:

* `dexclient.deckhouse.io/allow-access-to-kubernetes` — для DexClient;
* `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` — для DexAuthenticator.

{% raw %}

Пример настройки ресурса DexClient:

```yaml
apiVersion: deckhouse.io/v1
kind: DexClient
metadata:
  name: myname
  namespace: mynamespace
  annotations:
    dexclient.deckhouse.io/allow-access-to-kubernetes: "true"
spec:
  redirectURIs:
  - https://app.example.com/callback
```

{% endraw %}

Аннотация регистрирует клиента в качестве доверенного (trusted peer) для привилегированного OAuth2-клиента `kubernetes`, что позволяет ему запрашивать ID-токены, предназначенные для API-сервера. В таком токене имя пользователя определяется значением claim `email`, а группы — значением claim `groups`. В результате приложение обращается к API-серверу от имени прошедшего аутентификацию пользователя и с предоставленными ему правами.

{% alert level="warning" %}
Предоставляемый таким образом доступ действует на уровне всего кластера, хотя DexClient и DexAuthenticator являются namespaced-объектами. По этой причине добавить аннотацию или изменить её значение на `"true"` может только субъект, которому разрешено действие `create` для субресурса `dexclients/allow-access-to-kubernetes` (для DexClient) или `dexauthenticators/allow-access-to-kubernetes` (для DexAuthenticator) в неймспейсе объекта. Это право входит в роль `d8:system-capability:user-authn:edit` и в уровень доступа `ClusterAdmin`. Чтобы выдать право в одном неймспейсе, создайте Role с действием `create` для этих субресурсов и привяжите её через RoleBinding в этом неймспейсе. Прав на создание объектов DexClient или DexAuthenticator недостаточно.

Добавление аннотации ограничено независимо от указанного значения, включая `"false"`. Это необходимо для совместимости с версиями Deckhouse Platform до 1.77, в которых доступ предоставляется при наличии аннотации независимо от её значения. Если доступ к API-серверу Kubernetes не требуется, не добавляйте аннотацию.

Ограничение не распространяется на ресурсы, у которых аннотация уже установлена. Пользователь с правами на изменение такого ресурса может продолжать изменять его, в том числе удалить аннотацию или изменить значение так, чтобы доступ был отключён.
{% endalert %}

## Браузерные сессии Dex

По умолчанию Dex не запоминает пользователя между входами: каждое приложение, отправляющее пользователя в Dex, получает
форму входа. При включённых браузерных сессиях Dex запоминает пользователя, вошедшего в браузере.
Чтобы включить сессии, задайте параметр [`sessions.enabled`](configuration.html#parameters-sessions-enabled):

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    sessions:
      enabled: true
```

{% endraw %}

Что меняется при включённых сессиях:

- Веб-интерфейсы Deckhouse используют общий вход: пользователь вводит пароль и второй фактор один раз и открывает
  остальные без них.
  Это приложения объектов DexAuthenticator в неймспейсах `d8-*`. Пользовательские объекты DexAuthenticator и DexClient
  общий вход не используют. Чтобы отключить общий вход, задайте
  [`sessions.sso`](configuration.html#parameters-sessions-sso) равным `None`.
  Общий вход действует, пока браузер использует любое из этих приложений, но не дольше
  [`sessions.absoluteLifetime`](configuration.html#parameters-sessions-absolutelifetime). Если в течение
  [`sessions.validIfNotUsedFor`](configuration.html#parameters-sessions-validifnotusedfor) они не использовались,
  следующее приложение снова запросит пароль.
  Пользователь внешнего провайдера использует общий вход, пока данные пользователя, сохранённые в Dex, актуальны: если
  в течение [`sessions.identityRefresh.maxAge`](configuration.html#parameters-sessions-identityrefresh-maxage) не было
  ни входа, ни обновления токенов приложения, следующее приложение снова запросит пароль.
- Форма входа показывает флажок «Запомнить меня». С ним сессия сохраняется после закрытия браузера.
- Когда пользователь выходит из приложения объекта DexAuthenticator, DexAuthenticator отправляет браузер в Dex, и Dex
  завершает сессию. За несколько секунд браузер выходит из всех приложений объектов DexAuthenticator, в которые вошёл
  через эту сессию. Если приложение и Dex находятся на разных сайтах (разных регистрируемых доменах), Dex просит
  пользователя подтвердить выход. Приложение выполняет выход переходом браузера: запрос из скрипта (`fetch`, XHR) не
  следует перенаправлению в Dex, и сессия Dex остаётся.
- Выход из Dex не завершает сессию во внешнем OIDC-провайдере: провайдер может снова впустить пользователя без пароля.
  Чтобы завершать сессию и у провайдера, включите параметр
  [`upstreamLogout`](cr.html#dexprovider-v1-spec-oidc-upstreamlogout) провайдера или задайте `promptType: login`, чтобы
  пароль запрашивался при каждом входе.
- Dex снова получает данные пользователя (например, группы) у провайдера, когда сохранённые данные старше
  [`sessions.identityRefresh.maxAge`](configuration.html#parameters-sessions-identityrefresh-maxage). Если провайдер
  отвечает, что пользователя больше нет или ему закрыт доступ, Dex завершает все сессии и токены этого пользователя
  этого провайдера.

Что не меняется:

- Токены для `kubectl` (kubeconfig, `d8 login`) выдаются только после интерактивного входа и никогда — по сессии браузера.
- Приложение объекта DexAuthenticator остаётся доступным в течение
  [`keepUsersLoggedInFor`](cr.html#dexauthenticator-v1-spec-keepusersloggedinfor), даже когда сессия Dex заканчивается по
  времени жизни или из-за неактивности. Завершает его только выход.

### Сколько длится сессия

Сессия Dex завершается при первом из событий:

- прошло [`sessions.absoluteLifetime`](configuration.html#parameters-sessions-absolutelifetime) с момента входа
  (по умолчанию неделя), как бы активно ни использовался браузер;
- сессию не использовали в течение
  [`sessions.validIfNotUsedFor`](configuration.html#parameters-sessions-validifnotusedfor) (по умолчанию сутки);
- пользователь вышел или администратор завершил сессию;
- браузер закрыт, если при входе пользователь не выбрал «Запомнить меня».

Браузер использует сессию, когда входит через неё в приложение и когда приложение, вошедшее через неё, обновляет свои
токены: приложение объекта DexAuthenticator или объекта DexClient с
[`refreshTokenLifetime: Session`](cr.html#dexclient-v1-spec-refreshtokenlifetime). Такое приложение обновляет токены
каждые [`idTokenTTL`](configuration.html#parameters-idtokenttl), пока оно открыто и отправляет запросы. Закрытое
приложение или приложение в браузере спящего компьютера сессию не использует.

Со значениями по умолчанию пользователь, работающий каждый день, вводит пароль и второй фактор раз в неделю и ещё раз
после перерыва дольше суток, например после выходных. Ночь сессия переживает.

Когда сессия завершается, уже открытые приложения остаются доступными в течение
[`keepUsersLoggedInFor`](cr.html#dexauthenticator-v1-spec-keepusersloggedinfor), но вход в другой веб-интерфейс снова
запрашивает пароль.

Чтобы запрашивать пароль чаще, уменьшите `sessions.validIfNotUsedFor`, например до `1h`: тогда пользователь снова
вводит пароль после каждого перерыва в час. `sessions.validIfNotUsedFor` не может быть больше
`sessions.absoluteLifetime`; если задан только `sessions.absoluteLifetime` и он меньше суток, неиспользуемая сессия
завершается через `sessions.absoluteLifetime`.

### Выход из приложения объекта DexClient

Приложение объекта DexClient выводит пользователя из Dex, отправляя браузер на адрес
`https://dex.<PUBLIC_DOMAIN>/logout` с параметрами `client_id` и `post_logout_redirect_uri`. Dex возвращает браузер только на
адреса из параметра [`postLogoutRedirectURIs`](cr.html#dexclient-v1-spec-postlogoutredirecturis) объекта DexClient:

```text
https://dex.example.com/logout?client_id=dex-client-myapp@mynamespace&post_logout_redirect_uri=https%3A%2F%2Fmyapp.example.com%2F
```

Чтобы узнавать о завершении сессии пользователя, приложение может задать
[`backchannelLogoutURI`](cr.html#dexclient-v1-spec-backchannellogouturi). Dex отправляет уведомление один раз, без
повторов, когда пользователь выходит из сессии, провайдер отвечает, что пользователя больше нет, сессию завершает
операция UserOperation или самообслуживание в веб-интерфейсе или пользователя удаляют; о сессии, завершённой вне Dex, —
в течение нескольких секунд. Приложение, которому важно не пропустить завершение, может также проверять свои токены на
эндпоинте интроспекции Dex (`/token/introspect`), который отвечает, что токены завершённой сессии или устройства
неактивны.

Приложение, запрашивающее офлайн-доступ (refresh-токены), получает их только после интерактивного входа. С параметром
[`refreshTokenLifetime: Session`](cr.html#dexclient-v1-spec-refreshtokenlifetime) приложение может впустить пользователя
по сессии браузера, а его токены перестают действовать, когда пользователь выходит из этой сессии.

## Локальная аутентификация

Локальная аутентификация обеспечивает проверку и управление доступом пользователей с возможностью настройки парольной политики, поддержкой двухфакторной аутентификации (2FA) и управлением группами.  
В параметрах модуля можно настроить сложность паролей, историю паролей, ротацию паролей, блокировку после неудачных попыток входа и ограничение частоты неудачных попыток с одного IP-адреса. Внешняя система аутентификации не требуется.

### Создание пользователя

Рекомендуемый способ создания локального пользователя — команда [`d8 iam user create`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-create). Она поддерживает интерактивный ввод пароля, автоматическую генерацию пароля, добавление в группы и TTL для временных пользователей.

Примеры:

Интерактивный ввод пароля (по умолчанию если stdin — терминал):

```shell
d8 iam user create anton --email anton@abc.com
```

Автогенерация пароля (сгенерированный пароль показывается в выводе команды один раз):

```shell
d8 iam user create anton --email anton@abc.com --generate-password
```

Пароль из stdin (для CI/CD пайплайнов):

```shell
echo "s3cret" | d8 iam user create anton --email anton@abc.com --password-stdin
```

Готовый bcrypt-хеш (например от htpasswd):

```shell
d8 iam user create anton --email anton@abc.com --password-hash '$2y$10$abcdef...'
```

Создать пользователя и добавить в группы (с автосозданием групп):

```shell
d8 iam user create anton --email anton@abc.com --generate-password --member-of admins --create-groups
```

Создать временного пользователя с TTL:

```shell
d8 iam user create anton --email anton@abc.com --generate-password --ttl 24h
```

Предпросмотр манифеста без применения:

```shell
d8 iam user create anton --email anton@abc.com --generate-password --dry-run -o yaml
```

В качестве альтернативы можно создать ресурс [User](cr.html#user) вручную. Придумайте пароль и укажите его хеш-сумму, закодированную в base64, в поле `password`. Email-адрес должен быть в нижнем регистре.

Для вычисления хеш-суммы пароля воспользуйтесь командой:

```shell
echo -n '3xAmpl3Pa$$wo#d' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
```

{% alert level="info" %}
Если команда `htpasswd` недоступна, установите соответствующий пакет:

* `apache2-utils` — для дистрибутивов, основанных на Debian;
* `httpd-tools` — для дистрибутивов, основанных на CentOS;
* `apache2-htpasswd` — для ALT Linux.
{% endalert %}

Обратите внимание, что в приведенном примере указан [`ttl`](cr.html#user-v1-spec-ttl).

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@yourcompany.com
  # echo -n '3xAmpl3Pa$$wo#d' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  password: 'JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H'
  ttl: 24h
```

{% endraw %}

[Правила авторизации](/modules/user-authz/cr.html#authorizationrule) предоставляют права пользователю на основе email из выданного токена. По этой причине нельзя создать ресурс User, у которого значение `spec.email` совпадает с субъектом типа `User` в существующем ресурсе [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) или [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule). Это предотвращает незаметное предоставление прав новому пользователю.

Если совпадение необходимо, например, если правило авторизации было создано заранее, укажите другой email или установите для пользователя аннотацию `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"`. Аннотация только подтверждает совпадение имени. Назначить грант по-прежнему можно, только если запрашивающий покрывает роли или они входят в его диапазон can-assign.

При сопоставлении email приводится к нижнему регистру, поскольку в таком виде он записывается в токен. Например, `Admin@Example.com` совпадает с субъектом `admin@example.com`.

Ограничение не распространяется на уже существующих пользователей, чей email совпадает с субъектами правил авторизации. Такие пользователи продолжают работать, а при обнаружении совпадения выводится предупреждение.

Учитывайте это ограничение при декларативном управлении пользователями и правилами авторизации. Если пользователь и соответствующее правило создаются одновременно, правило может быть создано раньше пользователя, в результате чего создание пользователя будет отклонено. Чтобы разрешить такое совпадение, добавьте аннотацию `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` в манифест User.

При удалении пользователя соответствующий субъект из правила авторизации не удаляется автоматически. Пока он остаётся в правиле, права продолжают предоставляться для указанного email, а система выводит предупреждение об отсутствии соответствующего пользователя. Если впоследствии будет создан пользователь с тем же email, он получит эти права. Если такое поведение не требуется, удалите соответствующий субъект из правила авторизации.

### Удаление пользователя

Для удаления локального пользователя используйте команду [`d8 iam user delete`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-delete). По умолчанию команда также удаляет пользователя из всех ресурсов [Group](cr.html#group), в которых он состоит.

Примеры:

Удалить пользователя (и автоматически удалить из всех групп):

```shell
d8 iam user delete anton
```

Удалить пользователя, оставив ссылки в группах:

```shell
d8 iam user delete anton --keep-memberships
```

При удалении пользователя, вручную или по истечении `ttl`, модуль завершает сессии пользователя так же, как операция `Logout` (подробнее — [в разделе «Жизненный цикл и побочные эффекты UserOperation»](#жизненный-цикл-и-побочные-эффекты-useroperation)), и удаляет его секрет 2FA (TOTP) и блокировку в Dex. Пользователь, созданный заново с тем же именем, не наследует сессии, секрет 2FA и блокировку.

### Операции над локальным пользователем

Операции сброса пароля, принудительной смены пароля, сброса 2FA, блокировки, разблокировки и выхода выполняются через ресурс [UserOperation](cr.html#useroperation). В поле `initiatorType` указывается, кто инициировал операцию: администратор (`admin`) или сам пользователь (`self`).

{% alert level="warning" %}
Не меняйте `User.spec.password` после создания User (например, с помощью `d8 k edit`). Это поле нельзя изменить, и оно не обновляет учётные данные в Dex. Сбрасывайте пароль через UserOperation или `d8 iam user reset-password`.
{% endalert %}

#### Административные операции

Для административных действий над локальными пользователями используйте команды `d8 iam user`. Они создают ресурс UserOperation с `initiatorType: admin`, дожидаются выполнения операции и выводят результат.

Удалить или пересоздать локального пользователя либо выполнить над ним UserOperation (`ResetPassword`, `RequirePasswordChange`, `Reset2FA`, `Lock`, `Unlock`, `Logout`) можно, только если вы можете назначить все роли, уже выданные email или группам пользователя (покрывающие права или явный диапазон can-assign). `initiatorType: self` эту проверку не обходит. То же правило действует для объектов DexProvider. Подробнее — [в разделе «Ограничение идентичностей провайдера»](#ограничение-идентичностей-провайдера).

После сброса пароля администратором операцией `ResetPassword` пользователь один раз входит с этим паролем. Затем Dex просит пользователя задать новый пароль, соответствующий парольной политике.

Пример интерактивного сброса пароля:

```shell
d8 iam user reset-password admin
```

Пример сброса пароля с чтением нового пароля из stdin:

```shell
echo "N3wPa$$wo#d" | d8 iam user reset-password admin --password-stdin
```

Пример сброса пароля с автоматической генерацией нового пароля:

```shell
d8 iam user reset-password admin --generate-password
```

Если пароль уже захеширован, передайте bcrypt-хеш без кодирования в Base64:

```shell
d8 iam user reset-password admin --password-hash '$2y$10$abcdef...'
```

Пример сброса 2FA:

```shell
d8 iam user reset2fa admin
```

Пример блокировки пользователя на 30 минут:

```shell
d8 iam user lock admin 30m
```

Команда `d8 iam user lock` блокирует локального пользователя и принимает длительность в секундах, минутах или часах, например `30s`, `10m` или `1h`. Чтобы заблокировать пользователя на несколько дней (`lock.for: 7d`) или бессрочно (`lock.for: permanent`), создайте UserOperation, как описано [в разделе «Ручное создание UserOperation»](#ручное-создание-useroperation).

Разблокировка пользователя:

```shell
d8 iam user unlock admin
```

По умолчанию команды ожидают завершения операции. Чтобы только создать UserOperation и вывести его имя, используйте флаг `--wait=false`.

#### Операции, которые выполняет сам пользователь

В веб-интерфейсе Deckhouse Platform пользователь может выйти на всех устройствах, а локальный пользователь — сменить пароль. Веб-интерфейс создаёт UserOperation с `initiatorType: self` для пользователя, который вошёл в систему:

- При выходе на всех устройствах создаётся операция `Logout`. Она завершает все сессии пользователя, включая файлы kubeconfig и сессию в веб-интерфейсе. Базовая аутентификация в API Kubernetes через LDAP-провайдер продолжает работать по паролю.
- При смене пароля создаётся операция `RequirePasswordChange`. При следующем входе Dex запрашивает текущий пароль и новый, соответствующий парольной политике. Операция не завершает сессии.

Смена пароля доступна только для локальных учётных записей (встроенный коннектор `Local`). Пользователи, которые входят через внешние провайдеры аутентификации, меняют пароль в соответствующей системе.

Пользователь также видит устройства, на которых выполнен вход в его учётную запись (поле [`status.sessions`](cr.html#useraccount-v1alpha1-status-sessions) его объекта UserAccount), и может выходить на них по одному:

- При выходе на одном устройстве создаётся операция `Logout` с полем [`logout.session`](cr.html#useroperation-v1-spec-logout-session) для браузера или [`logout.grant`](cr.html#useroperation-v1-spec-logout-grant) для другого устройства, например файла kubeconfig. Остальные устройства сохраняют свои сессии.
- При выходе на остальных устройствах создаётся операция `Logout` с полем [`logout.exceptSession`](cr.html#useroperation-v1-spec-logout-exceptsession) или [`logout.exceptGrant`](cr.html#useroperation-v1-spec-logout-exceptgrant). Сохраняется только устройство, с которого действует пользователь.

Выход на всех устройствах, выход на остальных устройствах или на другом устройстве и смена пароля доступны только на доверенном устройстве — том, на котором вход выполнен не меньше [`devices.trustedAfter`](configuration.html#parameters-devices-trustedafter) назад (по умолчанию 24 часа). С устройства, на котором вход выполнен позже, можно выйти только на нём самом. Поэтому тот, кто только что вошёл с украденным паролем, не может ни выкинуть владельца, ни сменить пароль, а владелец видит новое устройство в списке на доверенном устройстве и завершает на нём вход. Пользователь без доверенного устройства обращается к администратору: операции администратора не ограничены. С `devices.trustedAfter: 0s` каждое устройство доверенное сразу.

Возраст устройства считается от первого входа из его браузера, а не только от последнего. При входе Dex выдаёт браузеру долгоживущую cookie и запоминает, когда впервые увидел вход в учётную запись из этого браузера (поле [`knownSince`](cr.html#useraccount-v1alpha1-status-sessions-knownsince)). Браузер, из которого вышли и снова вошли, сохраняет свой возраст. Dex забывает браузер, и следующий вход из него считается новым, если:

- последний вход из браузера завершили с другого устройства или это сделал администратор. Пока в браузере есть другая действующая сессия или токен учётной записи, он остаётся знакомым;
- пользователь вышел на остальных устройствах: забываются все браузеры, кроме текущего, независимо от того, выполнен ли в них вход;
- пользователь вышел на всех устройствах либо администратор заблокировал пользователя, сбросил пароль, сбросил 2FA или завершил его сеансы;
- у учётной записи запомненных браузеров больше, чем [`devices.rememberedBrowsers`](configuration.html#parameters-devices-rememberedbrowsers) (по умолчанию 20): забывается браузер, который дольше всех не появлялся. С `0` Dex не помнит браузеры, и каждый вход — новое устройство.

У того, кто входит с украденным паролем, в браузере нет этой cookie, поэтому для него устройство новое. От украденной сессии это не защищает: возраст берётся у сессии или токена запроса, поэтому у скопированной cookie Dex или веб-интерфейса возраст устройства владельца.

Если пароль мог быть украден, действуйте с доверенного устройства в таком порядке:

1. Запросите смену пароля.
2. Сразу выйдите на этом устройстве и войдите снова: Dex попросит текущий и новый пароль. Браузер сохраняет свой возраст.
3. Только после этого выйдите на остальных устройствах. Текущий браузер остаётся знакомым и доверенным, а со всех остальных устройств, в том числе с устройства злоумышленника, выполняется выход.

Выполняйте шаги 1 и 2 сразу друг за другом. Пока новый пароль не задан, его просит любой вход, в том числе вход злоумышленника с украденным паролем, а возраст устройства на этой странице не проверяется.

Не начинайте с выхода на всех устройствах: после него текущий браузер тоже становится новым и в течение `devices.trustedAfter` не может ни запросить смену пароля, ни выкинуть устройство, которое снова вошло с украденным паролем.

Без 2FA для входа в учётную запись достаточно украденного пароля. Включите 2FA для локальных пользователей параметром [`staticUsers2FA`](configuration.html#parameters-staticusers2fa).

#### Ручное создание UserOperation

Когда CLI `d8 iam user` недоступен (например, в CI/CD, GitOps или скриптах автоматизации), ресурс [UserOperation](cr.html#useroperation) можно создать напрямую. Используйте `apiVersion: deckhouse.io/v1` и укажите `initiatorType: admin`.

Пример — сброс пароля локального пользователя (в `newPasswordHash` указывается bcrypt-хеш без кодирования в Base64; модуль кодирует его автоматически):

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: reset-password-admin
spec:
  user: admin
  type: ResetPassword
  initiatorType: admin
  resetPassword:
    newPasswordHash: "$2y$10$..."
```

Пример — блокировка локального пользователя на 1 час:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: lock-admin-1h
spec:
  user: admin
  type: Lock
  initiatorType: admin
  lock:
    for: "1h"
```

Пример — бессрочная блокировка:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: lock-admin-permanent
spec:
  user: admin
  type: Lock
  initiatorType: admin
  lock:
    for: "permanent"
```

Пример — разблокировка:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: unlock-admin
spec:
  user: admin
  type: Unlock
  initiatorType: admin
```

Пример — сброс 2FA:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: reset-2fa-admin
spec:
  user: admin
  type: Reset2FA
  initiatorType: admin
```

Пример — требование задать новый пароль при следующем входе без сброса текущего:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: require-password-change-admin
spec:
  user: admin
  type: RequirePasswordChange
  initiatorType: admin
```

Пример — завершение всех сессий пользователя во всех приложениях и файлах kubeconfig:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: logout-admin
spec:
  user: admin
  type: Logout
  initiatorType: admin
```

Пример — выход пользователя на одном устройстве с сохранением сессий на остальных. Возьмите `id` устройства из поля [`status.sessions`](cr.html#useraccount-v1alpha1-status-sessions) объекта UserAccount пользователя (подробнее — в разделе [Просмотр состояния учётной записи](#просмотр-состояния-учётной-записи)). Для устройства вида `Browser` укажите его в поле [`logout.session`](cr.html#useroperation-v1-spec-logout-session), для устройства вида `Grant`, например файла kubeconfig, — в поле [`logout.grant`](cr.html#useroperation-v1-spec-logout-grant):

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: logout-admin-kubeconfig
spec:
  user: admin
  type: Logout
  initiatorType: admin
  logout:
    grant: qx3w2ttiyq5ovzvbuehdcbbse
```

#### Операции над внешними пользователями

Для пользователей внешних провайдеров любого типа (LDAP, Atlassian Crowd, OIDC, SAML, GitHub, GitLab, Bitbucket Cloud) вместо `spec.user` используется поле [`spec.target`](cr.html#useroperation-v1-spec-target). Для внешних пользователей поддерживаются операции `Lock`, `Unlock` и `Logout`.

Операция находит учётную запись пользователя по email, который провайдер сообщил при последнем входе пользователя. Поля `connectorID` и `userID` сужают поиск до одного провайдера и одного идентификатора пользователя. Блокировка действует на одну пару из провайдера и пользователя. Она блокирует только найденную учётную запись и не блокирует локального пользователя или пользователя другого провайдера с тем же email. Локальный пользователь указывается только в `spec.user`. Dex проверяет блокировку при каждом входе через провайдер, при каждом запросе с refresh-токеном, при обмене токена и при базовой аутентификации в API-сервере через LDAP-провайдер. Для провайдеров Crowd и OIDC basic-auth-proxy проверяет учётные данные напрямую у провайдера, поэтому при базовой аутентификации блокировка не действует.

Операция завершается ошибкой, если ни у одной учётной записи нет этого email (пользователь ещё ни разу не входил) или если подходит несколько учётных записей. В этом случае укажите `connectorID`, а если у провайдера несколько пользователей с этим email, то и `userID`.

Блокировка действует на идентификатор пользователя, который провайдер возвращает при входе, поэтому этот идентификатор должен оставаться одним и тем же при каждом входе пользователя. Для SAML-провайдера укажите в параметре [`nameIDPolicyFormat`](cr.html#dexprovider-v1-spec-saml-nameidpolicyformat) значение `persistent` или `emailAddress` либо не задавайте параметр (тогда Dex запрашивает `persistent`). При значении `transient` провайдер возвращает новый идентификатор при каждом входе. Тогда блокировка не действует при следующем входе пользователя, а после второго входа пользователя операция завершается ошибкой, так как этот email есть у нескольких пользователей провайдера.

Пример — блокировка внешнего пользователя по `connectorID` + email на 30 минут:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: lock-external-user
spec:
  target:
    connectorID: my-ldap
    email: jane.doe@example.org
  type: Lock
  initiatorType: admin
  lock:
    for: "30m"
```

#### Жизненный цикл и побочные эффекты UserOperation

Использование объекта UserOperation имеет следующие особенности:

- UserOperation — **одноразовый** объект. После создания модуль обрабатывает его и записывает результат в `status.phase` (`Succeeded` или `Failed`).
- Операции **автоматически удаляются** через [`retention.userOperations`](configuration.html#parameters-retention-useroperations) (по умолчанию 24 часа).
- UserOperation выполняется один раз. Изменения спецификации после создания игнорируются.
- Для нового действия нужно создать новый UserOperation.

{% alert level="warning" %}
Операции `Lock`, `ResetPassword`, `Reset2FA` и `Logout` сразу завершают сессии пользователя:

- Dex отклоняет все refresh-токены, коды авторизации и сессии браузера, которые он выдал пользователю до операции. Приложения и файлы kubeconfig просят пользователя войти заново.
- DexAuthenticator перестаёт принимать сессии пользователя в течение нескольких секунд.
- API Kubernetes отклоняет ID-токены, выданные пользователю до операции, в течение нескольких секунд, если в модуле `user-authz` включён параметр [`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) и в кластере работает Kubernetes 1.34 или новее. В остальных случаях эти ID-токены действуют в API Kubernetes до истечения их срока (параметр [`idTokenTTL`](configuration.html#parameters-idtokenttl)).
- Базовая аутентификация в API Kubernetes через LDAP-провайдер заново проверяет пароль в Dex. После `Logout` она продолжает работать по паролю.

`Unlock` и `RequirePasswordChange` не завершают сессии. Секрет 2FA пользователя удаляет только `Reset2FA`. Операция `Lock` запрещает вход, запросы с refresh-токеном и обмен токена до окончания блокировки. Что отозвала операция, показывает её поле `status.revocation`.
{% endalert %}

#### Проверка статуса операции

Для проверки статуса операции выполните следующие действия:

1. Получите список всех операций:

   ```shell
   d8 k get useroperations
   ```

1. Получите полный статус операции:

   ```shell
   d8 k get useroperation <имя> -o yaml
   ```

1. Получите только статус завершения:

   ```shell
   d8 k get useroperation <имя> -o jsonpath='{.status.phase}'
   ```

#### Просмотр состояния учётной записи

Для каждого локального пользователя и каждого пользователя, входившего через внешний провайдер, модуль ведёт объект [UserAccount](cr.html#useraccount). В нём видно, когда пользователь входил последний раз, настроил ли он 2FA, заблокирована ли учётная запись, до какого времени и сколько неудачных попыток входа учтено. Не создавайте и не изменяйте объекты UserAccount.

Чтобы посмотреть учётные записи, выполните:

```shell
d8 k get useraccounts
```

В поле [`status.sessions`](cr.html#useraccount-v1alpha1-status-sessions) перечислены устройства, на которых выполнен вход в учётную запись. Первыми идут устройства с самой свежей активностью: браузеры, выполнившие вход в Dex (вид `Browser`), и устройства с refresh-токеном, не привязанным к действующей сессии Dex в браузере, например файлы kubeconfig (вид `Grant`). Для каждого устройства видны приложения, адрес клиента и User-Agent при последнем входе, время входа и час последней активности. Адрес клиента верный, только если его видит Ingress-контроллер (подробнее — в разделе [Какой адрес клиента видит Dex?](faq.html#какой-адрес-клиента-видит-dex)). Чтобы посмотреть устройства учётной записи, выполните:

```shell
d8 k get useraccount <ИМЯ> -o jsonpath='{range .status.sessions[*]}{.kind}{"\t"}{.id}{"\t"}{.clients}{"\t"}{.ipAddress}{"\t"}{.lastActivity}{"\n"}{end}'
```

Пока [браузерные сессии Dex](#браузерные-сессии-dex) выключены, каждый вход в приложение DexAuthenticator — отдельное устройство.

Dex хранит не больше [`devices.maxPerApplication`](configuration.html#parameters-devices-maxperapplication) (по умолчанию 10) refresh-токенов пользователя для одного приложения. Вход сверх лимита завершает вход в это приложение на устройстве, которое дольше всех им не пользовалось; доверенное устройство (параметр [`devices.trustedAfter`](configuration.html#parameters-devices-trustedafter)) завершается, только когда недоверенных не осталось. Refresh-токен заменяется, а не добавляется, в двух случаях: приложение объекта DexAuthenticator снова входит в браузере с сессией Dex и базовая аутентификация в API Kubernetes снова выполняет вход. Любой другой вход — новое устройство: например, каждый новый файл kubeconfig — отдельное устройство, а предыдущие продолжают работать.

#### Автоматическая блокировка

Если число неудачных попыток входа превышает [`passwordPolicy.lockout.maxAttempts`](configuration.html#parameters-passwordpolicy-lockout-maxattempts), Dex блокирует пользователя на время [`lockout.lockDuration`](configuration.html#parameters-passwordpolicy-lockout-lockduration), после чего снимает блокировку автоматически. Для пользователей LDAP и Crowd это работает, только если тип провайдера указан в [`lockout.applyToConnectors`](configuration.html#parameters-passwordpolicy-lockout-applytoconnectors). UserOperation для такой блокировки Dex не создаёт. Чтобы разблокировать пользователя раньше, выполните `d8 iam user unlock` или создайте UserOperation с типом `Unlock`.

Пока локальный пользователь или пользователь LDAP или Crowd заблокирован, Dex отказывает во входе по паролю независимо от того, верен ли пароль: после автоматической блокировки — с кодом `429` и страницей «Account temporarily locked», при блокировке администратором — с кодом `403` и страницей «Account locked by an administrator». Страница смены пароля отказывает заблокированному пользователю так же. Автоматическая блокировка не сокращает блокировку, которая заканчивается позже, в том числе блокировку администратора.

### Добавление пользователя в группу

Пользователи могут быть объединены в группы для управления правами доступа. Рекомендуемый способ управления группами — команда [`d8 iam group`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-group).

Примеры:

Создать группу:

```shell
d8 iam group create admins
```

Добавить пользователя в группу:

```shell
d8 iam group add-member admins user anton
```

Добавить вложенную группу:

```shell
d8 iam group add-member admins group devops
```

Удалить пользователя из группы:

```shell
d8 iam group remove-member admins user anton
```

Удалить группу:

```shell
d8 iam group delete admins
```

В качестве альтернативы можно создать ресурс [Group](cr.html#group) вручную. Пример манифеста ресурса Group для группы:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: admins
spec:
  name: admins
  members:
    - kind: User
      name: admin
```

{% endraw %}

Здесь `members` — список членов: `kind: User` с `name` = `User.metadata.name`, либо вложенная `kind: Group` с `name` = `Group.spec.name` (имя в токене, не `metadata.name`).

Вложенные группы не должны образовывать цикл. Изменение ресурса Group отклоняется, если после него группа становится членом самой себя напрямую или через другие группы. Если цикл в иерархии уже есть, изменения, которые не создают нового цикла, принимаются с предупреждением. В предупреждении указан путь цикла, в котором каждая группа является членом предыдущей. Чтобы разорвать цикл, удалите одну из этих групп из списка членов предыдущей группы.

Имя группы записывается в выданный токен без изменений. При этом оно неотличимо от имени группы, полученного от внешнего провайдера аутентификации. Поэтому нельзя создать ресурс Group, если значение `spec.name` совпадает с субъектом типа `Group` в существующем ресурсе [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) или [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule). Это предотвращает незаметное предоставление прав участникам новой группы.

Если совпадение необходимо, переименуйте группу или установите на ней аннотацию `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"`.

В отличие от email, имя группы при сопоставлении не приводится к нижнему регистру. По этой причине имена, различающиеся только регистром букв, считаются разными.

Ограничение не распространяется на уже существующие группы, имена которых совпадают с субъектами правил авторизации. Такие группы продолжают работать, а при обнаружении совпадения выводится предупреждение.

Учитывайте это ограничение при декларативном управлении группами и правилами авторизации. Если группа и соответствующее правило создаются одновременно, правило может быть создано раньше группы, в результате чего создание группы будет отклонено. Чтобы разрешить такое совпадение, добавьте аннотацию `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` в манифест Group.

При удалении группы соответствующий субъект из правила авторизации не удаляется автоматически. Пока он остаётся в правиле, права продолжают предоставляться группе с указанным именем. Группа, впоследствии созданная с тем же именем, получит эти права. Если такое поведение не требуется, удалите соответствующий субъект из правила авторизации.

### Просмотр пользователей и групп

Для просмотра пользователей, групп и их прав (группы, выданные права, уровень доступа) используйте команды [`d8 iam get`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-get) и [`d8 iam list`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-list).

Примеры:

Список всех пользователей с их правами:

```shell
d8 iam list users
```

Детали для конкретного пользователя (группы, выданные права, уровень доступа):

```shell
d8 iam get user anton
```

Список всех групп:

```shell
d8 iam list groups
```

Детали для конкретной группы (участники, выданные права):

```shell
d8 iam get group admins
```

### Парольная политика

Настройки парольной политики позволяют контролировать сложность пароля, ротацию и блокировку пользователей:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    passwordPolicy:
      complexityLevel: Fair
      passwordHistoryLimit: 10
      lockout:
        lockDuration: 15m
        maxAttempts: 3
      rotation:
        interval: "30d"
```

{% endraw %}

Описание полей:

* `complexityLevel` — уровень сложности пароля: `None`, `Low`, `Fair`, `Good`, `Excellent` или `Custom`;
* `custom` — пользовательские правила сложности (используются только при `complexityLevel: Custom`):
  * `custom.minLength` — минимальное количество символов в пароле;
  * `custom.specialCharacters` — если `true`, требуется хотя бы один специальный символ;
  * `custom.numbers` — если `true`, требуется хотя бы одна цифра;
  * `custom.capitalized` — если `true`, требуется хотя бы одна заглавная буква;
  * `custom.repeatedChars` — если `true`, запрещается более 2 одинаковых символов подряд;
* `passwordHistoryLimit` — число предыдущих паролей, которые хранит система, чтобы предотвратить их повторное использование;
* `lockout` — настройки блокировки при превышении лимита неудачных попыток входа:
  * `lockout.maxAttempts` — лимит неудачных попыток;
  * `lockout.lockDuration` — длительность блокировки пользователя;
  * `lockout.applyToConnectors` — типы внешних провайдеров, на пользователей которых также распространяется блокировка: `LDAP`, `Crowd`. На локальных пользователей блокировка распространяется всегда;
* `rotation` — настройки ротации паролей:
  * `rotation.interval` — период обязательной смены пароля.

Чтобы ограничить число неудачных попыток входа с одного IP-адреса, используйте параметр [`rateLimit`](configuration.html#parameters-ratelimit).

Если локальный пользователь входит с паролем, который нужно сменить, Dex вместо приложения открывает страницу смены пароля. Пароль нужно сменить после сброса администратором операцией `ResetPassword`, если он не соответствует правилам сложности и если он старше `rotation.interval`. Страница запрашивает только новый пароль, дважды, потому что форма входа только что проверила текущий. При включённой 2FA Dex сначала запрашивает код и открывает страницу смены пароля только после него, поэтому тот, кто знает только пароль, сменить его не сможет. После смены пароля вход продолжается обычным образом и открывает приложение.

Страница смены пароля открывается только в рамках входа, который на неё привёл, и в том же браузере. Если открыть её в другом браузере или после истечения времени входа, Dex её не откроет, и пользователь входит заново.

Пример с пользовательскими правилами сложности:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    passwordPolicy:
      complexityLevel: Custom
      custom:
        minLength: 10
        specialCharacters: true
        numbers: false
        capitalized: true
        repeatedChars: false
      passwordHistoryLimit: 10
```

{% endraw %}

### Двухфакторная аутентификация (2FA)

2FA позволяет повысить уровень безопасности, требуя ввести код из приложения-аутентификатора TOTP (например, Google Authenticator) при входе.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    staticUsers2FA:
      enabled: true
      issuerName: "awesome-app"
```

{% endraw %}

Описание полей:

* `enabled` — включает или отключает 2FA для всех статических пользователей;
* `issuerName` — имя, которое будет отображаться в приложении-аутентификаторе при добавлении аккаунта.

{% alert level="info" %}
После включения 2FA каждый пользователь должен пройти процесс регистрации в приложении-аутентификаторе при первом входе.
{% endalert %}

Dex запрашивает код у локального пользователя при каждом входе. Пользователи внешних провайдеров входят без кода.

Чтобы пользователь заново зарегистрировал приложение-аутентификатор, например после потери устройства, сбросьте его 2FA командой `d8 iam user reset2fa <USERNAME>` или с помощью UserOperation с типом `Reset2FA`. Остальные операции, включая `Lock` и `ResetPassword`, секрет 2FA сохраняют.

### Выдача прав пользователю или группе

Чтобы выдать права пользователю или группе, привяжите одну из ролей модуля `user-authz` через RoleBinding или ClusterRoleBinding, указав в качестве субъекта email пользователя или имя группы. Подробнее — [в описании гранулярной ролевой модели](/modules/user-authz/#гранулярная-ролевая-модель).
