---
title: "Модуль user-authn: FAQ"
---

## Как защитить мое приложение?

Существует возможность спрятать ваше приложение за аутентификацией через Dex с помощью пользовательского ресурса `DexAuthenticator` (custom resource).
По факту, создавая DexAuthenticator в кластере, пользователь создает экземпляр [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy), который уже подключен к Dex.

### Пример custom resource `DexAuthenticator`

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: my-cool-app # Поды аутентификатора будут иметь префикс my-cool-app.
  namespace: my-cool-namespace # Namespace, в котором будет развернут dex-authenticator.
spec:
  applicationDomain: "my-app.kube.my-domain.com" # Домен, на котором висит ваше приложение.
  sendAuthorizationHeader: false # Отправлять ли `Authorization: Bearer` header приложению, полезно в связке с auth_request в NGINX.
  applicationIngressCertificateSecretName: "ingress-tls" # Имя Secret'а с TLS-сертификатом.
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

После появления custom resource `DexAuthenticator` в кластере в указанном namespace'е появятся необходимые Deployment, Service, Ingress, Secret.
Чтобы подключить свое приложение к Dex, достаточно будет добавить в Ingress-ресурс вашего приложения следующие аннотации:

{% raw %}

```yaml
annotations:
  nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
  nginx.ingress.kubernetes.io/auth-url: https://my-cool-app-dex-authenticator.my-cool-namespace.svc.{{ домен вашего кластера, например | cluster.local }}/dex-authenticator/auth
  nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
```

{% endraw %}

### Настройка ограничений на основе CIDR

В DexAuthenticator нет встроенной системы управления разрешением аутентификации на основе IP-адреса пользователя. Вместо этого вы можете воспользоваться аннотациями для Ingress-ресурсов:

* Если нужно ограничить доступ по IP и оставить прохождение аутентификации в Dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:

  ```yaml
  nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1`
  ```

* Если вы хотите, чтобы пользователи из указанных сетей были освобождены от прохождения аутентификации в Dex, а пользователи из остальных сетей были обязаны аутентифицироваться в Dex, добавьте следующую аннотацию:

  ```yaml
  nginx.ingress.kubernetes.io/satisfy: "any"
  ```

### Как работает аутентификация с помощью DexAuthenticator

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

Одному пользователю разрешено только 20 попыток входа. Если лимит был израсходован, еще одна попытка будет добавлена каждые 6 секунд.
