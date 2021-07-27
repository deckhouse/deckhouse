---
title: "The user-authn module: usage"
---

## An example of the module configuration

{% raw %}
```yaml
  userAuthn: |
    kubeconfigGenerator:
    - id: direct
      masterURI: https://159.89.5.247:6443
      description: "Direct access to kubernetes API"
    publishAPI:
      enable: true
```
{% endraw %}

## An example of the `DexAuthenticator` CR

{% raw %}
```yaml
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: my-cool-app # the authenticator's pods will be prefixed with my-cool-app
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

After the `DexAuthenticator` CR is created in the cluster, Kubernetes will make the necessary deployment, service, ingress, secret in the specified namespace.
Add the following annotations to your app's Ingress resource to connect your application to dex:

{% raw %}
```yaml
annotations:
  nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
  nginx.ingress.kubernetes.io/auth-url: https://my-cool-app-dex-authenticator.my-cool-namespace.svc.{{ cluster domain, e.g., | cluster.local }}/dex-authenticator/auth
  nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
```
{% endraw %}

### Setting up CIDR-based restrictions

DexAuthenticator does not have a built-in system for allowing the user authentication based on its IP address. Instead, you can use annotations for Ingress resources:

* If you want to restrict access by IP and use dex for authentication, add the following annotation with a comma-separated list of allowed CIDRs:
```yaml
nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1`
```
* Add the following annotation if you want to exclude users from specific networks from passing authentication via dex, and force users from all other networks to authenticate via dex:
```yaml
nginx.ingress.kubernetes.io/satisfy: "any"
```

## Configuring a provider
### Github

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: github
spec:
  type: Github
  displayName: My Company Github
  github:
    clientID: plainstring
    clientSecret: plainstring
```

In your GitHub organization, create a new application:

To do this, go to `Settings` -> `Developer settings` -> `OAuth Aps` -> `Register a new OAuth application` and specify the `https://dex.<modules.publicDomainTemplate>/callback` address as the `Authorization callback URL`.

Paste the generated `Client ID` and `Client Secret` into the [DexProvider](cr.html#dexprovider) custom resource.

If the Github organization is managed by the client, go to `Settings` -> `Applications` -> `Authorized OAuth Apps` -> `<name of created OAuth App>` and request confirmation by clicking on `Send Request`. Then ask the client to confirm the request that will be sent to him by email.

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
Create a new application in the GitLab project.

To do this, you need to:
* **self-hosted**: go to `Admin area` -> `Application` -> `New application` and specify the `https://dex.<modules.publicDomainTemplate>/callback` address as the `Redirect URI (Callback url)` and set scopes `read_user`, `openid`;
* **cloud gitlab.com**: under the main project account, go to `User Settings` -> `Application` -> `New application` and specify the `https://dex.<modules.publicDomainTemplate>/callback` address as the `Redirect URI (Callback url)`; also, don't forget to set scopes `read_user`, `openid`.

Paste the generated `Application ID` and `Secret` into the [DexProvider](cr.html#dexprovider) custom resource.

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

Create a new `Generic` application in the corresponding Atlassian Crowd project.

To do this, go to `Applications` -> `Add application`.

Paste the generated `Application Name` and `Password` into the [DexProvider](cr.html#dexprovider) custom resource.

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

Create a new OAuth consumer in the Bitbucket's team menu.

To do this, go to `Settings` -> `OAuth consumers` -> `New application` and specify the `https://dex.<modules.publicDomainTemplate>/callback` address as the `Callback URL`. Also, allow access for `Account: Read` and `Workspace membership: Read`.

Paste the generated `Key` and `Secret` into the [DexProvider](cr.html#dexprovider) custom resource.

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

To configure authentication, refer to your provider's documentation on how to create an application.

Paste the generated `clientID` and `clientSecret` into the [DexProvider](cr.html#dexprovider) custom resource.

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

To configure authentication, create a read-only user (service account) in LDAP.

Specify the generated user path and password in the `bindDN` and `bindPW` fields of the [DexProvider](cr.html#dexprovider) custom resource.
1. You can omit these settings of anonymous read access is configured for LDAP.
2. Enter the password into the `bindPW` in the plain text format. Strategies involving the passing of hashed passwords are not supported.

## Configuring the OAuth2 client in dex for connecting an application

This configuration is suitable for applications that can independently perform oauth2 authentication without using an oauth2 proxy.
The [`DexClient`](cr.html#dexclient) custom resource enables applications to use dex.

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

After the DexClient CR is created, dex will register a client with a `dex-client-myname:mynamespace` ID (clientID).

The client access password (clientSecret) will be stored in the secret object:
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

## An example of creating a static user:
{% raw %}
```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@yourcompany.com
  password: $2a$10$etblbZ9yfZaKgbvysf1qguW3WULdMnxwWFrkoKpRH1yeWa5etjjAa
  userID: some-unique-user-id
  groups:
  - Everyone
  - admins
  ttl: 24h
```
{% endraw %}

## External access to the Kubernetes API

In the `deckhouse` ConfigMap configure `publishAPI`:

{% raw %}
```yaml
  userAuthn: |
    publishAPI:
      enable: true
```
{% endraw %}

You can access a web interface at `kubeconfig.%publicDomainTemplate%` that allows you to generate `kubeconfig`
and use it for kubectl or any number of other Kubernetes API clients.

## Configuring kube-apiserver

You need to configure the kube-apiserver so that dashboard and kubeconfig-generator modules can work in the cluster. A dedicated  [control-plane-manager](../../modules/040-control-plane-manager/) module can help you with this.

{% offtopic title="kube-apiserver arguments that will be configured" %}

* --oidc-client-id=kubernetes
* --oidc-groups-claim=groups
* --oidc-issuer-url=https://dex.%addonsPublicDomainTemplate%/
* --oidc-username-claim=email

If self-signed certificates are used, dex will get one more argument. At the same time, the CA file will be mounted to the apiserver's pod:

* --oidc-ca-file=/etc/kubernetes/oidc-ca.crt
{% endofftopic %}
