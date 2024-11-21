---
title: "Интеграция с внешними системами аутентификации"
permalink: ru/virtualization-platform/documentation/admin/platform-management/access-control/user-management.html
lang: ru
---

## Описание

Платформа поддерживает управление как внутренними пользователями и группами, так и интеграцию с внешними провайдерами аутентификации и протоколами, такими как:

- [GitHub](#github);
- [GitLab](#gitlab);
- [Crowd](#atlassian-crowd);
- [BitBucket Cloud](#bitbucket-cloud);
- [LDAP](#ldap);
- [OIDC](#oidc-openid-connect).

Можно подключить несколько внешних провайдеров аутентификации одновременно.

Пользователи могут получать доступ к веб-интерфейсам платформы (например, Grafana, Console), а также использовать командные утилиты (`d8`, `kubectl`) для взаимодействия с API платформы с учетом назначенных прав доступа.

Информация о назначении прав пользователям и группам представлена в [документации](./role-model.html).

## Создание пользователя

Для создания статического пользователя используется ресурс [User](../../../reference/cr.html#user).

Перед этим необходимо сгенерировать хеш пароля с помощью следующей команды:

```shell
echo "$password" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
```

Также можно воспользоваться [онлайн-сервисом Bcrypt](https://bcrypt-generator.com/).

Пример манифеста для создания пользователя:

```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: joe
spec:
  email: joe@example.com # используется в RoleBinding, ClusterRoleBinding для назначения прав пользователю
  password: $2a$10$etblbZ9yfZaKgbvysf1qguW3WULdMnxwWFrkoKpRH1yeWa5etjjAa
  ttl: 24h # (опционально) задает срок жизни учетной записи
```

## Создание группы пользователей

Для создания группы пользователей используется ресурс [Group](../../../reference/cr.html#group).

Пример манифеста для создания группы:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: vms-admins
spec:
  # список пользователей
  members:
  - kind: User
    name: joe
  name: vms-admins # используется в RoleBinding, ClusterRoleBinding для назначения прав группе пользователей
```

## Создание конфигурационного файла для удаленного доступа

Для того чтобы удалённо управлять кластером с помощью утилит командной строки (`d8` или `kubectl`), необходимо создать конфигурационный файл:

1. Включите доступ к API платформы, установив параметр `.spec.settings.publishAPI.enabled` в значении `true` в ресурсе ModuleConfig модуля`user-authn`.
1. Через веб-интерфейс kubeconfigurator, сгенерируйте `kubeconfig` файл для удалённого доступа к кластеру. Для доступа к веб-интерфейсу, позволяющему сгенерировать `kubeconfig`, зарезервировано имя `kubeconfig`. URL для доступа зависит от значения параметра `publicDomainTemplate`.

Чтобы узнать адрес, по которому доступен сервис, выполните следующую команду:

```bash
d8 k get ingress -n d8-user-authn
# NAME                   CLASS   HOSTS                              ADDRESS                            PORTS     AGE
# ...
# kubeconfig-generator   nginx   kubeconfig.example.com             172.25.0.2,172.25.0.3,172.25.0.4   80, 443   267d
# ...
```

1. Перейдите по предоставленному адресу и используйте в качестве учетных данных e-mail и пароль, которые вы указали при создании пользователя.

## Настройка внешних провайдеров

Для настройки внешни провайдеров используется ресурс [DexProvider](../../../reference/cr.html#dexprovider).

### GitHub

В примере представлены настройки провайдера для интеграции с GitHub:

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

В организации GitHub необходимо создать новое приложение, для этого выполните следующие шаги:

1. Перейдите в «Settings» → «Developer settings» → «OAuth Aps» → «Register a new OAuth application».
1. В поле «Authorization callback URL» укажите адрес:
   `https://dex.<modules.publicDomainTemplate>/callback`.

Полученные `Client ID` и `Client Secret` укажите в Custom Resource DexProvider.

Если организация GitHub находится под управлением клиента, выполните следующие шаги:

1. Перейдите в «Settings» -> «Applications» -> «Authorized OAuth Apps».
1. Найдите созданное приложение по имени и нажмите «Send Request» для подтверждения.
1. Попросите клиента подтвердить запрос, который будет отправлен ему на email.

После выполнения этих шагов ваше приложение будет готово для использования в качестве провайдера аутентификации через GitHub.

### GitLab

В примере представлены настройки провайдера для интеграции с GitLab.

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

Для того чтобы создать приложение в GitLab, выполните следующие шаги:

Для GitLab, размещённого на собственном сервере:

1. Перейдите в «Admin area» → «Application» → «New application».
1. В поле «Redirect URI (Callback URL)» укажите адрес:  
   `https://dex.<modules.publicDomainTemplate>/callback`.
1. Выберите следующие scopes:
   - `read_user`
   - `openid`

Для GitLab, размещённого в облаке:

1. Под главной учетной записью проекта перейдите в «User Settings» → «Applications» → «New application».
1. В поле «Redirect URI (Callback URL)» укажите адрес:  
   `https://dex.<modules.publicDomainTemplate>/callback`.
1. Выберите следующие scopes:
   - `read_user`
   - `openid`

Для GitLab версии 16 и выше:

 Включите опцию «Trusted»:  
`Trusted applications are automatically authorized on GitLab OAuth flow` при создании приложения.

Полученные `Application ID` и `Secret` укажите в Custom Resource DexProvider.

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
    enableBasicAuth: true
    groups:
    - administrators
    - users
```

Для того чтобы создать Generic-приложение в Atlassian Crowd, выполните следующие шаги:

1. Перейдите в раздел «Applications» → «Add application».
1. Полученные «Application Name» и «Password» укажите в Custom Resource [DexProvider](../../../reference/cr.html#dexprovider).

Полученные `Application Name` и `Password` укажите в Custom Resource DexProvider.

Группы CROWD укажите в lowercase-формате для Custom Resource `DexProvider`.

### Bitbucket Cloud

В примере представлены настройки провайдера для интеграции с Bitbucket.

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

Для настройки аутентификации в Bitbucket выполните следующие шаги:

1. В меню команды создайте новый OAuth consumer.
1. Перейдите в «Settings» → «OAuth consumers» → «New application» и в качестве «Callback URL» укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`.
1. Разрешите доступ для `Account: Read` и `Workspace membership: Read`.

Полученные `Key` и `Secret` укажите в Custom Resource DexProvider.

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

Для настройки аутентификации заведите в LDAP read-only-пользователя (service account).

Полученные путь до пользователя и пароль укажите в параметрах `bindDN` и `bindPW` Custom Resource DexProvider.

1. Если в LDAP настроен анонимный доступ на чтение, настройки можно не указывать.
1. В параметре `bindPW` укажите пароль в plain-виде. Стратегии с передачей хэшированных паролей не предусмотрены.

### OIDC (OpenID Connect)

Аутентификация через OIDC-провайдера требует регистрации клиента (или создания приложения). Сделайте это по документации вашего провайдера (например, [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration) или [Blitz](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html)).

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` укажите в Custom Resource DexProvider.

Далее можно ознакомиться с некоторыми примерами.

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
    insecureSkipEmailVerified: true
    getUserInfo: true
```

#### Blitz Identity Provider

На стороне провайдера Blitz Identity Provider при [регистрации приложения](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html) необходимо указать URL для перенаправления пользователя после авторизации. При использовании `DexProvider` необходимо указать `https://dex.<publicDomainTemplate>/`, где `publicDomainTemplate` – [указанный](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) в модуле `global` шаблон DNS-имен кластера.

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

Чтобы корректно работал выход из приложений (происходил отзыв токена и требовалась повторная авторизация), нужно установить `login` в значении параметра `promptType`.

Для обеспечения детализированного доступа пользователя к приложениям необходимо:

* добавить параметр `allowedUserGroups` в ModuleConfig нужного приложения;
* добавить группы к пользователю (наименования групп должны совпадать как на стороне Blitz, так и на стороне Deckhouse).

Пример добавления групп для модуля Prometheus:

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
