---
title: "Аутентификация"
permalink: /admin/authentication.html
---

## Введение

В DKP реализована сквозная аутентификация, которая охватывает все интерфейсы взаимодействия, реализованные в рамках DKP, а также пользовательские приложения.
В основе механизма аутентификации DKP — федеративный OpenID Connect провайдер `Dex`, позволяющий подключать внешних поставщиков аутентификации (GitHub, GitLab, Bitbucket Cloud, Crowd, LDAP, OIDC), а также работать со статическими пользователями и группами.

DKP позволяет:

- Включать аутентификацию в любом веб-приложении;
- Организовать защищенный доступ к Kubernetes API;
- Управлять пользователями (внешние и статические) и группами;
- Использовать локальную аутентификацию и интегрироваться со следующими внешними системами аутентификации:
  - [GitHub](todo);
  - [GitLab](todo);
  - [Bitbucket Cloud](todo);
  - [Crowd](todo);
  - [LDAP](todo);
  - [OIDC](todo);
- Генерировать конфигурации для kubectl и других утилит Kubernetes — после авторизации в веб-интерфейсе пользователю предоставляется набор команд, которые можно скопировать и вставить в консоль для настройки kubectl;
- Настроить несколько адресов `kube-apiserver` и сертификаты ЦС (CA) для каждого из них — например, если доступ к кластеру Kubernetes осуществляется через VPN или прямое подключение.

## Интеграция

### Включение аутентификации в веб-приложении

Чтобы задействовать единую систему аутентификации для собственного веб-приложения в кластере Kubernetes, необходимо выполнить несколько основных шагов:

1. Убедитесь, что модуль user-authn включён:

Пример включения модуля:

- с помощью ресурса ModuleConfig:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: user-authn
  spec:
    enabled: true
  ```

- с помощью команды `deckhouse-controller` (требуется kubectl, настроенный на работу с кластером):

  ```console
  kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable user-authn
  ```

1. Убедитесь, что все необходимые параметры модуля user-authn корректно заданы — при необходимости настройте publishAPI для генерации kubeconfig, укажите нужные дополнительные настройки.

1. Включите ручное управление режимом отказоустойчивости (HA).
Чтобы включить HA-режим, в ModuleConfig необходимо настроить соответствующий параметр — [`highAvailability.enabled: true`](todo).

1. Настройте провайдеров аутентификации или статических пользователей и группы.

1. Создайте ресурс DexAuthenticator в пространстве имен приложения и добавьте несколько аннотаций к ресурсу Ingress. Это позволит:
   - ограничить список групп, которым разрешен доступ;
   - ограничить список адресов, с которых разрешена аутентификация;
   - интегрировать приложение в единую систему аутентификации, если приложение поддерживает OIDC. Для этого в Kubernetes создается ресурс DexClient в пространстве имён приложения. В том же пространстве имён создается секрет с данными для подключения в Dex по OIDC.

### Поддерживаемые провайдеры

DKP поддерживает подключение следующих внешних провайдеров и протоколов аутентификации:

- [GitHub](todo);
- [GitLab](todo);
- [BitBucket Cloud](todo);
- [Atlassian Crowd](todo);
- [LDAP (например, Active Directory)](todo);
- [OIDC (например, Okta, Keycloak, Gluu, Blitz Identity Provider)](todo).

Можно подключить более одного провайдера одновременно. Для каждого провайдера создаётся свой ресурс DexProvider.

### Общая схема интеграции

1. Создайте OAuth-приложение у провайдера аутентификации:
   - укажите Redirect URI вида https://dex.<modules.publicDomainTemplate>/callback;
   - получите `clientID` и `clientSecret`.
1. Создайте ресурс DexProvider с учётом специфики выбранного провайдера.
1. Включите и настройте модуль user-authn (если он не активен по умолчанию):
   - создайте ModuleConfig с именем user-authn;
   - укажите необходимые параметры в секции `spec.settings`.

Пример конфигурации user-authn:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    kubeconfigGenerator:
    - id: direct
      masterURI: https://159.89.5.247:6443
      description: "Direct access to kubernetes API"
    publishAPI:
      enabled: true
```

### Настройки провайдера

1. **GitHub**.

В организации GitHub необходимо создать новое приложение.

Для этого выполните следующие шаги:

- перейдите в «Settings» → «Developer settings» → «OAuth Aps» → «Register a new OAuth application», и в качестве «Authorization callback URL» укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`;
- полученные `Client ID` и `Client Secret` укажите в ресурсе DexProvider.

Если организация GitHub находится под управлением клиента:

- перейдите в «Settings» → «Applications» → «Authorized OAuth Apps» → <name of created OAuth App> и нажмите «Send Request» для подтверждения;
- попросите клиента подтвердить запрос, который придет к нему на email.

Пример настройки провайдера для интеграции с GitHub:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: github
spec:
  type: Github
  displayName: My Company GitHub
  github:
    clientID: plainstring
    clientSecret: plainstring
```

1. **GitLab**.

В GitLab проекта необходимо создать новое приложение.

Для этого выполните следующие шаги:

- self-hosted: перейдите в «Admin area» → «Application» → «New application» и в качестве «Redirect URI (Callback url)» укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`, а также выберите «scopes»: `read_user`, `openid`;
- cloud gitlab.com: под главной учетной записью проекта перейдите в «User Settings» → «Application» → «New application» и в качестве «Redirect URI (Callback url)» укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`, а также выберите «scopes»: `read_user`, `openid`;
- полученные `Application ID` и секрет укажите в ресурсе DexProvider.

