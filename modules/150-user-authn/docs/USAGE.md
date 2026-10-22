---
title: "The user-authn module: usage"
description: "Examples of configuring authentication providers, DexAuthenticator, DexClient, local users, password policy and 2FA in the user-authn module."
---

## An example of the module configuration

The example adds a direct access method to the API server for the kubeconfig that users generate in the Deckhouse web UI.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    kubeconfigGenerator:
    - id: direct
      masterURI: https://192.0.2.10:6443
      description: "Direct access to kubernetes API"
```

{% endraw %}

## Configuring a provider

In the examples below, `<DEX_DOMAIN>` is the Dex domain. It is the [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) value with `%s` replaced by `dex`, for example `dex.example.com`.

### Checking provider connectivity

The provider page in the Deckhouse web UI has a "Check connection" action. It creates a [DexProviderCheck](cr.html#dexprovidercheck) resource and waits for the `user-authn` controller to write the result to its `status`.

The check verifies that:

- the referenced [DexProvider](cr.html#dexprovider) exists and is enabled;
- the Dex discovery endpoint inside the cluster is reachable;
- the external provider endpoint is reachable.

For OIDC providers, the check also reads the discovery document and JWKS. For LDAP providers, it verifies TCP/TLS/StartTLS reachability and, when Kerberos is enabled, that the keytab Secret exists. The check also verifies that the provider accepts the client or service account credentials (GitHub, GitLab, Bitbucket Cloud, Crowd, OIDC, LDAP bind) and warns about disabled TLS verification and certificates that expire soon. The check does not perform a full login flow and does not validate a test user password.

To run the check without the web UI, create a DexProviderCheck:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProviderCheck
metadata:
  name: check-github
spec:
  providerName: github
```

Then read its status:

```shell
d8 k get dexproviderchecks check-github -o yaml
```

### Limiting the identities of a provider

A user who signs in through a DexProvider gets the email and the groups that the identity provider reports. The email is the Kubernetes username. So the provider gives its users every permission already granted in the cluster to these emails and groups.

