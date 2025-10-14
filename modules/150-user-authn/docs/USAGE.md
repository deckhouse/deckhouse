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
  version: 2
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
  displayName: Dedicated GitLab
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
* (for GitLab version starting with 16) enable the `Trusted`/`Trusted applications are automatically authorized on GitLab OAuth flow` checkbox when creating an application.

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

#### Keycloak

After selecting a `realm` to configure, adding a user in the [Users](https://www.keycloak.org/docs/latest/server_admin/index.html#assembly-managing-users_server_administration_guide) section, and creating a client in the [Clients](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide) section with [authentication](https://www.keycloak.org/docs/latest/server_admin/index.html#capability-config) enabled, which is required to generate the `clientSecret`, you need to perform the following steps:

* Create a `scope` named `groups` in the [Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes) section and assign it the predefined mapping `groups`. ("Client scopes" → "Client scope details" → "Mappers" → "Add predefined mappers")
* In the previously created client, add this `scope` in the [Client scopes tab](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) ("Clients" → "Client details" → "Client Scopes" → "Add client scope").
* In the "Valid redirect URIs", "Valid post logout redirect URIs", and "Web origins" fields of [the client configuration](https://www.keycloak.org/docs/latest/server_admin/#general-settings), specify `https://dex.<publicDomainTemplate>/*`, where `publicDomainTemplate` is a value of the [parameter](https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) in the `global` module config.

The example shows the provider's settings for integration with Keycloak.

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: keycloak
spec:
  type: OIDC
  displayName: My Company Keycloak
  oidc:
    issuer: https://keycloak.my-company.com/realms/myrealm # Use the name of your realm
    clientID: plainstring
    clientSecret: plainstring
    insecureSkipEmailVerified: true    
    getUserInfo: true
    scopes:
      - openid
      - profile
      - email
      - groups
```

If email verification is not enabled in KeyCloak, to properly use it as an identity provider, adjust the [`Client Scopes`](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) settings in one of the following ways:

* Delete the `Email verified` mapping ("Client Scopes" → "Email" → "Mappers").
  This is required for proper processing of the [`insecureSkipEmailVerified`](cr.html#dexprovider-v1-spec-oidc-insecureskipemailverified) field when it's set to `true` and for correct permission assignment to users with unverified emails.

* If you can't modify or delete the `Email verified` mapping, create a new Client Scope named `email_dkp` (or any other name) and add two mappings:
  * `email`: "Client Scopes" → `email_dkp` → "Add mapper" → "From predefined mappers" → `email`.
  * `email verified`: "Client Scopes" → `email_dkp` → "Add mapper" → "By configuration" → "Hardcoded claim". Specify the following fields:
    * "Name": `email verified`
    * "Token Claim Name": `emailVerified`
    * "Claim value": `true`
    * "Claim JSON Type": `boolean`

  After that, in the client registered for the DKP cluster in "Clients", change `Client scopes` from `email` to `email_dkp`.

  In the DexProvider resource, specify `insecureSkipEmailVerified: true` and in the `.spec.oidc.scopes` field, change the Client Scope name to `email_dkp` following the example:
  
  ```yaml
      scopes:
        - openid
        - profile
        - email_dkp
        - groups
  ```

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

Note that you must specify a URL to redirect the user after authorization when [registering the application](https://docs.identityblitz.com/latest/integration-guide/oidc-app-enrollment.html) with the Blitz Identity Provider.  When using `DexProvider`, you must specify `https://dex.<publicDomainTemplate>/`, where `publicDomainTemplate` is the cluster's DNS name template as [defined](https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) in the `global` module.

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

## Local Authentication

Local authentication provides user verification and access management with support for configurable password policies, two-factor authentication (2FA), and group management.  
The implementation complies with OWASP recommendations, ensuring reliable protection of access to the cluster and applications without requiring integration with external authentication systems.

### Creating a user

Create a password and enter its hash encoded in base64 in the `password` field.

Use the command below to calculate the password hash:

```shell
echo -n '3xAmpl3Pa$$wo#d' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
```

{% alert level="info" %}
If the `htpasswd` command is not available, install the appropriate package:

* `apache2-utils` — for Debian-based distributions.
* `httpd-tools` — for CentOS-based distributions.
* `apache2-htpasswd` — for ALT Linux.
{% endalert %}

Alternatively, you can use the [online service](https://bcrypt-generator.com/) to calculate the password hash.

Note that in the below example the [`ttl`](cr.html#user-v1-spec-ttl) parameter is set.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@yourcompany.com
  # echo -n '3xAmpl3Pa$$wo#d' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  password: 'JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H'
  ttl: 24h
```

{% endraw %}

### Adding a user to a group

Users can be grouped to manage access rights. Example manifest of the Group resource for a group:

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

Where `members` is a list of users belonging to the group.

### Password policy

Password policy settings allow controlling password complexity, rotation, and user lockout:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    passwordPolicy:
      complexityLevel: Fair
      passwordHistoryLimit: 10
      lockout:
        lockDuration: 15m
        maxAttempts: 3
      rotation:
        interval: "30d"
```

{% endraw %}

Field description:

* `complexityLevel`: Password complexity level.
* `passwordHistoryLimit`: Number of previous passwords stored in the system to prevent their reuse.
* `lockout`: Lockout settings after exceeding the limit of failed login attempts:
  * `lockout.maxAttempts`: Limit of allowed failed login attempts.
  * `lockout.lockDuration`: User lockout duration.
* `rotation`: Password rotation settings:
  * `rotation.interval`: Period for mandatory password change.

### Two-factor authentication (2FA)

2FA increases security by requiring a code from a TOTP authenticator application (for example, Google Authenticator) during login.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    staticUsers2FA:
      enabled: true
      issuerName: "awesome-app"
```

{% endraw %}

Field description:

* `enabled`: Enables or disables 2FA for all static users.
* `issuerName`: Name displayed in the authenticator application when adding an account.

{% alert level="info" %}
After enabling 2FA, each user must register in the authenticator application during their first login.
{% endalert %}

## How to set permissions for a user or group

Parameters in the custom resource [`ClusterAuthorizationRule`](../../modules/user-authz/cr.html#clusterauthorizationrule) are used for configuration.
