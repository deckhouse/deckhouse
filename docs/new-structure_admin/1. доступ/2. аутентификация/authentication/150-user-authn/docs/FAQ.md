---
title: "The user-authn module: FAQ"
---

## How to secure my application?

It is possible to hide your application behind Dex authentication by using the `DexAuthenticator` custom resource (CR).
In fact, by creating the DexAuthenticator in a cluster, user creates an instance [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy), which is already connected to Dex.

### An example of the `DexAuthenticator` CR

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: my-cool-app # the authenticator's Pods will be prefixed with my-cool-app
  namespace: my-cool-namespace # the namespace where the dex-authenticator will be deployed
spec:
  applicationDomain: "my-app.kube.my-domain.com" # the domain used for your app
  sendAuthorizationHeader: false # whether to send the `Authorization: Bearer` header to the application (comes in handy with auth_request in nginx)
  applicationIngressCertificateSecretName: "ingress-tls" # the name of the secret with the tls certificate
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

After the `DexAuthenticator` custom resource is created in the cluster, Kubernetes will make the necessary deployment, service, ingress, secret in the specified namespace.
Add the following annotations to your app's Ingress resource to connect your application to dex:

{% raw %}

```yaml
annotations:
  nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
  nginx.ingress.kubernetes.io/auth-url: https://my-cool-app-dex-authenticator.my-cool-namespace.svc.{{ cluster domain, e.g., | cluster.local }}/dex-authenticator/auth
  nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email,Authorization
```

{% endraw %}

### Setting up CIDR-based restrictions

DexAuthenticator does not have a built-in system for allowing the user authentication based on its IP address. Instead, you can use annotations for Ingress resources:

* If you want to restrict access by IP and use Dex for authentication, add the following annotation with a comma-separated list of allowed CIDRs:

  ```yaml
  nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1`
  ```

* Add the following annotation if you want to exclude users from specific networks from passing authentication via dex, and force users from all other networks to authenticate via dex:

  ```yaml
  nginx.ingress.kubernetes.io/satisfy: "any"
  ```

### Authentication flow with DexAuthenticator

![Authentication flow with DexAuthenticator](../../images/150-user-authn/dex_login.svg)

1. Dex redirects the user to the provider's login page in most cases and wait for the user to be redirected back to the `/callback` URL. However, some providers like LDAP or Atlassian Crowd do not support this flow. The user should write credentials to the Dex login form instead, and Dex will make a request to the provider's API to validate them.

2. DexAuthenticator sets the cookie with the whole refresh token (instead of storing it in Redis like an id token) because Redis does not persist data.
If there is no id token by the id token ticket in Redis, the user will be able to get the new id token by providing the refresh token from the cookie.

3. DexAuthenticator sets the `Authorization` HTTP header to the ID token value from Redis. It is not required for services like [Upmeter](../500-upmeter/), because permissions to Upmeter entities are not highly grained.
On the other hand, for the [Kubernetes Dashboard](../500-dashboard/), it is a crucial functionality because it sends the ID token further to access Kubernetes API.