To limit them, use [`spec.allowedIdentities`](cr.html#dexprovider-v1-spec-allowedidentities). The `emails` and `emailDomains` lists limit the email, and Dex refuses sign-in for any other email. The `groups` list limits the groups. Dex keeps only the listed groups in the token and refuses sign-in if none is left.

The limits apply to every provider type, in addition to the group filters of the type (`oidc.allowedGroups`, `gitlab.groups`, `crowd.groups`, `bitbucketCloud.teams` without `includeTeamGroups`, `github.orgs[].teams` in every organization, `saml.allowedGroups` with `filterGroups: true`). Group names are compared exactly. Nesting of Group objects does not extend the list, because a token from an external provider carries only the groups that the provider reports.

Dex checks the limits at sign-in, at every refresh-token request and at Basic Authentication to the API server through an LDAP provider. Dex revokes a refresh token that falls outside the limits. Narrowing the limits therefore ends the existing sessions of the users it excludes, and they have to sign in again. For a Crowd or OIDC provider, basic-auth-proxy checks the credentials with the provider directly, so the limits do not apply to Basic Authentication.

The same limits define who can create or change the provider. You must be able to assign every role already granted to the emails and groups that the provider can report. A provider without an email limit can report the email of a SuperAdmin, so only a SuperAdmin can create or change it. A Crowd or OIDC provider with `enableBasicAuth: true` counts as a provider without an email limit, because Basic Authentication through it bypasses the limits. Only a SuperAdmin can create such a provider or turn on `enableBasicAuth` in an existing one.

Example of a provider limited by an email domain and a group:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: contractors
spec:
  type: OIDC
  displayName: Contractors
  oidc:
    issuer: https://idp.contractor.example
    clientID: dex
    clientSecret: secret
  allowedIdentities:
    emailDomains: [contractor.example]
    groups: [contractors]
```

The following changes are not checked: rotating `clientSecret` or `bindPW`, changing `displayName`, disabling the provider, narrowing the limits, applying the same manifest again, and deleting the provider. Any other change is checked as a new provider, including enabling a disabled provider.

If the check fails, the error lists the roles that the provider could reach, for example:

```text
dexproviders.deckhouse.io "corp": the provider can assert identities that already carry roles [user-authz:super-admin]; the requester's can-assign range is basic<=ClusterAdmin and does not cover them. Narrow spec.allowedIdentities (emails, emailDomains, groups) or ask a SuperAdmin
```

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
  # Optional: disable the provider without deleting CR
  # enabled: false
  github:
    clientID: plainstring
    clientSecret: plainstring
```

In your GitHub organization, register an OAuth application. To do this, go to `Settings` → `Developer settings` → `OAuth Apps` → `New OAuth App` and set `Authorization callback URL` to `https://<DEX_DOMAIN>/callback`.

Paste the generated `Client ID` and `Client Secret` into the [DexProvider](cr.html#dexprovider).

If the organization restricts third-party applications, request approval in `Settings` → `Applications` → `Authorized OAuth Apps` → `<APPLICATION_NAME>` → `Send Request`, and ask an organization owner to approve the request.

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

Create a new application in GitLab.

To do this:

* **Self-hosted**: Go to `Admin area` → `Application` → `New application`, set `Redirect URI (Callback URL)` to `https://<DEX_DOMAIN>/callback` and select the `read_user` and `openid` scopes.
* **Cloud GitLab.com**: Under the main project account, go to `User Settings` → `Application` → `New application`, set `Redirect URI (Callback URL)` to `https://<DEX_DOMAIN>/callback` and select the `read_user` and `openid` scopes.
* **GitLab 16 and later**: Enable the `Trusted`/`Trusted applications are automatically authorized on GitLab OAuth flow` checkbox when creating the application.

Paste the generated `Application ID` and `Secret` into the [DexProvider](cr.html#dexprovider).

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
    groups:
    - administrators
    - users
```

Create a new `Generic` application in the corresponding Atlassian Crowd project. To do this, go to `Applications` → `Add application`.

Paste the generated `Application Name` and `Password` into the [DexProvider](cr.html#dexprovider).

Specify Crowd groups in the DexProvider in lowercase.

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

Create a new OAuth consumer in the Bitbucket team menu. To do this, go to `Settings` → `OAuth consumers` → `New application`, set `Callback URL` to `https://<DEX_DOMAIN>/callback` and allow access for `Account: Read` and `Workspace membership: Read`.

Paste the generated `Key` and `Secret` into the [DexProvider](cr.html#dexprovider).

### OIDC (OpenID Connect)

Authentication through an OIDC provider requires registering a client (or "creating an application"). Register it as the provider's documentation describes (for example, [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration)).

Paste the generated `clientID` and `clientSecret` into the [DexProvider](cr.html#dexprovider).

#### Keycloak

After selecting a `realm` to configure, adding a user in the [Users](https://www.keycloak.org/docs/latest/server_admin/index.html#assembly-managing-users_server_administration_guide) section, and creating a client in the [Clients](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide) section with [authentication](https://www.keycloak.org/docs/latest/server_admin/index.html#capability-config) enabled, which is required to generate the `clientSecret`, you need to perform the following steps:

* Create a `scope` named `groups` in the [Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes) section and assign it the predefined mapping `groups`. ("Client scopes" → "Client scope details" → "Mappers" → "Add predefined mappers")
* In the previously created client, add this `scope` in the [Client scopes tab](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) ("Clients" → "Client details" → "Client Scopes" → "Add client scope").
* In the "Valid redirect URIs", "Valid post logout redirect URIs", and "Web origins" fields of [the client configuration](https://www.keycloak.org/docs/latest/server_admin/#general-settings), specify `https://<DEX_DOMAIN>/*`.

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

If email verification is not enabled in Keycloak, to properly use it as an identity provider, adjust the [`Client Scopes`](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) settings in one of the following ways:

* Delete the `Email verified` mapping ("Client Scopes" → "Email" → "Mappers").
  This is required for proper processing of the [`insecureSkipEmailVerified`](cr.html#dexprovider-v1-spec-oidc-insecureskipemailverified) field when it's set to `true` and for correct permission assignment to users with unverified emails.

* If you can't modify or delete the `Email verified` mapping, create a new Client Scope named `email_dkp` (or any other name) and add two mappings:
  * `email`: "Client Scopes" → `email_dkp` → "Add mapper" → "From predefined mappers" → `email`.
  * `email verified`: "Client Scopes" → `email_dkp` → "Add mapper" → "By configuration" → "Hardcoded claim". Specify the following fields:
    * "Name": `email verified`
    * "Token Claim Name": `emailVerified`
    * "Claim value": `true`
    * "Claim JSON Type": `boolean`

  After that, in the client registered for the Deckhouse Platform cluster in "Clients", change `Client scopes` from `email` to `email_dkp`.

  In the DexProvider resource, specify `insecureSkipEmailVerified: true` and in the `.spec.oidc.scopes` field, change the Client Scope name to `email_dkp` following the example:
  
  ```yaml
      scopes:
        - openid
        - profile
        - email_dkp
        - groups
  ```

{% alert level="warning" %}
With `insecureSkipEmailVerified: true`, Dex accepts unverified email addresses. Permissions are granted by email, so use this parameter only if the identity provider does not let users set an arbitrary email address.
{% endalert %}

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
    getUserInfo: true
```

#### Blitz Identity Provider

When you [register the application](https://docs.identityblitz.com/latest/integration-guide/oidc-app-enrollment.html) in the Blitz Identity Provider, specify `https://<DEX_DOMAIN>/callback` as the URL to redirect the user to after authorization.

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

For the application logout to work correctly (the token is revoked and the user must sign in again), set the `promptType` parameter to `login`.

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
    rootCAData: |
      -----BEGIN CERTIFICATE-----
      MIIFaDC...
      -----END CERTIFICATE-----

    bindDN: cn=dex-readonly,cn=users,dc=example,dc=com
    bindPW: <PASSWORD>
    
    usernamePrompt: Email Address

    enableBasicAuth: true

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

To configure authentication, create a read-only service account in LDAP. Specify its DN and password in the `bindDN` and `bindPW` fields of the [DexProvider](cr.html#dexprovider). Enter the password in `bindPW` in plain text. Hashed passwords are not supported.

If LDAP allows anonymous read access, you can omit `bindDN` and `bindPW`.

#### Configuring Basic Authentication

To enable Basic Authentication for the Kubernetes API using LDAP credentials:

1. Ensure that the [`apiserver.publishAPI`](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) parameter is enabled in the `control-plane-manager` module configuration.
1. Set [`enableBasicAuth: true`](cr.html#dexprovider-v1-spec-ldap-enablebasicauth) in the LDAP DexProvider.

{% alert level="info" %}
Only one provider in the cluster can have [`enableBasicAuth`](cr.html#dexprovider-v1-spec-ldap-enablebasicauth) enabled.
{% endalert %}

After configuration, users can access the Kubernetes API via `kubectl`, using their LDAP username and password.

Example kubeconfig for the user:

```yaml
apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: https://api.example.com
    # Path to CA certificate or insecure-skip-tls-verify: true
    certificate-authority: /path/to/ca.crt
users:
- name: ldap-user
  user:
    username: janedoe@example.com
    password: userpassword
contexts:
- name: default
  context:
    cluster: my-cluster
    user: ldap-user
current-context: default
```

#### Kerberos (SPNEGO) SSO for LDAP

Dex supports passwordless Kerberos (SPNEGO) flow for the LDAP connector. When enabled, a browser that trusts the Dex host will send `Authorization: Negotiate …` and Dex will validate the Kerberos ticket using a service keytab, skip the login form, map the Kerberos principal to an LDAP username, query groups, and complete the OIDC flow.

Minimal example extending the LDAP provider spec:

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
    bindDN: cn=dex-readonly,cn=users,dc=example,dc=com
    bindPW: <PASSWORD>
    userSearch:
      baseDN: cn=Users,dc=example,dc=com
      username: sAMAccountName
      idAttr: uid
      emailAttr: mail
      nameAttr: cn
    groupSearch:
      baseDN: cn=Users,dc=example,dc=com
      nameAttr: cn
      userMatchers:
      - userAttr: uid
        groupAttr: memberUid
    kerberos:
      enabled: true
      keytabSecretName: dex-kerberos-keytab   # Secret in d8-user-authn with key 'krb5.keytab'
      expectedRealm: EXAMPLE.COM              # optional, case-insensitive match
      usernameFromPrincipal: SAMAccountName   # Localpart|SAMAccountName|UserPrincipalName
      fallbackToPassword: false               # default false; if true, render form when header missing/invalid
```

Notes:

- The Secret `dex-kerberos-keytab` must exist in the `d8-user-authn` namespace and have a data key named exactly `krb5.keytab`.
- A single Dex Pod can serve multiple LDAP+Kerberos providers. Each provider mounts its own keytab; a shared `krb5.conf` is not required (Dex validates tickets offline using the keytab).
- A DexProvider with `enabled: true` must have `keytabSecretName` set, otherwise the object is rejected when saved. To turn Kerberos off and keep its settings, set `enabled: false`.

### SAML

The example shows the provider's settings for integration with a SAML 2.0 Identity Provider (e.g., AD FS, Okta, Keycloak).

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: saml-provider
spec:
  type: SAML
  displayName: Corporate SAML
  saml:
    ssoURL: https://saml-idp.example.com/saml/sso
    rootCAData: |
      -----BEGIN CERTIFICATE-----
      MIIFaDC...
      -----END CERTIFICATE-----
    entityIssuer: https://dex.example.com/callback
    ssoIssuer: https://saml-idp.example.com
    usernameAttr: name
    emailAttr: email
    groupsAttr: groups
    nameIDPolicyFormat: persistent
```

To configure the SAML Identity Provider:

1. Register Dex as a Service Provider (SP) with your identity provider using the following settings:
   - **ACS URL (Assertion Consumer Service)**: `https://<DEX_DOMAIN>/callback`
   - **Entity ID**: `https://<DEX_DOMAIN>/callback`
   - **Name identifier format**: `persistent` or `emailAddress`

1. Configure attribute mapping in the identity provider to send `email`, `name` (username), and `groups` attributes in the SAML assertion.

1. Export the identity provider signing certificate and specify it in the `rootCAData` field of the DexProvider resource.

{% alert level="info" %}
SAML does not natively support refresh tokens. Dex caches the user identity from the initial SAML assertion and returns it on subsequent refresh requests. The session lifetime is limited by the [`refreshTokenAbsoluteLifetime`](configuration.html#parameters-refreshtokenabsolutelifetime) module parameter.
{% endalert %}

## Configuring the OAuth2 client in Dex for connecting an application

This configuration is suitable for applications that can perform OAuth2 authentication themselves, without an OAuth2 proxy.
The [DexClient](cr.html#dexclient) lets such applications use Dex.

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

The DexClient group and email allow-lists are enforced on login and on every refresh-token request. Changing the lists denies further refresh for sessions that no longer match; those users must sign in again.

After the DexClient is created, Dex registers a client with the `dex-client-myname@mynamespace` ID (**clientID**).

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

### Granting an application access to the Kubernetes API

A [DexClient](cr.html#dexclient) or a [DexAuthenticator](cr.html#dexauthenticator) can be allowed to obtain tokens that the Kubernetes API server accepts. To do that, set one of the following annotations on the object to `"true"`:

* `dexclient.deckhouse.io/allow-access-to-kubernetes` on a DexClient.
* `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` on a DexAuthenticator.

{% raw %}

The following is a DexClient configuration example:

```yaml
apiVersion: deckhouse.io/v1
kind: DexClient
metadata:
  name: myname
  namespace: mynamespace
  annotations:
    dexclient.deckhouse.io/allow-access-to-kubernetes: "true"
spec:
  redirectURIs:
  - https://app.example.com/callback
```

{% endraw %}

The annotation registers the client as a trusted peer of the privileged `kubernetes` OAuth2 client, allowing it to request ID tokens intended for the API server. In such a token, the username is determined by the `email` claim and groups by the `groups` claim. As a result, the application accesses the API server on behalf of the authenticated user and with the permissions granted to that user.

{% alert level="warning" %}
Access granted this way applies at the cluster level, even though DexClient and DexAuthenticator are namespaced objects. Therefore, only a subject allowed to `create` the `dexclients/allow-access-to-kubernetes` subresource (for DexClient) or the `dexauthenticators/allow-access-to-kubernetes` subresource (for DexAuthenticator) in the namespace of the object can add the annotation or change its value to `"true"`. The `d8:system-capability:user-authn:edit` role and the `ClusterAdmin` access level carry this permission. To grant it in one namespace, create a Role with the `create` verb on these subresources and bind it with a RoleBinding in that namespace. Permissions to create DexClient or DexAuthenticator objects are not sufficient.

Adding the annotation is restricted regardless of its value, including `"false"`. This is required for compatibility with Deckhouse Platform versions earlier than 1.77, where access is granted based on the presence of the annotation regardless of its value. If access to the Kubernetes API server is not required, do not add the annotation.

The restriction does not apply to resources that already have the annotation. A user with permissions to modify such a resource can continue to modify it, including removing the annotation or changing its value to disable access.
{% endalert %}

## Local Authentication

Local authentication provides user verification and access management with support for configurable password policies, two-factor authentication (2FA), and group management.  
In the module parameters, you can configure password complexity, password history, password rotation, lockout after failed sign-in attempts and per-IP rate limiting of failed attempts. No external authentication system is required.

### Creating a user

The recommended way to create a local user is the [`d8 iam user create`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-create) command. It supports interactive password entry, automatic password generation, group assignment, and time-to-live (TTL) for temporary users.

Examples:

Interactive password prompt (default when stdin is a terminal):

```shell
d8 iam user create anton --email anton@abc.com
```

Automatically generate a password (the generated password is shown in the command output once):

```shell
d8 iam user create anton --email anton@abc.com --generate-password
```

Read the password from stdin (for CI/CD pipelines):

```shell
echo "s3cret" | d8 iam user create anton --email anton@abc.com --password-stdin
```

Use a pre-computed bcrypt hash (e.g. from htpasswd):

```shell
d8 iam user create anton --email anton@abc.com --password-hash '$2y$10$abcdef...'
```

Create the user and add them to groups (auto-creating groups if missing):

```shell
d8 iam user create anton --email anton@abc.com --generate-password --member-of admins --create-groups
```

Create a temporary user with a TTL:

```shell
d8 iam user create anton --email anton@abc.com --generate-password --ttl 24h
```

Preview the manifest without applying it:

```shell
d8 iam user create anton --email anton@abc.com --generate-password --dry-run -o yaml
```

As an alternative, you can create the [User](cr.html#user) resource manually. Create a password and enter its hash encoded in base64 in the `password` field. The email address must be in lowercase.

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

[Authorization rules](/modules/user-authz/cr.html#authorizationrule) grant permissions to users based on the email address from the issued token. Therefore, a User resource cannot be created if its `spec.email` value matches a `User` subject in an existing [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) or [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) resource. This prevents permissions from being unintentionally granted to a new user.

If the match is intentional, for example, if the authorization rule was created in advance, use a different email address or add the `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` annotation to the User resource. The annotation acknowledges the name collision only. Assigning the grant still requires covering permissions or a can-assign range that includes the target roles.

Email addresses are converted to lowercase when matched, as this is how they are stored in the token. For example, `Admin@Example.com` matches the `admin@example.com` subject.

The restriction does not apply to existing users whose email addresses match subjects in authorization rules. Such users continue to work, and a warning is issued when a match is detected.

Consider this restriction when managing users and authorization rules declaratively. If a user and the corresponding rule are created at the same time, the rule may be created first, causing creation of the User resource to be rejected. To allow such a match, add the `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` annotation to the User manifest.

Deleting a user does not automatically remove the corresponding subject from the authorization rule. As long as the subject remains in the rule, permissions continue to be granted to the specified email address, and a warning is issued that no corresponding user exists. If a user with the same email address is created later, that user receives these permissions. If this behavior is not required, remove the corresponding subject from the authorization rule.

### Deleting a user

To delete a local user, use the [`d8 iam user delete`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-delete) command. By default, it also removes the user from all [Group](cr.html#group) resources they belong to.

Examples:

Delete the user (and automatically remove them from all groups):

```shell
d8 iam user delete anton
```

Delete the user but keep their references in groups:

```shell
d8 iam user delete anton --keep-memberships
```

### Local user operations

Password reset, 2FA reset, and lock/unlock operations are performed via the [UserOperation](cr.html#useroperation) resource. The `initiatorType` field indicates who initiated the operation: an administrator (`admin`) or the user (`self`).

{% alert level="warning" %}
Do not change `User.spec.password` after the User is created (for example, with `d8 k edit`). The field is immutable and does not update Dex credentials. Reset passwords with a UserOperation or `d8 iam user reset-password`.
{% endalert %}

#### Administrative operations

Use the `d8 iam user` commands for administrative actions on local users. They create a UserOperation resource with `initiatorType: admin`, wait for the operation to complete, and print the result.

You can delete or recreate a local user, or run a UserOperation (`ResetPassword`, `Reset2FA`, `Lock`, `Unlock`) against them, only if you can assign every role already granted to the user's email or groups (covering permissions or an explicit can-assign range). `initiatorType: self` does not bypass this check. The same rule applies to DexProvider objects. For details, see [Limiting the identities of a provider](#limiting-the-identities-of-a-provider).

After an administrator resets the password with `ResetPassword`, the user signs in with that password once. Dex then asks the user to set a new password that complies with the password policy.

Interactive password reset example:

```shell
d8 iam user reset-password admin
```

Example of reading the new password from stdin:

```shell
echo "N3wPa$$wo#d" | d8 iam user reset-password admin --password-stdin
```

Example of generating a new password automatically:

```shell
d8 iam user reset-password admin --generate-password
```

If the password is already hashed, pass the bcrypt hash without Base64 encoding:

```shell
d8 iam user reset-password admin --password-hash '$2y$10$abcdef...'
```

2FA reset example:

```shell
d8 iam user reset2fa admin
```

Example of locking a user for 30 minutes:

```shell
d8 iam user lock admin 30m
```

The `d8 iam user lock` command locks a local user and accepts a duration in seconds, minutes or hours, for example `30s`, `10m` or `1h`. To lock a user for several days (`lock.for: 7d`) or permanently (`lock.for: permanent`), create a UserOperation as described in [Creating UserOperation manually](#creating-useroperation-manually).

User unlock example:

```shell
d8 iam user unlock admin
```

By default, commands wait for the operation to complete. To only create a UserOperation and print its name, use the `--wait=false` flag.

#### Self-service password reset

A local user can reset their own password in the Deckhouse Platform authentication interface. This creates a UserOperation resource with `type: ResetPassword` and `initiatorType: self`.

Self-service password reset is available only for local accounts (the built-in `Local` connector). Users who sign in through external authentication providers must contact the administrator of the corresponding system.

When a user resets their password, the new password must comply with the password policy, and the user's active sessions are terminated — re-authentication is required.

#### Creating UserOperation manually

When the `d8 iam user` CLI is unavailable (e.g. in CI/CD, GitOps, or automation pipelines), you can create a [UserOperation](cr.html#useroperation) resource directly. Use `apiVersion: deckhouse.io/v1` and set `initiatorType: admin`.

Example — reset a local user's password (the `newPasswordHash` contains a bcrypt hash without Base64 encoding; the module encodes it automatically):

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: reset-password-admin
spec:
  user: admin
  type: ResetPassword
  initiatorType: admin
  resetPassword:
    newPasswordHash: "$2y$10$..."
```

Example — lock a local user for 1 hour:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: lock-admin-1h
spec:
  user: admin
  type: Lock
  initiatorType: admin
  lock:
    for: "1h"
```

Example — permanent lock:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: lock-admin-permanent
spec:
  user: admin
  type: Lock
  initiatorType: admin
  lock:
    for: "permanent"
```

Example — unlock:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: unlock-admin
spec:
  user: admin
  type: Unlock
  initiatorType: admin
```

Example — reset 2FA:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: reset-2fa-admin
spec:
  user: admin
  type: Reset2FA
  initiatorType: admin
```

#### Operations on external users (LDAP/Crowd)

For users authenticated through external providers (LDAP, Atlassian Crowd), use the [`spec.target`](cr.html#useroperation-v1-spec-target) field instead of `spec.user`. Only the `Lock` and `Unlock` operation types are supported for external users.

A lock applies only to sign-ins through the provider set in `connectorID`. It does not block a local user or a user of another provider with the same email. The operation fails if the user has never signed in through this provider.

Example — lock an external user by `connectorID` + email for 30 minutes:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: lock-external-user
spec:
  target:
    connectorID: my-ldap
    email: jane.doe@example.org
  type: Lock
  initiatorType: admin
  lock:
    for: "30m"
```

#### UserOperation lifecycle and side effects

Using a UserOperation object has the following specifics:

- A UserOperation is a **single-use** object. After creation, the module processes it and writes the result to `status.phase` (`Succeeded` or `Failed`).
- Completed operations are **automatically deleted** after 24 hours.
- A UserOperation runs once. Changes to its spec after creation are ignored.
- To perform a new action, create a new UserOperation.

{% alert level="warning" %}
For a local user, the `ResetPassword`, `Reset2FA`, and `Lock` operations delete the user's Dex OfflineSessions and RefreshToken objects, so the user must sign in again. A `Lock` of an external user (`spec.target`) blocks sign-in and token refresh until the lock ends.
{% endalert %}

#### Checking operation status

To check the operation status, follow these steps:

1. Get the list of all operations:

   ```shell
   d8 k get useroperations
   ```

1. Get the full status of an operation:

   ```shell
   d8 k get useroperation <name> -o yaml
   ```

1. Get only the completion status:

   ```shell
   d8 k get useroperation <name> -o jsonpath='{.status.phase}'
   ```

#### Viewing account state

The module keeps a [UserAccount](cr.html#useraccount) object for each local user and for each user who has signed in through an external provider. It shows whether the account is locked, until when, and how many failed login attempts were counted. Do not create or edit UserAccount objects.

To list the accounts, run:

```shell
d8 k get useraccounts
```

#### Automatic lockout

If the number of failed sign-in attempts exceeds [`passwordPolicy.lockout.maxAttempts`](configuration.html#parameters-passwordpolicy-lockout-maxattempts), Dex locks the user for [`lockout.lockDuration`](configuration.html#parameters-passwordpolicy-lockout-lockduration) and then unlocks them automatically. For LDAP and Crowd users, this works only if the provider type is listed in [`lockout.applyToConnectors`](configuration.html#parameters-passwordpolicy-lockout-applytoconnectors). Dex does not create a UserOperation for this lockout. To unlock the user earlier, run `d8 iam user unlock` or create a UserOperation of type `Unlock`.

### Adding a user to a group

Users can be grouped to manage access rights. The recommended way to manage groups is the [`d8 iam group`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-group) command.

Examples:

Create a group:

```shell
d8 iam group create admins
```

Add a user to a group:

```shell
d8 iam group add-member admins user anton
```

Add a nested group:

```shell
d8 iam group add-member admins group devops
```

Remove a user from a group:

```shell
d8 iam group remove-member admins user anton
```

Delete a group:

```shell
d8 iam group delete admins
```

As an alternative, you can create the [Group](cr.html#group) resource manually. Example manifest of the Group resource for a group:

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

Where `members` is a list of members: `kind: User` with `name` = `User.metadata.name`, or a nested `kind: Group` with `name` = `Group.spec.name` (the name in the token, not `metadata.name`).

Nested groups must not form a cycle. A change to a Group resource is rejected if after it a group becomes its own member, directly or through other groups. If the hierarchy already contains a cycle, changes that do not create a new one are accepted with a warning that shows the path of the cycle, where each group is a member of the previous one. To break the cycle, remove one of these groups from the members of the previous group.

The group name is stored in the issued token without modification. It is indistinguishable from a group name received from an external authentication provider. Therefore, a Group resource cannot be created if its `spec.name` value matches a `Group` subject in an existing [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) or [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) resource. This prevents permissions from being unintentionally granted to members of a new group.

If the match is intentional, use a different group name or add the `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` annotation to the Group resource.

Unlike email addresses, group names are not converted to lowercase when matched. Therefore, group names that differ only in letter case are considered different.

The restriction does not apply to existing groups whose names match subjects in authorization rules. Such groups continue to work, and a warning is issued when a match is detected.

Consider this restriction when managing groups and authorization rules declaratively. If a group and the corresponding rule are created at the same time, the rule may be created first, causing creation of the Group resource to be rejected. To allow such a match, add the `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` annotation to the Group manifest.

Deleting a group does not automatically remove the corresponding subject from the authorization rule. As long as the subject remains in the rule, permissions continue to be granted to a group with the specified name. If a group with the same name is created later, its members receive these permissions. If this behavior is not required, remove the corresponding subject from the authorization rule.

### Viewing users and groups

Use the [`d8 iam get`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-get) and [`d8 iam list`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-list) commands to view users, groups, and their effective access (groups, grants, and access level).

Examples:

List all users with effective access:

```shell
d8 iam list users
```

Details for a specific user (groups, grants, access level):

```shell
d8 iam get user anton
```

List all groups:

```shell
d8 iam list groups
```

Details for a specific group (members, grants):

```shell
d8 iam get group admins
```

### Password policy

Password policy settings allow controlling password complexity, rotation, and user lockout:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
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

* `complexityLevel`: Password complexity level. One of `None`, `Low`, `Fair`, `Good`, `Excellent` or `Custom`.
* `custom`: Custom complexity rules (used only with `complexityLevel: Custom`):
  * `custom.minLength`: Minimum number of characters in a password.
  * `custom.specialCharacters`: If `true`, require at least one special character.
  * `custom.numbers`: If `true`, require at least one digit.
  * `custom.capitalized`: If `true`, require at least one uppercase letter.
  * `custom.repeatedChars`: If `true`, forbid more than 2 identical characters in a row.
* `passwordHistoryLimit`: Number of previous passwords stored in the system to prevent their reuse.
* `lockout`: Lockout settings after exceeding the limit of failed login attempts:
  * `lockout.maxAttempts`: Limit of allowed failed login attempts.
  * `lockout.lockDuration`: User lockout duration.
  * `lockout.applyToConnectors`: External provider types the lockout also applies to: `LDAP`, `Crowd`. Local users are always covered.
* `rotation`: Password rotation settings:
  * `rotation.interval`: Period for mandatory password change.

To limit failed login attempts per client IP address, use the [`rateLimit`](configuration.html#parameters-ratelimit) parameter.

Example with a custom complexity rules:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    passwordPolicy:
      complexityLevel: Custom
      custom:
        minLength: 10
        specialCharacters: true
        numbers: false
        capitalized: true
        repeatedChars: false
      passwordHistoryLimit: 10
```

{% endraw %}

### Two-factor authentication (2FA)

2FA increases security by requiring a code from a `TOTP` authenticator application (for example, Google Authenticator) during login.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
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

### Assigning permissions to a user or group

To grant permissions to a user or a group, bind one of the `user-authz` roles with a RoleBinding or ClusterRoleBinding, using the user's email or the group name as the subject. See the [granular role-based model](/modules/user-authz/#granular-role-based-model).
