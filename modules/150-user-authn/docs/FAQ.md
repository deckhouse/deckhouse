---
title: "The user-authn module: FAQ"
---

{% raw %}

## How to secure my application?

To enable Dex authentication for your application, follow these steps:
1. Create a [DexAuthenticator](cr.html#dexauthenticator) custom resource.

   Creating `DexAuthenticator` in a cluster results in an [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy) instance being created. The latter is connected to Dex. Once the `DexAuthenticator` custom resource becomes available, the necessary Deployment, Service, Ingress, Secret objects will be created in the specified namespace.

   An example of the `DexAuthenticator` custom resource:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: DexAuthenticator
   metadata:
     # Dex authenticator pod name prefix.
     # For example, if the name prefix is `app-name`, then Dex authenticator pods will look like `app-name-dex-authenticator-7f698684c8-c5cjg`.
     name: app-name
     # Namespace to deploy Dex authenticator to.
     namespace: app-ns
   spec:
     # Your application's domain. Requests to it will be redirected for Dex authentication.
     applicationDomain: "app-name.kube.my-domain.com"
     # A parameter that determines whether to send the `Authorization: Bearer` header to the application.
     # This one is useful in combination with auth_request in NGINX.
     sendAuthorizationHeader: false
     # The name of the Secret containing the SSL certificate.
     applicationIngressCertificateSecretName: "ingress-tls"
     # The name of the Ingress class to use in the Ingress resource created for the Dex authenticator.
     applicationIngressClassName: "nginx"
     # The duration of the active user session.
     keepUsersLoggedInFor: "720h"
     # The list of groups whose users are allowed to authenticate.
     allowedGroups:
     - everyone
     - admins
     # The list of addresses and networks for which authentication is allowed.
     whitelistSourceRanges:
     - 1.1.1.1/32
     - 192.168.0.0/24
   ```

2. Connect your application to Dex.

   For this, add the following annotations to the application's Ingress resource:

   - `nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in`
   - `nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email`
   - `nginx.ingress.kubernetes.io/auth-url: https://<NAME>-dex-authenticator.<NS>.svc.{{ C_DOMAIN }}/dex-authenticator/auth`, where:
      - `NAME` — the value of the `metadata.name` parameter of the `DexAuthenticator` resource;
      - `NS` — the value of the `metadata.namespace` parameter of the `DexAuthenticator` resource;
      - `C_DOMAIN` — the cluster domain (the [clusterDomain](../../installing/configuration.html#clusterconfiguration-clusterdomain) parameter of the `ClusterConfiguration` resource).

   Below is an example of annotations added to an application's Ingress resource so that it can be connected to Dex:

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

* Add the following annotation if you want to exclude users from specific networks from passing authentication via Dex and force users from all other networks to authenticate via Dex:

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
    enabled: true
  ```

The name `kubeconfig` is reserved for accessing the web interface that allows generating `kubeconfig`. The URL for access depends on the value of the parameter [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) (for example, for `publicDomainTemplate: %s.kube.my` it will be `kubeconfig.kube.my`, and for `publicDomainTemplate: %s-kube.company.my` it will be `kubeconfig-kube.company.my`).  
{% endraw %}

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

{% raw %}

### The flow of accessing Kubernetes API with generated kubeconfig

![Interaction scheme when accessing Kubernetes API using generated kubeconfig](../../images/150-user-authn/kubeconfig_dex.svg)

1. Before the start, kube-apiserver needs to request the configuration endpoint of the OIDC provider (Dex in our case) to get the issuer and JWKS endpoint settings.

2. Kubeconfig generator stores id token and refresh token to the kubeconfig file.

3. After receiving request with an id token, kube-apiserver goes to validate, that the token is signed by the provider configured on the first step by getting keys from the JWKS endpoint. As the next step, it compares `iss` and `aud` claims values of the token with the values from configuration.

## How secure is Dex from brute-forcing my credentials?

Only 20 login attempts are allowed per user. If this limit is used up, one additional attempt will be added every 6 seconds.

{% endraw %}
