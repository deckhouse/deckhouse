modules/150-user-authn/docs/USAGE_RU.md

---
title: "Модуль user-authn: примеры конфигурации"
---

## Пример конфигурации модуля

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 1
  enabled: true
  settings:
    kubeconfigGenerator:
    - id: direct
      masterURI: https://159.89.5.247:6443
      description: "Direct access to kubernetes API"
    publishAPI:
      enable: true
```

{% endraw %}

## Примеры настройки провайдера

### GitHub

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

В организации GitHub необходимо создать новое приложение.

Для этого необходимо перейти в `Settings` -> `Developer settings` -> `OAuth Aps` -> `Register a new OAuth application` и в качестве `Authorization callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`.

Полученные `Client ID` и `Client Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

В том случае, если организация GitHub находится под управлением клиента, необходимо перейти в `Settings` -> `Applications` -> `Authorized OAuth Apps` -> `<name of created OAuth App>` и запросить подтверждение нажатием на `Send Request`. После попросить клиента подтвердить запрос, который придет к нему на email.

### GitLab

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

В GitLab проекта необходимо создать новое приложение.

Для этого необходимо:
* **self-hosted**: перейти в `Admin area` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, scopes выбрать: `read_user`, `openid`;
* **cloud gitlab.com**: под главной учетной записью проекта перейти в `User Settings` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, scopes выбрать: `read_user`, `openid`;
* (для GitLab версии 16 и выше) включить опцию `Trusted`/`Trusted applications are automatically authorized on Gitlab OAuth flow` при создании приложения.

Полученные `Application ID` и `Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### Atlassian Crowd

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

В соответствующем проекте Atlassian Crowd необходимо создать новое `Generic`-приложение.

Для этого необходимо перейти в `Applications` -> `Add application`.

Полученные `Application Name` и `Password` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

Группы CROWD указываются в lowercase-формате для custom resource `DexProvider`.

### Bitbucket Cloud

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

Для настройки аутентификации необходимо в Bitbucket в меню команды создать нового OAuth consumer.

Для этого необходимо перейти в `Settings` -> `OAuth consumers` -> `New application` и в качестве `Callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, разрешить доступ для `Account: Read` и `Workspace membership: Read`.

Полученные `Key` и `Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### OIDC (OpenID Connect)

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

Аутентификация через OIDC-провайдера требует регистрации клиента (или создания приложения). Сделайте это по документации вашего провайдера (например, [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration)).

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### LDAP

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

Для настройки аутентификации необходимо завести в LDAP read-only-пользователя (service account).

Полученные путь до пользователя и пароль необходимо указать в полях `bindDN` и `bindPW` custom resource [DexProvider](cr.html#dexprovider).
1. Если в LDAP настроен анонимный доступ на чтение, настройки можно не указывать.
2. В поле `bindPW` необходимо указывать пароль в plain-виде. Стратегии с передачей хэшированных паролей не предусмотрены.

## Настройка OAuth2-клиента в Dex для подключения приложения

Данный вариант настройки подходит приложениям, которые имеют возможность использовать oauth2-аутентификацию самостоятельно, без помощи oauth2-proxy.
Чтобы позволить подобным приложениям взаимодействовать с Dex, используется custom resource [`DexClient`](cr.html#dexclient).

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

Пароль для доступа к клиенту (**clientSecret**) будет сохранен в Secret'е:
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

## Пример создания статического пользователя

Придумайте пароль и укажите его хэш-сумму в поле `password`.

Для вычисления хэш-суммы пароля воспользуйтесь командой:

```shell
echo "$password" | htpasswd -inBC 10 "" | tr -d ':\n' | sed 's/$2y/$2a/'
```

Также можно воспользоваться [онлайн-сервисом](https://bcrypt-generator.com/).

{% raw %}

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

{% endraw %}

## Пример добавления статического пользователя в группу

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

---
title: "Модуль user-authn: FAQ"
---

{% raw %}

## Как защитить мое приложение?

Чтобы включить аутентификацию через Dex для приложения, выполните следующие шаги:
1. Создайте custom resource [DexAuthenticator](cr.html#dexauthenticator).

   Создание `DexAuthenticator` в кластере приводит к созданию экземпляра [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy), подключенного к Dex. После появления custom resource `DexAuthenticator` в указанном namespace появятся необходимые объекты Deployment, Service, Ingress, Secret.

   Пример custom resource `DexAuthenticator`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: DexAuthenticator
   metadata:
     # Префикс имени подов Dex authenticator.
     # Например, если префикс имени `app-name`, то поды Dex authenticator будут вида `app-name-dex-authenticator-7f698684c8-c5cjg`.
     name: app-name
     # Namespace, в котором будет развернут Dex authenticator.
     namespace: app-ns
   spec:
     # Домен вашего приложения. Запросы на него будут перенаправляться для прохождения аутентификацию в Dex.
     applicationDomain: "app-name.kube.my-domain.com"
     # Отправлять ли `Authorization: Bearer` header приложению. Полезно в связке с auth_request в NGINX.
     sendAuthorizationHeader: false
     # Имя Secret'а с SSL-сертификатом.
     applicationIngressCertificateSecretName: "ingress-tls"
     # Название Ingress-класса, которое будет использоваться в создаваемом для Dex authenticator Ingress-ресурсе.
     applicationIngressClassName: "nginx"
     # Время, на протяжении которого пользовательская сессия будет считаться активной.
     keepUsersLoggedInFor: "720h"
     # Список групп, пользователям которых разрешено проходить аутентификацию.
     allowedGroups:
     - everyone
     - admins
     # Список адресов и сетей, с которых разрешено проходить аутентификацию.
     whitelistSourceRanges:
     - 1.1.1.1/32
     - 192.168.0.0/24
   ```

