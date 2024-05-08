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
{% endraw %}

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

{% raw %}

### Как работает подключение к Kubernetes API с помощью сгенерированного kubeconfig

![Схема взаимодействия при подключении к Kubernetes API с помощью сгенерированного kubeconfig](../../images/150-user-authn/kubeconfig_dex.svg)

1. До начала работы kube-apiserver необходимо запросить конфигурационный endpoint OIDC провайдера (в нашем случае — Dex), чтобы получить issuer и настройки JWKS endpoint.

2. Kubeconfig generator сохраняет ID token и refresh token в файл kubeconfig.

3. После получения запроса с ID token kube-apiserver идет проверять, что token подписан провайдером, который мы настроили на первом шаге, с помощью ключей, полученных с точки доступа JWKS. В качестве следующего шага он сравнивает значения claim'ов `iss` и `aud` из token'а со значениями из конфигурации.

## Как Dex защищен от подбора логина и пароля?

Одному пользователю разрешено только 20 попыток входа. Если указанный лимит израсходован, одна дополнительная попытка будет добавляться каждые 6 секунд.

{% endraw %}
