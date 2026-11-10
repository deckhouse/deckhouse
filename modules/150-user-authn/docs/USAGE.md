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

### Prefix for the groups of a provider

The cluster knows the groups a provider reports by their names, so a rule for the group `admins` applies to every member of any group `admins`. When two providers report a group of the same name, for example an LDAP directory and a GitLab, the members of both get the rights of the rule.

[`spec.groupPrefix`](cr.html#dexprovider-v1-spec-groupprefix) of a DexProvider puts a prefix in front of every group the provider asserts, whatever its type. With `groupPrefix: corp-ldap:`, the group `admins` of the directory is `corp-ldap:admins` in the tokens, in the rules (the subjects of the AuthorizationRule objects) and in the `allowedGroups` of DexClient and DexAuthenticator.
A prefix keeps the groups of the providers apart only when every provider that can emit such names has its own prefix. A provider without a prefix passes the group names as they are (OIDC, a GitHub team named `org:team`, GitLab, SAML and the local groups) and can emit a name that starts with the prefix. When the rules rely on prefixed groups, set a prefix on every provider.

The following provider prefixes the groups of an LDAP directory with `corp-ldap:`:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: corp-ldap
spec:
  type: LDAP
  displayName: Corp
  groupPrefix: "corp-ldap:"
  allowedIdentities:
    groups:
    - "corp-ldap:admins"
    - "corp-ldap:developers"
  ldap:
    host: ldap.example.com:636
    userSearch:
      baseDN: ou=people,dc=example,dc=com
      username: mail
      idAttr: DN
      emailAttr: mail
```

Keep the following in mind when you set a prefix:

* There is no prefix by default. The prefix renames the groups, and the rules written for the old names stop applying to the users of the provider and so would lock them out. An existing provider takes the prefix only when you set it. Rewrite the rules first, then set the prefix.
* Rewrite the subjects of ClusterAuthorizationRule and AuthorizationRule, the groups of ClusterRoleBinding and RoleBinding, `auth.allowedUserGroups` in the ModuleConfig of `openvpn`, `documentation`, `istio`, `cilium-hubble` and `deckhouse-tools` (those modules render a DexAuthenticator from the setting), `allowedIdentities.groups` of the DexProvider and `allowedGroups` of DexAuthenticator and DexClient.
* After the prefix is set or changed, the users of the provider sign in again, and the new tokens carry the prefix.
* The access and ID tokens already issued keep the old names only until they expire. A rule that still names a group without the prefix, including a cluster-admin rule, stops matching as soon as those tokens are gone.
* A token that a Dex pod still running the old configuration issues later than 30 minutes after the change can keep the old names. The rules rewritten for the prefix do not match them.
* The lists that name the groups of Dex take the names with the prefix: `allowedIdentities.groups` of the provider, and `allowedGroups` of a DexClient or a DexAuthenticator. Every name in `allowedIdentities.groups` must start with `groupPrefix`, otherwise the API refuses the manifest, because Dex adds the prefix before it compares.
* The group settings of the provider type (for example, `oidc.allowedGroups`, `gitlab.groups`, `crowd.groups` and `ldap.groupSearch`) are about the groups of the identity provider and take its names, without the prefix.
* A group that already starts with the prefix keeps its name.
* The prefix ends with a colon, as in `corp-ldap:`, and differs from the prefix of every other DexProvider and from the group prefixes of every TokenExchangeProvider. Neither of two prefixes may start with the other. The admission webhooks refuse a prefix that breaks this, whichever of the objects is created or changed last.
* The prefix cannot start with `system:`, which the Kubernetes API server reserves, or with `peering:`, which is reserved for the ServiceAccounts of the `d8-peering` namespace. It cannot be set together with `enableBasicAuth` of an `OIDC` or `Crowd` provider, because basic-auth-proxy checks the credentials with such a provider directly and passes its groups as they are, so the prefix would not apply to them. Basic Authentication of an `LDAP` provider goes through Dex and gets the prefix.
* With a prefix, the provider can assert only groups that start with it. Therefore the check of who may create or change the provider (["Limiting the identities of a provider"](#limiting-the-identities-of-a-provider)) counts, of the roles granted to groups, only the roles of the groups with this prefix.

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

#### Nested groups

By default, Dex adds to the token only the groups of which the user is a direct member. A group that contains the user's group is not added. To add the groups of the higher levels, set `recursionGroupAttr` in `userMatchers` to the attribute of a group that stores its members. Dex then looks for the groups that contain each found group, level by level, and adds them to the groups of the user. A cycle of groups ends the search.

The following `groupSearch` adds the groups of every level through the `member` attribute:

```yaml
    groupSearch:
      baseDN: cn=Users,dc=example,dc=com
      filter: "(objectClass=group)"
      userMatchers:
      - userAttr: DN
        groupAttr: member
        recursionGroupAttr: member
      nameAttr: cn
```

Dex makes one more search for every group of every level at each sign-in and each refresh of the tokens. Keep `baseDN` and `filter` of `groupSearch` narrow, and expect a longer sign-in on a deep hierarchy.

#### Signing in with one of several attributes

`userSearch.username` names one attribute that the entered username is compared with. To let users sign in with a login or an email, list the attributes in `userSearch.usernames` instead of `username` (set one of the two parameters). Dex searches for an entry in which any of them matches. A sign-in is refused if the username matches more than one entry.

The following `userSearch` lets users sign in with `sAMAccountName` or `userPrincipalName`:

```yaml
    userSearch:
      baseDN: cn=Users,dc=example,dc=com
      filter: "(objectClass=person)"
      usernames: [sAMAccountName, userPrincipalName]
      idAttr: DN
      emailAttr: userPrincipalName
      nameAttr: cn
```

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

The principal mapped to a username is compared with every attribute listed in `userSearch.usernames` (or with `username`, when that is set), with the same filter as a password sign-in.
With `usernameFromPrincipal: UserPrincipalName`, the principal is compared as `user@REALM`, with `Localpart` or `SAMAccountName` as the part before `@`. The value must match an attribute of the user's own entry.
List only attributes whose values are unique in the directory and consistent with `usernameFromPrincipal`. If a value of another entry equals the principal, SSO stops working for that user (`multiple results`). A principal that matches none of the attributes of its own entry is matched to the entry of another user. The password is not checked on this path.

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

In the authorization code flow, Dex accepts only the `S256` method of Proof Key for Code Exchange (PKCE). An application that sends a `code_challenge` with `code_challenge_method=plain`, or without `code_challenge_method` (the OAuth2 default is `plain`), is sent back to its redirect URI with the `invalid_request` error "Unsupported PKCE challenge method". Such an application has to switch to `S256`.
PKCE stays optional, and an application that sends no `code_challenge` signs in without it. The device flow (`/device/code`) is not affected by this check. Dex accepts `plain` for it, because the device code itself serves as the secret.

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

### Limiting the providers of an application

By default, the sign-in page of an application offers every DexProvider of the cluster and the built-in users ([User](cr.html#user)). To offer only some of them, list their names in `spec.allowedProviders` of the [DexClient](cr.html#dexclient-v1-spec-allowedproviders) or the [DexAuthenticator](cr.html#dexauthenticator-v1-spec-allowedproviders). The name of a DexProvider is its `metadata.name`. The built-in users are `local`.

The following DexAuthenticator lets users sign in to the application only through the `corp-ldap` provider and as built-in users:

```yaml
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: admin-panel
  namespace: team
spec:
  applicationDomain: panel.example.com
  applicationIngressClassName: nginx
  allowedProviders:
  - corp-ldap
  - local
```

Dex hides the other providers on the sign-in page and refuses a sign-in through them even when the user opens the address of the provider directly. If the list holds one provider, the user goes straight to it, without the choice page. Dex also checks the provider at every refresh-token request and token exchange of the application, so a token issued through a provider that is no longer in the list stops refreshing, and its user has to sign in again.

The list works together with the other limits: `allowedEmails` and `allowedGroups` of the application, and `allowedIdentities` of the provider. A user has to pass all of them. An empty or missing list allows every provider.
`trustedPeers` is the exception. A peer client can ask for a token whose audience is this client through a provider the peer itself allows, and this client's `allowedProviders` is not checked for that token. The same exception applies to `allowedEmails` and `allowedGroups`.

The users who signed in to the application through a provider that left the list have to sign in again even if the provider is added back. With [`sessions.enabled`](configuration.html#parameters-sessions-enabled), Dex keeps the browser session of such a user. Once the provider is back in the list, the user can be signed in by that session without entering the credentials until the session ends.

A name that matches no DexProvider is accepted with a warning. Nobody can sign in through it until the provider is created. Therefore a DexAuthenticator or DexClient applied before its provider by a GitOps tool does not fail.

## Exchanging tokens of other issuers

A [TokenExchangeProvider](cr.html#tokenexchangeprovider) lets Dex exchange a token of another issuer for its own token (RFC 8693 token exchange). The Kubernetes API server accepts the Dex token like the token of any other Dex user. Typical issuers are CI systems, the Dex of a neighbouring cluster and the ServiceAccount token issuer of another cluster.

Dex accepts a token if all of the following hold:

* The signature verifies with the keys of the issuer, from its discovery document (`keys.discovery`) or given in the object (`keys.jwks`).
* If the token header has `kid`, the signature is checked with the key of that `kid` only.
* The `iss` claim equals `issuer`, and the token has not expired.
* The token is valid for no longer than `subjectTokenMaxLifetime` (24 hours by default), counted from `iat` to `exp`, or from now to `exp` for a token without `iat`. The `exp` claim is required.
* The `aud` claim matches an entry of `audiences`.
* The claims listed in `claimValidation` carry the required values.
* The DexClient that sends the request is listed in `clients`.

The subject is named by `identity`. The user name is a prefix followed by the value of a claim, and groups are taken from claims in the same way. The API server takes the user name from the `email` claim. Without `identity.email`, the user name in the cluster is the result of `identity.username`; with `identity.email`, it is the result of `identity.email`. RBAC subjects use this name.

Example for users of a neighbouring cluster A. The Dex of cluster A issues the tokens for its DexClient `cluster-b` in the `d8-user-authn` namespace, so `aud` is `dex-client-cluster-b@d8-user-authn`. In cluster B, the DexClient `cluster-a-exchange` that exchanges them needs the `dexclient.deckhouse.io/allow-access-to-kubernetes` annotation. Apply the following manifest in cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: TokenExchangeProvider
metadata:
  name: cluster-a
spec:
  issuer: https://dex.cluster-a.example.com/
  keys:
    discovery:
      caData: |
        -----BEGIN CERTIFICATE-----
        <CA_OF_CLUSTER_A>
        -----END CERTIFICATE-----
  audiences:
  - exact: dex-client-cluster-b@d8-user-authn
  identity:
    username:
      claim: email
      prefix: "cluster-a:"
    groups:
    - claim: groups
      prefix: "cluster-a:"
  allowedIdentities:
    emailDomains:
    - example.com
  clients:
  - dexClient:
      name: cluster-a-exchange
      namespace: d8-user-authn
```

The client exchanges a token at the Dex token endpoint with `connector_id` set to `status.connectorID` of the object. An example for CI systems is in [Access for CI/CD](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/ci_cd.html#token-exchange).

A TokenExchangeProvider has the following features:

* It serves token exchange only and is not shown on the sign-in page.
* A token of the issuer is accepted only as a JWT ID token. Access tokens are not accepted.
* A change of the object takes effect without a restart of Dex and applies to new exchanges. Dex tokens issued earlier stay valid until they expire. They are refused earlier if the provider loses its connector, that is, if the object is deleted or created again under the same name, or if the provider gives way to an earlier one (`PrefixConflict`). The refusal works under the same condition as for a deleted object, described below. After `PrefixConflict` is gone, the provider gets a connector with a new `status.connectorID`.
* A change refused as invalid (`InvalidSpec`, for example a malformed key or `caData` without a certificate) is not applied. The provider keeps working with its previous spec until the spec is fixed.
* Every prefix is required and ends with a colon. The prefix sets the subjects of the provider apart from people and from the subjects of other providers. Two providers cannot give the same names. A provider whose user name or group prefix overlaps one of a provider created earlier is not ready (`PrefixConflict`) and exchanges nothing until the conflict is removed.
* With `keys.discovery`, the provider becomes ready only after the discovery document and the keys of the issuer are read. Otherwise it is not ready (`DiscoveryFailed`) and is checked again every minute; its connector is written all the same, so a change that narrows the provider applies at once. The discovery document is read over https only.
* Deleting the object makes the Kubernetes API refuse every token issued through it within seconds if the [`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) parameter of the `user-authz` module is enabled and the cluster runs Kubernetes 1.34 or later. Otherwise these tokens stay valid until they expire (the [`idTokenTTL`](configuration.html#parameters-idtokenttl) parameter).
* User and group names starting with `peering:` are reserved for the ServiceAccounts of the `d8-peering` namespace. No DexProvider and no other TokenExchangeProvider can assert them.
* TokenExchangeProvider objects are managed by the `d8:system-capability:user-authn:edit` role, which the `d8:subsystem:iam:manager` role and the roles that include it carry, and in the basic role model by the `ClusterAdmin` access level. Whoever creates a TokenExchangeProvider chooses the issuer, its keys and the names, and so can get a token for any user or group outside the reserved prefixes. This right is equivalent to the right to impersonate users. Grant it to cluster administrators only. The module writes the Dex connectors and revocations in the `d8-user-authn` namespace; only a cluster administrator can change them.

Token exchange through a DexProvider is deprecated and off by default. An `OIDC` or `Gitlab` DexProvider exchanges tokens only with [`spec.tokenExchange.enabled: true`](cr.html#dexprovider-v1-spec-tokenexchange-enabled). The parameter can only be turned off; nobody but the module can set `true`. During the upgrade from a release without this parameter, the module sets it once on the existing `OIDC` and `Gitlab` providers. It sets `true` if some DexClient has the `dexclient.deckhouse.io/allow-access-to-kubernetes` annotation, so that a cluster that may exchange tokens keeps doing it, and `false` otherwise. A DexAuthenticator with access to Kubernetes does not count. Its oauth2-proxy never exchanges tokens, and exchange with its client secret is not kept. The parameter and token exchange through a DexProvider will be removed after the next LTS release.

While a DexProvider exchanges tokens, the `D8UserAuthnDexProviderTokenExchangeDeprecated` alert fires. Move the exchange to a TokenExchangeProvider as described in [Migration from DexProvider](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/ci_cd.html#migration-from-dexprovider), then set `spec.tokenExchange.enabled: false`.

## Browser sessions of Dex

By default, Dex does not remember a user between sign-ins: every application that sends the user to Dex gets the
sign-in form. With the browser sessions of Dex enabled, Dex remembers the user signed in to the browser.
To enable the sessions, set the [`sessions.enabled`](configuration.html#parameters-sessions-enabled) parameter:

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
    sessions:
      enabled: true
```

{% endraw %}

What changes with the sessions:

- The web interfaces of Deckhouse share the sign-in: the user enters the password and the second factor once and opens
  the others without them.
  These are the applications of the DexAuthenticator objects in the `d8-*` namespaces. User DexAuthenticator and
  DexClient objects do not share the sign-in. To turn the sharing off, set
  [`sessions.sso`](configuration.html#parameters-sessions-sso) to `None`.
  The shared sign-in lasts while the browser uses any of these applications, up to
  [`sessions.absoluteLifetime`](configuration.html#parameters-sessions-absolutelifetime). After
  [`sessions.validIfNotUsedFor`](configuration.html#parameters-sessions-validifnotusedfor) without using them, the next
  application asks for the password again.
  A user of an external provider shares the sign-in while the user data Dex stored is fresh: after
  [`sessions.identityRefresh.maxAge`](configuration.html#parameters-sessions-identityrefresh-maxage) without a sign-in or
  a refresh of the tokens of an application, the next application asks for the password again.
- The sign-in form shows the "Remember me" checkbox. With it, the session survives closing the browser.
- When a user signs out of an application of a DexAuthenticator, DexAuthenticator sends the browser to Dex, and Dex ends
  the session. Within seconds, the browser is signed out of every application of a DexAuthenticator it signed in to
  through this session. If the application and Dex are on different sites (different registrable domains), Dex asks the
  user to confirm the sign-out. An application signs out with a navigation of the browser: a script request (`fetch`,
  XHR) does not follow the redirect to Dex, and the session of Dex stays.
- Signing out of Dex does not sign the user out of an external OIDC provider: the provider may sign the user in again
  without a password. To end the session at the provider too, enable
  [`upstreamLogout`](cr.html#dexprovider-v1-spec-oidc-upstreamlogout) of the provider, or set `promptType: login` to
  ask for the password at every sign-in.
- Dex reads the data of the user (such as the groups) from the provider again when the stored data is older than
  [`sessions.identityRefresh.maxAge`](configuration.html#parameters-sessions-identityrefresh-maxage). If the provider
  answers that the user no longer exists or is no longer allowed in, Dex ends all the sessions and tokens of the user of
  this provider.

What does not change:

- Tokens for `kubectl` (kubeconfig, `d8 login`) are issued only after an interactive sign-in, never from a session of the
  browser.
- An application of a DexAuthenticator stays signed in for
  [`keepUsersLoggedInFor`](cr.html#dexauthenticator-v1-spec-keepusersloggedinfor), even when the session of Dex ends by
  its lifetime or idle timeout. Only signing out ends it.

### How long a session lasts

A session of Dex ends at the first of these:

- [`sessions.absoluteLifetime`](configuration.html#parameters-sessions-absolutelifetime) after the sign-in, a week by
  default, however much the browser is used;
- [`sessions.validIfNotUsedFor`](configuration.html#parameters-sessions-validifnotusedfor) without use, a day by
  default;
- the sign-out of the user, or an end of the session by an administrator;
- the closing of the browser, if the user did not select "Remember me" at the sign-in.

The browser uses the session when it signs in to an application through it, and when an application signed in through
it refreshes its tokens: an application of a DexAuthenticator object or of a DexClient object with
[`refreshTokenLifetime: Session`](cr.html#dexclient-v1-spec-refreshtokenlifetime). Such an application refreshes its
tokens every [`idTokenTTL`](configuration.html#parameters-idtokenttl) while it is open and sends requests. A closed
application, or one open in the browser of a sleeping computer, does not use the session.

With the defaults, a user who works every day enters the password and the second factor once a week, and once more
after a break longer than a day, such as a weekend. The session survives a night.

When the session ends, the applications already open stay signed in for
[`keepUsersLoggedInFor`](cr.html#dexauthenticator-v1-spec-keepusersloggedinfor), but signing in to another web
interface asks for the password again.

To ask for the password more often, shorten `sessions.validIfNotUsedFor`, for example to `1h`: the user then enters it
again after every break of an hour. `sessions.validIfNotUsedFor` must not be longer than
`sessions.absoluteLifetime`; when only `sessions.absoluteLifetime` is set and is shorter than a day, an unused session
ends after `sessions.absoluteLifetime`.

### Signing out of the application of a DexClient

An application of a DexClient object signs the user out of Dex by sending the browser to
`https://dex.<PUBLIC_DOMAIN>/logout` with the `client_id` and `post_logout_redirect_uri` parameters. Dex returns the browser
only to the addresses listed in [`postLogoutRedirectURIs`](cr.html#dexclient-v1-spec-postlogoutredirecturis) of the
DexClient:

```text
https://dex.example.com/logout?client_id=dex-client-myapp@mynamespace&post_logout_redirect_uri=https%3A%2F%2Fmyapp.example.com%2F
```

To learn that a session of a user ended, the application can set
[`backchannelLogoutURI`](cr.html#dexclient-v1-spec-backchannellogouturi). Dex sends the notification once, without
retries, when the user signs out of the session, the provider answers that the user is gone, a UserOperation or the
self-service of the web interface ends the session, or the user is deleted; for a session ended outside Dex, within
seconds. An application that must not miss the end can also check its tokens at the introspection endpoint of Dex
(`/token/introspect`), which reports the tokens of an ended session or device inactive.

An application that requests offline access (refresh tokens) gets them only after an interactive sign-in. With
[`refreshTokenLifetime: Session`](cr.html#dexclient-v1-spec-refreshtokenlifetime), the application can sign the user in
from the session of the browser, and its tokens end when the user signs out of this session.

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

When a user is deleted, manually or after its `ttl` expires, the module ends the user's sessions in the same way as the `Logout` operation (see [UserOperation lifecycle and side effects](#useroperation-lifecycle-and-side-effects)) and deletes the user's 2FA (TOTP) secret and lock in Dex. A user created again with the same name does not inherit the sessions, the 2FA secret or the lock.

### Local user operations

Password reset, forced password change, 2FA reset, lock, unlock and logout operations are performed via the [UserOperation](cr.html#useroperation) resource. The `initiatorType` field indicates who initiated the operation: an administrator (`admin`) or the user (`self`).

{% alert level="warning" %}
Do not change `User.spec.password` after the User is created (for example, with `d8 k edit`). The field is immutable and does not update Dex credentials. Reset passwords with a UserOperation or `d8 iam user reset-password`.
{% endalert %}

#### Administrative operations

Use the `d8 iam user` commands for administrative actions on local users. They create a UserOperation resource with `initiatorType: admin`, wait for the operation to complete, and print the result.

You can delete or recreate a local user, or run a UserOperation (`ResetPassword`, `RequirePasswordChange`, `Reset2FA`, `Lock`, `Unlock`, `Logout`) against them, only if you can assign every role already granted to the user's email or groups (covering permissions or an explicit can-assign range). `initiatorType: self` does not bypass this check. The same rule applies to DexProvider objects. For details, see [Limiting the identities of a provider](#limiting-the-identities-of-a-provider).

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

#### Self-service operations

A user can sign out on all devices, and a local user can change their password, in the Deckhouse Platform web interface. The web interface creates a UserOperation with `initiatorType: self` for the user who is signed in:

- Signing out on all devices creates a `Logout` operation. It ends every session of the user, including kubeconfig files and the session in the web interface. Basic Authentication to the Kubernetes API through an LDAP provider keeps working with the password.
- Changing the password creates a `RequirePasswordChange` operation. At the next sign-in, Dex asks for the current password and a new one that complies with the password policy. The operation ends no sessions.

Changing the password is available only for local accounts (the built-in `Local` connector). Users who sign in through external authentication providers change their password in the corresponding system.

A user can also see the devices signed in to their account (the [`status.sessions`](cr.html#useraccount-v1alpha1-status-sessions) field of their UserAccount) and sign out of them one by one:

- Signing out of one device creates a `Logout` operation with [`logout.session`](cr.html#useroperation-v1-spec-logout-session) for a browser or [`logout.grant`](cr.html#useroperation-v1-spec-logout-grant) for another device, such as a kubeconfig file. The other devices keep their sessions.
- Signing out of the other devices creates a `Logout` operation with [`logout.exceptSession`](cr.html#useroperation-v1-spec-logout-exceptsession) or [`logout.exceptGrant`](cr.html#useroperation-v1-spec-logout-exceptgrant). It keeps only the device the user acts from.

Signing out on all devices, signing out of the other devices or of another device, and changing the password are available only on a trusted device: one signed in at least [`devices.trustedAfter`](configuration.html#parameters-devices-trustedafter) ago, 24 hours by default. A device signed in more recently can sign the user out only of itself. Someone who has just signed in with a stolen password therefore can neither sign the owner out nor change the password, while the owner sees the new device in the list on a trusted device and signs it out. A user without a trusted device asks an administrator: the operations of an administrator are not limited. With `devices.trustedAfter: 0s`, every device is trusted at once.

The age of a device counts from the first sign-in of its browser, not only the last one. At sign-in, Dex gives the browser a long-lived cookie and remembers when it first saw that browser sign in to the account (the [`knownSince`](cr.html#useraccount-v1alpha1-status-sessions-knownsince) field). A browser that signed out and signed in again keeps its age. Dex forgets the browser, and its next sign-in counts as new, when:

- the last sign-in of the browser is ended from another device or by an administrator. While the browser holds another active session or token of the account, it stays known;
- the user signs out of the other devices: every browser but the current one is forgotten, signed in or not;
- the user signs out on all devices, or an administrator locks the user, resets the password, resets 2FA, or signs the user out;
- the account has more remembered browsers than [`devices.rememberedBrowsers`](configuration.html#parameters-devices-rememberedbrowsers) (20 by default): the browser seen longest ago is forgotten. With `0`, Dex remembers no browser, and every sign-in is a new device.

Someone who signs in with a stolen password uses a browser without that cookie, so the device is new for them. A stolen session is not covered: the age is that of the session or token of the request, so a copied cookie of Dex or of the web interface has the age of the owner's device.

If the password may have been stolen, act from a trusted device in this order:

1. Request a password change.
2. Right away, sign out on this device and sign in again: Dex asks for the current and the new password. The browser keeps its age.
3. Only then sign out of the other devices. The current browser stays known, so it stays trusted, and every other device, the intruder's included, is signed out.

Do steps 1 and 2 one right after the other. Until the new password is set, any sign-in asks for it, the intruder's with the stolen password included, and the age of the device is not checked on that page.

Do not start with signing out on all devices: it makes the current browser new as well, and for `devices.trustedAfter` it can neither request a password change nor sign out a device that signs in again with the stolen password.

A compromised password alone signs in to an account without 2FA. Enable 2FA with the [`twoFactor`](configuration.html#parameters-twofactor) parameter.

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

Example — require a new password at the next sign-in without resetting the current one:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: require-password-change-admin
spec:
  user: admin
  type: RequirePasswordChange
  initiatorType: admin
```

Example — end every session of a user in every application and kubeconfig file:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: logout-admin
spec:
  user: admin
  type: Logout
  initiatorType: admin
```

Example — sign a user out of one device and keep the sessions on the others. Take the `id` of the device from [`status.sessions`](cr.html#useraccount-v1alpha1-status-sessions) of the user's UserAccount (for details, see the [Viewing account state](#viewing-account-state) section). For a device of the `Browser` kind, set it in [`logout.session`](cr.html#useroperation-v1-spec-logout-session); for a device of the `Grant` kind, such as a kubeconfig file, in [`logout.grant`](cr.html#useroperation-v1-spec-logout-grant):

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: logout-admin-kubeconfig
spec:
  user: admin
  type: Logout
  initiatorType: admin
  logout:
    grant: qx3w2ttiyq5ovzvbuehdcbbse
```

#### Operations on external users

For users of external providers of any type (LDAP, Atlassian Crowd, OIDC, SAML, GitHub, GitLab, Bitbucket Cloud), use the [`spec.target`](cr.html#useroperation-v1-spec-target) field instead of `spec.user`. The `Lock`, `Unlock`, `Logout` and `Reset2FA` operation types are supported for external users. Only the users of the LDAP and Atlassian Crowd providers can have a second factor. For a user of a provider of another type, `Reset2FA` only ends the sessions.

The operation finds the user's account by the email the provider reported at the user's last sign-in. The `connectorID` and `userID` fields narrow the search down to one provider and one user ID. A lock applies to one pair of a provider and a user. It blocks only the account found and does not block a local user or a user of another provider with the same email. A local user is addressed by `spec.user` only. Dex checks the lock at every sign-in through the provider, at every refresh-token request, at token exchange, and at Basic Authentication to the API server through an LDAP provider. For a Crowd or OIDC provider, basic-auth-proxy checks the credentials with the provider directly, so the lock does not apply to Basic Authentication.

The operation fails if no account has the email (the user has never signed in) or if more than one account matches. In that case, set `connectorID`, and `userID` if the provider has several users with the email.

A lock applies to the user ID that the provider returns at sign-in, so this ID must stay the same at every sign-in of the user. For a SAML provider, set [`nameIDPolicyFormat`](cr.html#dexprovider-v1-spec-saml-nameidpolicyformat) to `persistent` or `emailAddress`, or leave it unset (Dex then requests `persistent`). With `transient`, the provider returns a new ID at every sign-in. A lock then does not apply to the next sign-in of the user, and once the user has signed in twice, the operation fails because more than one user of the provider has the email.

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

#### Users of a deleted provider

If you delete a DexProvider and create it again under the same name within 30 days, the second factor and the lock of its users are kept. If you create it later, the 2FA of a user who has not signed in for 30 days is registered again at the first sign-in. A user whose lock is still in force keeps the lock and the second factor until the lock ends.

#### UserOperation lifecycle and side effects

Using a UserOperation object has the following specifics:

- A UserOperation is a **single-use** object. After creation, the module processes it and writes the result to `status.phase` (`Succeeded` or `Failed`).
- Operations are **automatically deleted** after [`retention.userOperations`](configuration.html#parameters-retention-useroperations) (24 hours by default).
- A UserOperation runs once. Changes to its spec after creation are ignored.
- To perform a new action, create a new UserOperation.

{% alert level="warning" %}
The `Lock`, `ResetPassword`, `Reset2FA` and `Logout` operations end the sessions of the user at once:

- Dex refuses every refresh token, authorization code and browser session it issued to the user before the operation. Applications and kubeconfig files ask the user to sign in again.
- DexAuthenticator stops accepting the sessions of the user within seconds.
- The Kubernetes API refuses the ID tokens issued to the user before the operation within seconds if the [`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) parameter of the `user-authz` module is enabled and the cluster runs Kubernetes 1.34 or later. Otherwise these ID tokens stay valid for the Kubernetes API until they expire (the [`idTokenTTL`](configuration.html#parameters-idtokenttl) parameter).
- Basic Authentication to the Kubernetes API through an LDAP provider checks the password with Dex again. After `Logout`, it keeps working with the password.

`Unlock` and `RequirePasswordChange` end no sessions. Only `Reset2FA` removes the user's 2FA secret. A `Lock` refuses sign-in, refresh-token requests and token exchange until the lock ends. The `status.revocation` field of the operation shows what it revoked.
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

The module keeps a [UserAccount](cr.html#useraccount) object for each local user and for each user who has signed in through an external provider. It shows when the user last signed in, whether the user has set up 2FA, whether the account is locked, until when, and how many failed login attempts were counted. Do not create or edit UserAccount objects.

To list the accounts, run:

```shell
d8 k get useraccounts
```

The [`status.sessions`](cr.html#useraccount-v1alpha1-status-sessions) field lists the devices signed in to the account, the most recently active first: browsers signed in to Dex (the `Browser` kind) and devices holding a refresh token not bound to a live browser session of Dex, such as kubeconfig files (the `Grant` kind). For each device, it shows the applications, the client address and User-Agent of the last sign-in, the time of the sign-in, and the hour of the last activity. The client address is right only when the Ingress controller sees it (for details, see the [Which client address does Dex see?](faq.html#which-client-address-does-dex-see) section). To view the devices of an account, run:

```shell
d8 k get useraccount <NAME> -o jsonpath='{range .status.sessions[*]}{.kind}{"\t"}{.id}{"\t"}{.clients}{"\t"}{.ipAddress}{"\t"}{.lastActivity}{"\n"}{end}'
```

While the [browser sessions of Dex](#browser-sessions-of-dex) are disabled, every sign-in to an application of a DexAuthenticator is a device of its own.

Dex keeps at most [`devices.maxPerApplication`](configuration.html#parameters-devices-maxperapplication) (10 by default) refresh tokens of a user for one application. A sign-in past the limit signs the device of the least recently used one out of that application; a trusted device (the [`devices.trustedAfter`](configuration.html#parameters-devices-trustedafter) parameter) goes only once no untrusted one is left. A refresh token is replaced, not added, in two cases: an application of a DexAuthenticator signs in again in a browser with a session of Dex, and Basic Authentication to the Kubernetes API signs in again. Every other sign-in is a new device: for example, every new kubeconfig file is one, and the previous ones keep working.

#### Automatic lockout

If the number of failed sign-in attempts exceeds [`passwordPolicy.lockout.maxAttempts`](configuration.html#parameters-passwordpolicy-lockout-maxattempts), Dex locks the user for [`lockout.lockDuration`](configuration.html#parameters-passwordpolicy-lockout-lockduration) and then unlocks them automatically. For LDAP and Crowd users, this works only if the provider type is listed in [`lockout.applyToConnectors`](configuration.html#parameters-passwordpolicy-lockout-applytoconnectors). Dex does not create a UserOperation for this lockout. To unlock the user earlier, run `d8 iam user unlock` or create a UserOperation of type `Unlock`.

While a local, LDAP or Crowd user is locked, Dex refuses a password sign-in whether the password is correct or not: with the `429` status code and the "Account temporarily locked" page after an automatic lockout, with the `403` status code and the "Account locked by an administrator" page while a lock set by an administrator is in force. The password change page refuses a locked user in the same way. An automatic lockout does not shorten a lock that ends later, including a lock set by an administrator.

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

When a local user signs in with a password that has to change, Dex opens the password change page instead of the application. A password has to change after an administrator resets it with `ResetPassword`, when it does not meet the complexity rules, and when it is older than `rotation.interval`. The page asks only for the new password, twice, because the sign-in form has just checked the current one. If 2FA is enabled, Dex first asks for the code and opens the password change page only after it, so someone who knows only the password cannot change it. After the change, the sign-in continues as usual and opens the application.

The password change page opens only during the sign-in that led to it and in the same browser. If the page is opened in another browser or after the sign-in has expired, Dex does not open it, and the user signs in again.

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

2FA increases security by requiring a code from a `TOTP` authenticator application (for example, Google Authenticator) or a security key (`WebAuthn`) during login. It can be required from the local users and from the users of the LDAP and Atlassian Crowd providers.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 4
  enabled: true
  settings:
    twoFactor:
      enabled: true
      issuerName: "awesome-app"
      providerTypes: [Local, LDAP]
```

{% endraw %}

Field description:

* `enabled`: Enables or disables 2FA.
* `issuerName`: Name displayed in the authenticator application when adding an account (with `WebAuthn`, in the dialog of the browser that registers the key).
* `methods`: The second factors: `TOTP` (the default), `WebAuthn`, or both, see [Several methods](#several-methods).
* `require`: How two `methods` combine. Required with two methods, not allowed with one.
* `providerTypeOverrides`: Other `methods` for one type of provider, see [Several methods](#several-methods).
* `webAuthn`: Parameters of the `WebAuthn` method, see [Security keys](#security-keys-webauthn).
* `providerTypes`: Types of the providers whose users have to pass the second factor: `Local` (the built-in users), `LDAP` and `Crowd`. The users of every provider of a listed type are asked for it. The OIDC, SAML, GitHub, GitLab and Bitbucket Cloud providers are not on the list. The identity provider handles the second factor for them.

The `twoFactor` parameter replaces `staticUsers2FA`, which covered the local users only. A ModuleConfig of version 3 with `staticUsers2FA` keeps working, because the module converts it.

{% alert level="info" %}
After enabling 2FA, each user must register in the authenticator application during their first login. Until then, anyone who knows the password can register their own device, so ask the users to sign in right after you turn 2FA on.
{% endalert %}

Dex asks the user for the code (or the key) at a sign-in through a provider of a listed type, except while a browser session of Dex is valid. Sign-ins through the other providers go without it.
Turning 2FA on for a provider type does not end the refresh tokens or the Dex browser sessions issued before. A refresh token keeps issuing tokens without the code until it expires (by default, up to `refreshTokenAbsoluteLifetime`, 4380h), and a browser session created before the change passes without the code.
After enabling 2FA, run a UserOperation of type `Logout` for the users of the providers of the listed types.

Basic Authentication to the Kubernetes API (`enableBasicAuth` of an LDAP or Crowd provider) has no page for a code, so it is not possible for a provider of a listed type. The module refuses the settings that turn 2FA on while such a provider exists, and a provider that turns `enableBasicAuth` on while 2FA covers its type. To use both, give the Kubernetes API users a kubeconfig file with the web interface instead of Basic Authentication.

#### Security keys (WebAuthn)

With `methods: [WebAuthn]`, Dex asks the user to touch a security key (for example, a YubiKey) or to confirm the sign-in with the Touch ID or Windows Hello of the device, instead of entering a code. The key confirms a sign-in only for the domain it is bound to, so a fake sign-in page on another domain gets no response it can reuse.

The following settings ask the users for a security key:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 4
  enabled: true
  settings:
    twoFactor:
      enabled: true
      methods: [WebAuthn]
      issuerName: "Example"
      webAuthn:
        rpID: example.com
```

The method works as follows:

* A key is bound to the domain set by the [`rpID`](configuration.html#parameters-twofactor-webauthn-rpid) parameter. By default, it is the domain of the Dex address (`dex.<PUBLIC_DOMAIN>`). To keep the keys through a change of `publicDomainTemplate`, set `rpID` to the stable parent domain of the Dex address, for example `example.com` for `dex.example.com`.
  A change of `rpID`, or of `publicDomainTemplate` while `rpID` is not set, makes every registered key unusable. A user with such a key can neither sign in nor register a new one. Reset the second factor of the user with a UserOperation of type `Reset2FA`, and the user registers a key again at the next sign-in.
* Dex accepts a security key of any make and model and does not check which key it is.
* The browser has to support WebAuthn.
* The user registers the key at the first sign-in after the method is turned on, as the user of TOTP registers the application. The maximum number of keys that a user can have is specified in the [`webAuthn.maxCredentials`](configuration.html#parameters-twofactor-webauthn-maxcredentials) parameter (3 by default). A user adds one with the "Add a key" button on the page that confirms a sign-in with a key, after a registered key has confirmed it.
  Until the registration, anyone who knows the password can register their own key, so ask the users to sign in right after the switch.
* A single key cannot be removed. If any of the keys has been lost or compromised, reset the user's `WebAuthn` method with a [`Reset2FA` operation](#the-second-factor-of-an-account). Until then, such a key still confirms a sign-in for whoever knows the password. The operation removes every key of the user, and the user registers the keys again at the next sign-in.
* When the methods change, the TOTP secret and the keys the user had stay until a `Reset2FA`, so asking for a method again brings them back.

#### Several methods

If two methods are specified in the `methods` parameter, the `require` parameter defines how they are combined.
With `Any`, the user passes one of them. At the first sign-in the user chooses the method and registers it. Later Dex asks for a registered method, the one used last first, and a user who has registered both can sign in another way. A user who has registered one method cannot register the other at a sign-in, because otherwise anyone who knows the password could add a method of their own. To let a user switch, remove the registered method with a `Reset2FA` operation.
With `All`, the user passes both, the code first and then the key. A user who has only one of them confirms the sign-in with it first and only then registers the other.

The following settings let each user choose a code or a key:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 4
  enabled: true
  settings:
    twoFactor:
      enabled: true
      methods: [TOTP, WebAuthn]
      require: Any
      webAuthn:
        rpID: example.com
```

A Deckhouse Platform version without `require: Any` refuses such settings. Before a rollback to it, switch `require` to `All` or keep one method.

`providerTypeOverrides` sets other methods for one type of provider from `providerTypes`, for example a key for the users of the LDAP providers and a code for the local users:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 4
  enabled: true
  settings:
    twoFactor:
      enabled: true
      methods: [TOTP]
      providerTypes: [Local, LDAP, Crowd]
      providerTypeOverrides:
      - providerType: LDAP
        methods: [WebAuthn]
      - providerType: Crowd
        methods: [TOTP, WebAuthn]
        require: All
      webAuthn:
        rpID: example.com
```

A type without an entry takes `methods` and `require`. Each type appears once, and only a type listed in `providerTypes`.

#### The second factor of an account

[UserAccount](cr.html#useraccount) shows in `status.twoFactor` the `methods` and `require` asked of the users of its type of provider, and in `enrolled` whether the account has them. `totp` and `webAuthn` show the TOTP secret and the number of keys the account has, including those of a method no longer asked for, which do not count for `enrolled`.

To make a user register the second factor again, for example after losing the device, reset the user's 2FA.
For a local user, use `d8 iam user reset2fa <USERNAME>` or a UserOperation of the `Reset2FA` type with `spec.user`. For a user of an external provider, use a UserOperation of the `Reset2FA` type with [`spec.target`](#operations-on-external-users). Without other parameters the operation removes every method. To remove one, list it in `spec.reset2FA.methods`:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: reset-key-jane
spec:
  type: Reset2FA
  initiatorType: admin
  user: jane
  reset2FA:
    methods: [WebAuthn]
```

The operation also ends the sessions of the user. Other operations, including `Lock` and `ResetPassword`, keep the second factor.

### Assigning permissions to a user or group

To grant permissions to a user or a group, bind one of the `user-authz` roles with a RoleBinding or ClusterRoleBinding, using the user's email or the group name as the subject. See the [granular role-based model](/modules/user-authz/#granular-role-based-model).