2. Подключите приложение к Dex.

   Для этого добавьте в Ingress-ресурс приложения следующие аннотации:

   - `nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in`
   - `nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email`
   - `nginx.ingress.kubernetes.io/auth-url: https://<NAME>-dex-authenticator.<NS>.svc.{{ C_DOMAIN }}/dex-authenticator/auth`, где:
      - `NAME` — значение параметра `metadata.name` ресурса `DexAuthenticator`;
      - `NS` — значение параметра `metadata.namespace` ресурса `DexAuthenticator`;
      - `C_DOMAIN` — домен кластера (параметр [clusterDomain](../../installing/configuration.html#clusterconfiguration-clusterdomain) ресурса `ClusterConfiguration`).

   Ниже представлен пример аннотаций на Ingress-ресурсе приложения, для подключения его к Dex:

   ```yaml
   annotations:
     nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
     nginx.ingress.kubernetes.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
     nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
   ```

### Настройка ограничений на основе CIDR

В DexAuthenticator нет встроенной системы управления разрешением аутентификации на основе IP-адреса пользователя. Вместо этого вы можете воспользоваться аннотациями для Ingress-ресурсов:

* Если нужно ограничить доступ по IP и оставить прохождение аутентификации в Dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:

  ```yaml
  nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1
  ```

* Если необходимо, чтобы пользователи из указанных сетей освобождались от прохождения аутентификации в Dex, а пользователи из остальных сетей обязательно аутентифицировались в Dex, добавьте следующую аннотацию:

  ```yaml
  nginx.ingress.kubernetes.io/satisfy: "any"
  ```

## Как работает аутентификация с помощью DexAuthenticator

![Как работает аутентификация с помощью DexAuthenticator](../../images/150-user-authn/dex_login.svg)

1. Dex в большинстве случаев перенаправляет пользователя на страницу входа провайдера и ожидает, что пользователь будет перенаправлен на его `/callback` URL. Однако такие провайдеры, как LDAP или Atlassian Crowd, не поддерживают этот вариант. Вместо этого пользователь должен ввести свои логин и пароль в форму входа в Dex, и Dex сам проверит их верность, сделав запрос к API провайдера.

2. DexAuthenticator устанавливает cookie с целым refresh token (вместо того чтобы выдать тикет, как для ID token) потому что Redis не сохраняет данные на диск.
Если по тикету в Redis не найден ID token, пользователь сможет запросить новый ID token, предоставив refresh token из cookie.

3. DexAuthenticator выставляет HTTP-заголовок `Authorization`, равный значению ID token из Redis. Это необязательно для сервисов по типу [Upmeter](../500-upmeter/), потому что права доступа к Upmeter не такие проработанные.
С другой стороны, для [Kubernetes Dashboard](../500-dashboard/) это критичный функционал, потому что она отправляет ID token дальше для доступа к Kubernetes API.

## Как я могу сгенерировать kubeconfig для доступа к Kubernetes API?

Сгенерировать `kubeconfig` для удаленного доступа к кластеру через `kubectl` можно через веб-интерфейс `kubeconfigurator`.

Настройте параметр [publishAPI](configuration.html#parameters-publishapi):
- Откройте настройки модуля `user-authn` (создайте ресурс moduleConfig `user-authn`, если его нет):

  ```shell
  kubectl edit mc user-authn
  ```

- Добавьте следующую секцию в блок `settings` и сохраните изменения:

  ```yaml
  publishAPI:
    enable: true
  ```

Для доступа к веб-интерфейсу, позволяющему сгенерировать `kubeconfig`, зарезервировано имя `kubeconfig`. URL для доступа зависит от значения параметра [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) (например, для `publicDomainTemplate: %s.kube.my` это будет `kubeconfig.kube.my`, а для `publicDomainTemplate: %s-kube.company.my` — `kubeconfig-kube.company.my`)  

### Настройка kube-apiserver

С помощью функционала модуля [control-plane-manager](../../modules/040-control-plane-manager/) Deckhouse автоматически настраивает kube-apiserver, выставляя следующие флаги так, чтобы модули `dashboard` и `kubeconfig-generator` могли работать в кластере.

{% offtopic title="Аргументы kube-apiserver, которые будут настроены" %}

* `--oidc-client-id=kubernetes`
* `--oidc-groups-claim=groups`
* `--oidc-issuer-url=https://dex.%addonsPublicDomainTemplate%/`
* `--oidc-username-claim=email`

В случае использования самоподписанных сертификатов для Dex будет добавлен еще один аргумент, а также в под с apiserver будет смонтирован файл с CA:

* `--oidc-ca-file=/etc/kubernetes/oidc-ca.crt`
  {% endofftopic %}

### Как работает подключение к Kubernetes API с помощью сгенерированного kubeconfig

<img src="../../images/150-user-authn/kubeconfig_dex.svg">

1. До начала работы kube-apiserver необходимо запросить конфигурационный endpoint OIDC провайдера (в нашем случае — Dex), чтобы получить issuer и настройки JWKS endpoint.

2. Kubeconfig generator сохраняет ID token и refresh token в файл kubeconfig.

3. После получения запроса с ID token kube-apiserver идет проверять, что token подписан провайдером, который мы настроили на первом шаге, с помощью ключей, полученных с точки доступа JWKS. В качестве следующего шага он сравнивает значения claim'ов `iss` и `aud` из token'а со значениями из конфигурации.

## Как Dex защищен от подбора логина и пароля?

Одному пользователю разрешено только 20 попыток входа. Если указанный лимит израсходован, одна дополнительная попытка будет добавляться каждые 6 секунд.

{% endraw %}
