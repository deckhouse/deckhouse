---
title: "Модуль user-authn: примеры конфигурации"
---

## Пример конфигурации модуля

В примере представлена конфигурация модуля `user-authn` в Deckhouse Platform Certified Security Edition.

{% raw %}

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
      masterURI: https://<IP>:6443
      description: "Direct access to kubernetes API"
    publishAPI:
      enabled: true
```

{% endraw %}

## Примеры настройки провайдера

### OIDC (OpenID Connect)

Аутентификация через OIDC-провайдера требует регистрации клиента (или создания приложения). Сделайте это по документации вашего провайдера (например, [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration) или [Blitz](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html)).

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` укажите в Custom Resource [DexProvider](cr.html#dexprovider).

Ниже можно ознакомиться с некоторыми примерами.

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

Полученные путь до пользователя и пароль укажите в параметрах `bindDN` и `bindPW` Custom Resource [DexProvider](cr.html#dexprovider).
1. Если в LDAP настроен анонимный доступ на чтение, настройки можно не указывать.
2. В параметре `bindPW` укажите пароль в plain-виде. Стратегии с передачей хэшированных паролей не предусмотрены.

## Настройка OAuth2-клиента в Dex для подключения приложения

Этот вариант настройки подходит приложениям, которые имеют возможность использовать OAuth2-аутентификацию самостоятельно, без помощи `oauth2-proxy`.
Чтобы позволить подобным приложениям взаимодействовать с Dex, используется Custom Resource [`DexClient`](cr.html#dexclient).

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

## Локальная аутентификация

Локальная аутентификация обеспечивает проверку и управление доступом пользователей с возможностью настройки парольной политики, поддержкой двухфакторной аутентификации (2FA) и управлением группами.  
Реализация соответствует требованиям безопасности ФСТЭК и рекомендациям OWASP, обеспечивая надёжную защиту доступа к кластеру и приложениям без необходимости интеграции с внешними системами аутентификации.

### Создание пользователя

Придумайте пароль и укажите его хэш-сумму, закодированную в base64, в поле `password`.

Для вычисления хэш-суммы пароля воспользуйтесь командой:

```shell
echo -n '3xAmpl3Pa$$wo#d' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
```

{% alert level="info" %}
Если команда `htpasswd` недоступна, установите соответствующий пакет:

* `apache2-utils` — для дистрибутивов, основанных на Debian;
* `httpd-tools` — для дистрибутивов, основанных на CentOS;
* `apache2-htpasswd` — для ALT Linux.
{% endalert %}

Также можно воспользоваться [онлайн-сервисом](https://bcrypt-generator.com/).

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

### Добавление пользователя в группу

Пользователи могут быть объединены в группы для управления правами доступа. Пример манифеста ресурса Group для группы:

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

Здесь `members` — список пользователей, которые входят в группу.

### Парольная политика

Настройки парольной политики позволяют контролировать сложность пароля, ротацию и блокировку пользователей:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
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

* `complexityLevel` — уровень сложности пароля;
* `passwordHistoryLimit` — число предыдущих паролей, которые хранит система, чтобы предотвратить их повторное использование;
* `lockout` — настройки блокировки при превышении лимита неудачных попыток входа:
  * `lockout.maxAttempts` — лимит неудачных попыток;
  * `lockout.lockDuration` — длительность блокировки пользователя;
* `rotation` — настройки ротации паролей:
  * `rotation.interval` — период обязательной смены пароля.

### Двухфакторная аутентификация (2FA)

2FA позволяет повысить уровень безопасности, требуя ввести код из приложения-аутентификатора TOTP при входе.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
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

### Выдача прав пользователю или группе

Для настройки используются параметры в Custom Resource [`ClusterAuthorizationRule`](/modules/user-authz/cr.html#clusterauthorizationrule).
