---
title: "The user-authn module: FAQ"
---

{% raw %}

## How to secure my application?

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

### Setting up CIDR-based restrictions

DexAuthenticator does not have a built-in system for allowing the user authentication based on its IP address. Instead, you can use annotations for Ingress resources:

* If you want to restrict access by IP and use Dex for authentication, add the following annotation with a comma-separated list of allowed CIDRs:

  ```yaml
  nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1
  ```

* Add the following annotation if you want to exclude users from specific networks from passing authentication via Dex, and force users from all other networks to authenticate via Dex:

  ```yaml
  nginx.ingress.kubernetes.io/satisfy: "any"
  ```

## Authentication flow with DexAuthenticator

![Authentication flow with DexAuthenticator](../../images/150-user-authn/dex_login.svg)

1. Dex redirects the user to the provider's login page in most cases and wait for the user to be redirected back to the `/callback` URL. However, some providers like LDAP or Atlassian Crowd do not support this flow. The user should write credentials to the Dex login form instead, and Dex will make a request to the provider's API to validate them.

2. DexAuthenticator sets the cookie with the whole refresh token (instead of storing it in Redis like an id token) because Redis does not persist data.
If there is no id token by the id token ticket in Redis, the user will be able to get the new id token by providing the refresh token from the cookie.

3. DexAuthenticator sets the `Authorization` HTTP header to the ID token value from Redis. It is not required for services like [Upmeter](../500-upmeter/), because permissions to Upmeter entities are not highly grained.
On the other hand, for the [Kubernetes Dashboard](../500-dashboard/), it is a crucial functionality because it sends the ID token further to access Kubernetes API.

## How can I generate a kubeconfig and access Kubernetes API?

You can generate `kubeconfig` for remote access to the cluster via `kubectl` via the `kubeconfigurator` web interface.

Configure the [publishAPI](configuration.html#parameters-publishapi) parameter:
- Open the `user-authn` module settings (create the moduleConfig `user-authn` resource if there is none):

  ```shell
  kubectl edit mc user-authn
  ```

- Add the following section to the `settings` block and save the changes:

  ```yaml
  publishAPI:
    enable: true
  ```

The name `kubeconfig` is reserved for accessing the web interface that allows generating `kubeconfig`. The URL for access depends on the value of the parameter [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) (for example, for `publicDomainTemplate: %s.kube.my` it will be `kubeconfig.kube.my`, and for `publicDomainTemplate: %s-kube.company.my` it will be `kubeconfig-kube.company.my`).  

### Configuring kube-apiserver

With the functional of the [control-plane-manager](../../modules/040-control-plane-manager/) module, Deckhouse automatically configures kube-apiserver by providing the following flags, so that dashboard and kubeconfig-generator modules can work in the cluster.

{% offtopic title="kube-apiserver arguments that will be configured" %}

* `--oidc-client-id=kubernetes`
* `--oidc-groups-claim=groups`
* `--oidc-issuer-url=https://dex.%addonsPublicDomainTemplate%/`
* `--oidc-username-claim=email`

If self-signed certificates are used, Dex will get one more argument. At the same time, the CA file will be mounted to the apiserver's Pod:

* `--oidc-ca-file=/etc/kubernetes/oidc-ca.crt`
  {% endofftopic %}
  
### The flow of accessing Kubernetes API with generated kubeconfig

<img src="../../images/150-user-authn/kubeconfig_dex.svg">

1. Before the start, kube-apiserver needs to request the configuration endpoint of the OIDC provider (Dex in our case) to get the issuer and JWKS endpoint settings.

2. Kubeconfig generator stores id token and refresh token to the kubeconfig file.

3. After receiving request with an id token, kube-apiserver goes to validate, that the token is signed by the provider configured on the first step by getting keys from the JWKS endpoint. As the next step, it compares `iss` and `aud` claims values of the token with the values from configuration.

## How secure is Dex from brute-forcing my credentials?

Only 20 authentication requests are allowed for a single user. If the limit exceeds, another login attempt will be allowed each six seconds.

{% endraw %}
