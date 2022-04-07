---
title: "Модуль user-authn: FAQ"
---

## Как защитить мое приложение?

Существует возможность спрятать ваше приложение за аутентификацией через Dex при помощи пользовательского ресурса `DexAuthenticator` (CR).
По факту, создавая DexAuthenticator в кластере, пользователь создает экземпляр [oauth2-proxy](https://github.com/pusher/oauth2_proxy), который уже подключен к Dex.

### Пример CR `DexAuthenticator`

{% raw %}
```yaml
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: my-cool-app # Pod'ы аутентификатора будут иметь префикс my-cool-app
  namespace: my-cool-namespace # namespace, в котором будет развернут dex-authenticator
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

### Настройка ограничений на основе CIDR

В DexAuthenticator нет встроенной системы управления разрешением аутентификации на основе IP адреса пользователя. Вместо этого вы можете воспользоваться аннотациями для Ingress-ресурсов:

* Если нужно ограничить доступ по IP и оставить прохождение аутентификации в dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:
```yaml
nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1`
```
* Если вы хотите, чтобы пользователи из указанных сетей были освобождены от прохождения аутентификации в dex, а пользователи из остальных сетей были обязаны аутентифицироваться в Dex - добавьте следующую аннотацию:
```yaml
nginx.ingress.kubernetes.io/satisfy: "any"
```

### Как работает аутентификация при помощи DexAuthenticator

<img src="../../images/150-user-authn/dex_login.svg">

1. Dex в большинстве случаев перенаправляет пользователя на страницу входа провайдера и ожидает, что пользователь будет перенаправлен на его `/callback` URL. Однако, такие провайдеры как LDAP или Atlassian Crowd не поддерживают этот вариант. Вместо этого пользователь должен ввести свои логин и пароль в форму входа в Dex, и Dex сам проверит их верность сделав запрос к API провайдера.

2. DexAuthenticator устанавливает cookie с целым refresh token (вместо того чтобы выдать тикет, как для id token) потому что Redis не сохраняет данные на диск.
Если по тикету в Redis не найден id token, пользователь сможет запросить новый id token предоставив refresh token из cookie.

3. DexAuthenticator выставляет хидер `Authorization` равный значению id token из Redis. Это не обязательно для сервисов по типу Upmeter, потому что права доступа к Upmeter не такие проработанные.
С другой стороны, для Kubernetes Dashboard это критичный функционал, потому что она отправляет id token дальше для доступа к Kubernetes API.

## Как я могу сгенерировать kubeconfig для доступа к Kubernetes API?

Для начала, в ConfigMap `deckhouse` настройте `publishAPI`:

{% raw %}
```yaml
  userAuthn: |
    publishAPI:
      enable: true
```
{% endraw %}

После по адресу `kubeconfig.%publicDomainTemplate%` появится веб-интерфейс, позволяющий сгенерировать `kubeconfig`.

### Настройка kube-apiserver

При помощи функционала модуля [control-plane-manager](../../modules/040-control-plane-manager/), Deckhouse автоматически настраивает kube-apiserver выставляя следующие флаги, так чтобы модули dashboard и kubeconfig-generator могли работать в кластере.

{% offtopic title="Аргументы kube-apiserver, которые будут настроены" %}

* --oidc-client-id=kubernetes
* --oidc-groups-claim=groups
* --oidc-issuer-url=https://dex.%addonsPublicDomainTemplate%/
* --oidc-username-claim=email

В случае использования самоподписанных сертификатов для Dex будет добавлен ещё один аргумент, а также в Pod с apiserver будет смонтирован файл с CA:

* --oidc-ca-file=/etc/kubernetes/oidc-ca.crt
  {% endofftopic %}

### Как работает подключение к Kubernetes API при помощи сгенерированного kubeconfig

<img src="../../images/150-user-authn/kubeconfig_dex.svg">

1. До начала работы, kube-apiserver необходимо запросить конфигурационный endpoint OIDC провайдера (в нашем случае Dex) чтобы получить issuer и настройки JWKS endpoint.

2. Kubeconfig generator сохраняет id token и refresh token в файл kubeconfig.

3. После получения запроса с id token, kube-apiserver идет проверять, что token подписан провайдером, который мы настроили на первом шаге, при помощи ключей полученных с точки доступа JWKS. В качестве следующего шага, он сравнивает значения claim'ов `iss` и `aud` из token'а со значениями из конфигурации. 


## Как Dex защищен от подбора логина и пароля?

Одному пользователю разрешено только 20 попыток входа. Если лимит был израсходован, еще одна попытка будет добавлена каждые 6 секунд.