{% alert level="info" %}
Для GitLab версии 16 и выше включите опцию «Trusted/Trusted applications are automatically authorized on Gitlab OAuth flow» при создании приложения.
{% endalert %}

Пример настройки провайдера для интеграции с GitLab:

```yaml
apiVersion: deckhouse.io/v1
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

1. **Atlassian Crowd**.

В соответствующем проекте Atlassian Crowd необходимо создать новое Generic-приложение.

Для этого выполните следующие шаги:

- перейдите в «Applications» → «Add application»;
- полученные «Application Name» и «Password» укажите в ресурсе DexProvider;
- группы `CROWD` укажите в lowercase-формате для ресурса DexProvider.

Пример настройки провайдера для интеграции с Atlassian Crowd:

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
    enableBasicAuth: true
    groups:
    - administrators
    - users
```

1. **Bitbucket Cloud**.

Для настройки аутентификации необходимо в меню команды Bitbucket создать нового OAuth пользователя (consumer).

Для этого выполните следующие шаги:

- перейдите в «Settings» → «OAuth consumers» → «New application» и в качестве «Callback URL» укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`;
- разрешите доступ для «Account»: `Read и Workspace membership — Read`.
- полученные `Key` и секрет укажите в ресурсе DexProvider.

Пример настройки провайдера для интеграции с Bitbucket:

```yaml
apiVersion: deckhouse.io/v1
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

1. **OIDC (OpenID Connect)**.

Аутентификация через OIDC-провайдера требует регистрации клиента (или создания приложения). Сделайте это по документации вашего провайдера (например, [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration) или [Blitz](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html)).

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` укажите в ресурсе DexProvider.

{% alert level="info" %}
При регистрации приложения в Blitz Identity Provider требуется указать адрес перенаправления пользователя после авторизации. Если используется DexProvider, в качестве этого адреса следует указать `https://dex.<publicDomainTemplate>/`, где `publicDomainTemplate` – указанный в модуле global шаблон DNS-имен кластера.
{% endalert %}

{% alert level="info" %}
Чтобы при выходе из приложения корректно отзывался токен и требовалась повторная авторизация, в параметре promptType необходимо указать значение login.
{% endalert %}

Чтобы настроить детальный (гранулированный) доступ пользователей к приложениям:

- добавьте параметр `allowedUserGroups` в ModuleConfig нужного приложения;
- добавьте соответствующие группы пользователю, используя те же наименования групп как на стороне провайдера, так и на стороне Deckhouse.

Пример настройки провайдера для интеграции с Blitz Identity Provider:

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
    insecureSkipEmailVerified: true # Установить true, если нет необходимости в проверке email пользователя
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

Пример настройки провайдера для интеграции с Okta:

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
    insecureSkipEmailVerified: true
    getUserInfo: true
```

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

1. **LDAP**.

Для настройки аутентификации заведите в LDAP read-only-пользователя (service account).

Полученные путь до пользователя и пароль укажите в параметрах `bindDN` и `bindPW` ресурса DexProvider.

{% alert level="info" %}
Если в LDAP настроен анонимный доступ на чтение, настройки можно не указывать.

В параметре `bindPW` укажите пароль в plain-виде. Стратегии с передачей хэшированных паролей не предусмотрены.
{% endalert %}

Пример настройки провайдера для интеграции с Active Directory:

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

## Локальная аутентификация

Помимо внешних провайдеров, DKP позволяет создавать статические учётные записи и управлять ими через ресурсы User и Group.

1. Создание статического пользователя.

Для создания статического пользователя создайте ресурс User.

Пример создания ресурса (обратите внимание, что в приведенном примере указан [ttl](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authn/cr.html#user-v1-spec-ttl)):

```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@yourcompany.com
  password: $2a$10$etblbZ9yfZaKgbvysf1qguW3WULdMnxwWFrkoKpRH1yeWa5etjjAa
  ttl: 24h
```

Придумайте пароль и укажите его хэш-сумму в поле password. Пароль хранится в зашифрованном виде (bcrypt).
Хэш-сумму можно сгенерировать с помощью команды:

```console
echo "$password" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
```

1. Добавление пользователя в группу.

Чтобы объединять статических пользователей в группы, создайте ресурс Group.

Пример создания ресурса:

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

`Members` — список пользователей, которые входят в группу (указывается `kind`: User и имя пользователя).

1. Выдача прав пользователю или группе.

Права в кластере назначаются через ресурс [ClusterAuthorizationRule](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authz/cr.html#clusterauthorizationrule). Например:

```console
accessLevel: PrivilegedUser
```

## Доступ к Kubernetes API

## Важно

- Использовать OpenID Connect без HTTPS небезопасно (это подтверждается, например, отсутствием поддержки OIDC по HTTP в Kubernetes API-сервере), поэтому данный механизм можно установить только при включённом HTTPS (параметр `https.mode` должен быть отличен от `Disabled`  либо глобально для кластера, либо в самом механизме).

- После активации механизма, аутентификация во всех веб-интерфейсах переключается с HTTP Basic Auth на Dex, который в свою очередь использует ваши внешние провайдеры. Чтобы настроить доступ для kubectl, перейдите по адресу `https://kubeconfig.<modules.publicDomainTemplate>/`, выполните вход в нужного провайдера и скопируйте предложенные shell-команды.

- Для корректной работы аутентификации в Dashboard и kubectl требуется дополнительная настройка API-сервера. Этот процесс автоматизирован модулем [control-plane-manager](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/040-control-plane-manager/), который включен по умолчанию.
