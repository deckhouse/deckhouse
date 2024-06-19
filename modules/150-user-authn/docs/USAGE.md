---
title: "The user-authn module: usage"
---

## An example of the module configuration

The example shows the configuration of the 'user-authn` module in the Deckhouse Kubernetes Platform.

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
      enabled: true
```

{% endraw %}

## Configuring a provider

### GitHub

The example shows the provider's settings for integration with GitHub.

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

If the GitHub organization is managed by the client, go to `Settings` -> `Applications` -> `Authorized OAuth Apps` -> `<name of created OAuth App>` and request confirmation by clicking on `Send Request`. Then ask the client to confirm the request that will be sent to him by email.

### GitLab

The example shows the provider's settings for integration with GitLab.

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
* **cloud gitlab.com**: under the main project account, go to `User Settings` -> `Application` -> `New application` and specify the `https://dex.<modules.publicDomainTemplate>/callback` address as the `Redirect URI (Callback url)`; also, don't forget to set scopes `read_user`, `openid`;
* (for GitLab version starting with 16) enable the `Trusted`/`Trusted applications are automatically authorized on Gitlab OAuth flow` checkbox  when creating an application.

Paste the generated `Application ID` and `Secret` into the [DexProvider](cr.html#dexprovider) custom resource.

### Atlassian Crowd

The example shows the provider's settings for integration with Atlassian Crowd.

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

CROWD groups are specified in the lowercase format for the custom resource `DexProvider`.

### Bitbucket Cloud

The example shows the provider's settings for integration with Bitbucket Cloud.

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: bitbucket
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

Authentication through the OIDC provider requires registering a client (or "creating an application"). Please refer to the provider's documentation on how to do it (e.g., [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration)).

Paste the generated `clientID` and `clientSecret` into the [DexProvider](cr.html#dexprovider) custom resource.

#### Okta

The example shows the provider's settings for integration with Okta.

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

#### Blitz Identity Provider

Note that you must specify a URL to redirect the user after authorization when [registering the application](https://docs.identityblitz.com/latest/integration-guide/oidc-app-enrollment.html) with the Blitz Identity Provider.  When using `DexProvider`, you must specify `https://dex.<publicDomainTemplate>/`, where `publicDomainTemplate` is the cluster's DNS name template as [defined](https://deckhouse.io/documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) in the `global` module.

The example below shows the provider settings for integration with Blitz Identity Provider.

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: blitz
spec:
  displayName: Blitz Identity Provider
  oidc:
    basicAuthUnsupported: false
    claimMapping:
      email: email
      groups: your_claim # Claim for getting user groups, configured on the Blitz
    clientID: clientID
    clientSecret: clientSecret
    getUserInfo: true
    insecureSkipEmailVerified: true # Set to true if there is no need to verify the user's email
    insecureSkipVerify: false
    issuer: https://yourdomain.idblitz.ru/blitz
    promptType: consent 
    scopes:
    - profile
    - openid
    userIDKey: sub
    userNameKey: email
  type: OIDC
```

For the application logout to work correctly (the token being revoked so that re-authorization is required), set `login` as the value of the 'promptType` parameter.

To ensure granular user access to applications, you have to:
 
* Add the `allowedUserGroups` parameter to the `ModuleConfig` of the target application.
* Add the user to the groups (group names should be the same for Blitz and Deckhouse).

Below is an example for prometheus:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  settings:
    auth:
      allowedUserGroups:
        - adm-grafana-access
        - grafana-access
```

### LDAP

The example shows the provider's settings for integration with Active Directory.

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

## Configuring the OAuth2 client in Dex for connecting an application

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

After the `DexClient` custom resource is created, Dex will register a client with a `dex-client-myname@mynamespace` ID (**clientID**).

The client access password (**clientSecret**) will be stored in the secret object:
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

## An example of creating a static user

Create a password and enter its hash in the `password` field.

Use the command below to calculate the password hash:

```shell
echo "$password" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
```

Alternatively, you can use the [online service](https://bcrypt-generator.com/) to calculate the password hash.

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

## Example of adding a static user to a group

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

## How to set permissions for a user or group

Parameters in the custom resource [`ClusterAuthorizationRule`](../../modules/140-user-authz/cr.html#clusterauthorizationrule) are used for configuration.
