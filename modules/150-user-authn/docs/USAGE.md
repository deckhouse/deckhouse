---
title: "Модуль user-authn: примеры использования"
---

## Настройка OAuth2 клиента в dex для подключения приложения

Данный вариант настройки подходит приложением, которые имеют возможность использовать oauth2-аутентификацию самостоятельно без помощи oauth2-proxy.
Чтобы позволить подобным приложениям взаимодействовать с dex используется Custom Resource [`DexClient`](cr.html#dexclient).

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

После создание такого ресурса, в dex будет зарегистрирован клиент с идентификатором (clientID) - `dex-client-myname:mynamespace`

Пароль для доступа к клиенту (clientSecret) будет сохранен в секрете:
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

## Настройка kube-apiserver

Для работы модулей dashboard и kubeconfig-generator в кластере необходимо настроить kube-apiserver. Для этого предусмотрен специальный модуль [control-plane-configurator](/modules/160-control-plane-configurator/).

{% offtopic title="Аргументы kube-apiserver, которые будут настроены" %}

* --oidc-client-id=kubernetes
* --oidc-groups-claim=groups
* --oidc-issuer-url=https://dex.%addonsPublicDomainTemplate%/
* --oidc-username-claim=email

В случае использования самоподписанных сертификатов для dex будет добавлен ещё один аргумент, а так же в под с apiserver будет смонтирован файл с CA:

* --oidc-ca-file=/etc/kubernetes/oidc-ca.crt
{% endofftopic %}

## Создание приложения в Crowd для аутентификации в кластере

В Crowd, в соответствующем проекте, необходимо создать новое `Generic` приложение.

Для этого необходимо перейти в `Applications` -> `Add application`.

Полученные `Application Name` и `Password` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

## Создание приложения в GitHub организации для аутентификации в кластере

В организации GitHub необходимо создать новое приложение.

Для этого необходимо перейти в `Settings` -> `Developer settings` -> `OAuth Aps` -> `Register a new OAuth application` и в качестве `Authorization callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`.

Полученные `Client ID` и `Client Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

В том случае, если организация Github находится под управлением клиента, необходимо перейти в `Settings` -> `Applications` -> `Authorized OAuth Apps` -> `<name of created OAuth App>` и запросить подтверждение нажатием на `Send Request`. После попросить клиента подтвердить запрос, который придет к нему на email.

## Создание приложения в GitLab для аутентификации в кластере

В GitLab проекта необходимо создать новое приложение.

Для этого необходимо перейти в `Admin area` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, scopes выбрать: `read_user`, `openid`.

Полученные `Application ID` и `Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

## Создание приложения в Atlassian Crowd для аутентификации в кластере

Для настройки аутентификации с помощью модуля `user-authn` необходимо в Crowd'е проекта создать новое `Generic` приложение.

Для этого необходимо перейти в `Applications` -> `Add application`.

Полученные `Application Name` и `Password`  необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

## Создание приложения в Bitbucket Cloud для аутентификации в кластере

Для настройки аутентификации с помощью модуля `user-authn` необходимо в Bitbucket в меню команды создать нового OAuth consumer.

Для этого необходимо перейти в `Settings` -> `OAuth consumers` -> `New application` и в качестве `Callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, разрешить доступ только для `Account: Read`.

Полученные `Key` и `Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

## Создание приложения для аутентификации в кластере через OIDC (OpenID Connect)

Для настройки аутентификации с помощью модуля `user-authn` необходимо проконсультироваться с документацией
вашего провайдера для создания приложения.

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

## Создание приложения для аутентификации в кластере через LDAP

Для настройки аутентификации с помощью модуля `user-authn` необходимо завести в LDAP read-only пользователя (service account).

Полученные путь до пользователя и пароль необходимо указать в полях `bindDN` и `bindPW` custom resource [DexProvider](cr.html#dexprovider).
1. Если в LDAP настроен анонимный доступ на чтение, настройки можно не указывать.
2. В поле `bindPW` необходимо указывать пароль в plain-виде. Стратегии с передачей хешированных паролей не предусмотрены.


<!--## Connect an authentication provider using config values-->
<!--## Create static users using User CRD-->
<!--## Deploy DexAuthenticator CRD leads to the creation of oauth2 proxy Deployment with various parameters, Ingress, Service and Oauth2 client for accessing dex-->
<!--## Specifying Atlassian Crowd provider with enableBasicAuth option set to true leads to the creation of Deployment with crowd-basic-auth-proxy-->
<!--## Enabling of publishAPI option leads to the creation of Ingress object for apiserver connection with desired ingress-shim annotation-->
<!--## Switching on Control Plane Configurator for the module should add special Configmap to the cluster and generate necessary values-->
<!--## Specifying KubeconfigGenerator in module settings adds parameters to KubeconfigGenerator Configmap-->
<!--## Deploy of DexClient CRD must register oauth2 client entry to dex.-->

