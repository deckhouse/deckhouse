---
title: "Integration with external authentication providers"
permalink: en/admin/configuration/access/authentication/external-authentication-providers.html
description: "Integrate Deckhouse Kubernetes Platform with external authentication providers including LDAP, OIDC, GitHub, GitLab, Atlassian Crowd, and Bitbucket. Step-by-step configuration guide."
---

Connecting an external authentication provider allows you to use a single set of credentials to access multiple clusters and simultaneously work with multiple providers.

DKP supports integration with the following external authentication providers and protocols:

- [LDAP (for example, Active Directory)](#ldap-integration);
- [OIDC (for example, Okta, Keycloak, Gluu, Blitz Identity Provider)](#oidc-openid-connect-integration);
- [GitHub integration](#github-integration);
- [GitLab integration](#gitlab-integration);
- [Atlassian Crowd integration](#atlassiancrowd-integration);
- [Bitbucket Cloud integration](#bitbucketcloud-integration).

{% alert level="info" %}
Password policies (such as complexity requirements, expiration, history, two-factor authentication, etc.) are fully controlled by the external authentication provider.  
Deckhouse does not manage passwords or interfere with policy enforcement on the provider side.
{% endalert %}

## General integration workflow

{% alert level="info" %}
The [`allowedGroups`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-allowedgroups) parameter in the DexProvider resource allows you to restrict login access to users who belong to specific groups.  
If the `allowedGroups` list is specified, the user **must** be a member of at least one of these groups — otherwise, authentication will be considered unsuccessful.  
If the parameter is not specified, no group-based filtering will be applied.
{% endalert %}

1. Create an OAuth application in the authentication provider:
   - Set the redirect URI to `https://dex.<publicDomainTemplate>/callback`.
   - Obtain the `clientID` and `clientSecret`.

   > **Important**: When specifying the redirect URI, substitute the actual value of `publicDomainTemplate` without `%s`.  
   > For example, if `publicDomainTemplate: '%s.sandbox1.deckhouse-docs.flant.com'`, the actual URI would be:  
   > `https://dex.sandbox20.deckhouse-docs.flant.com/callback`.
   >
   > To retrieve the Dex address (URI), run:
   >
   > ```console
   > d8 k -n d8-user-authn get ingress dex -o jsonpath="{.spec.rules[*].host}"
   > ```

1. Create a [DexProvider](/modules/user-authn/cr.html#dexprovider) resource tailored to the specifics of your selected identity provider.

1. Enable the [`user-authn`](/modules/user-authn/) module (if it is currently disabled).

   This can be done either via the Deckhouse admin web interface or through the CLI.  
   Below is an example using the [Deckhouse CLI](/products/kubernetes-platform/documentation/v1/cli/d8/) (requires kubectl context configured to access the cluster):

   Check the module status:

   ```shell
   d8 k get module user-authn
   ```

   Example output:

   ```console
   NAME         STAGE   SOURCE     PHASE       ENABLED   READY
   user-authn           Embedded   Available   True      True
   ```

   Enable the module via CL:

   ```shell
   d8 platform module enable user-authn
   ```

1. Configure the [`user-authn`](/modules/user-authn/) module.

   - Open the `user-authn` module settings (create a ModuleConfig resource named `user-authn` if it doesn't exist):

     ```shell
     d8 k edit mc user-authn
     ```

   - Specify the required module parameters in the `spec.settings` section.
     For more details about the `user-authn` module settings, refer to the [module reference](/modules/user-authn/).

     Example `user-authn` configuration:

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

### OIDC (OpenID Connect) integration

Authentication via an OIDC provider requires registering a client (or creating an application). Follow your provider's documentation to do this (e.g., [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration), or [Blitz](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html)).

Specify the `clientID` and `clientSecret` obtained during setup in the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource.

{% alert level="info" %}
When registering an application with any OIDC provider, you must specify a redirect URI.  
For integration with DexProvider, use the following format: `https://dex.<publicDomainTemplate>/callback`, where [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) is the DNS name template of your cluster as defined in the `global` module.
{% endalert %}

{% alert level="info" %}
To ensure proper token revocation on logout and to force re-authentication, set the `prompt` parameter to `login`.  
This guarantees that the user will be prompted to re-enter credentials during subsequent logins.
{% endalert %}

To configure fine-grained access control for users in applications:

- Add the `allowedUserGroups` parameter to the ModuleConfig of the target application.
- Assign the appropriate groups to the user, using identical group names on both the provider and Deckhouse sides.

#### Keycloak

During Keycloak configuration, select the appropriate `realm`, add a user in the [Users](https://www.keycloak.org/docs/latest/server_admin/index.html#assembly-managing-users_server_administration_guide) section, and create a client in the [Clients](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide) section with [authentication](https://www.keycloak.org/docs/latest/server_admin/index.html#capability-config) enabled, which is required to generate a `clientSecret`. Then follow these steps:

1. In the [Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes) section, create a `scope` named `groups` and assign it a mapper `Group Membership` ("Client scopes" → "Client scope details" → "Mappers" → "Configure a new mapper"). Set values of "Name" and "Token Claim Name" as `groups` and turn off "Full group path".
1. In the previously created client, add this `scope` in the [Client scopes tab](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) ("Clients" → "Client details" → "Client Scopes" → "Add client scope").
1. In the "Valid redirect URIs", "Valid post logout redirect URIs", and "Web origins" fields in the [client configuration](https://www.keycloak.org/docs/latest/server_admin/#general-settings), specify `https://dex.<publicDomainTemplate>/*`, where `publicDomainTemplate` is the [cluster DNS name template](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) defined in the `global` module.

Example provider configuration for Keycloak integration:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: keycloak
spec:
  type: OIDC
  displayName: My Company Keycloak
  oidc:
    issuer: https://keycloak.my-company.com/realms/myrealm # Use your realm name
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

- Delete the `Email verified` mapping ("Client Scopes" → "Email" → "Mappers").
  This is required for proper processing of the [`insecureSkipEmailVerified`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-insecureskipemailverified) field when it's set to `true` and for correct permission assignment to users with unverified emails.

- If you can't modify or delete the `Email verified` mapping, create a new Client Scope named `email_dkp` (or any other name) and add two mappings:
  - `email`: "Client Scopes" → `email_dkp` → "Add mapper" → "From predefined mappers" → `email`.
  - `email verified`: "Client Scopes" → `email_dkp` → "Add mapper" → "By configuration" → "Hardcoded claim". Specify the following fields:
    - "Name": `email verified`
    - "Token Claim Name": `emailVerified`
    - "Claim value": `true`
    - "Claim JSON Type": `boolean`

  After that, in the client registered for the DKP cluster in "Clients", change `Client scopes` from `email` to `email_dkp`.

  In the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource, specify `insecureSkipEmailVerified: true` and in the `.spec.oidc.scopes` field, change the Client Scope name to `email_dkp` following the example:
  
  ```yaml
  scopes:
   - openid
   - profile
   - email_dkp
   - groups
  ```

#### Blitz Identity Provider

Example configuration of a provider for integration with Blitz Identity Provider:

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
      groups: your_claim # Claim for obtaining user groups; groups are configured on the Blitz Identity Provider side.
    clientID: clientID
    clientSecret: clientSecret
    getUserInfo: true
    insecureSkipEmailVerified: true # Set to true if email verification is not required.
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

#### Okta

Example configuration of a provider for integration with Okta:

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

After enabling integration with Okta, you can use Okta user groups to manage access rights.
For example, you can specify a list of groups whose members are allowed to access [Grafana](../../../../user/web/grafana.html).

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

### LDAP integration

To configure authentication, create a read-only account (service account) in your LDAP directory.  
This account will be used to perform search queries in the LDAP catalog.

In the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource, specify the following parameters:

- `bindDN`: Full Distinguished Name (DN) of the created service account. For example: `cn=readonly,dc=example,dc=org`.
- `bindPW`: Password for the specified `bindDN`.

{% alert level="info" %}
If your LDAP server allows anonymous access for search queries, the `bindDN` and `bindPW` parameters can be omitted.  
However, using authenticated access is recommended for improved security.

The `bindPW` parameter must contain the password in plain text. Dex does not support hashed passwords in this field.
{% endalert %}

Example configuration for integrating with Active Directory:

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

### GitHub integration

You need to create a new application in your GitHub organization.

To do this, follow these steps:

1. Go to "Settings → Developer settings → OAuth Apps → New OAuth App", and set the "Authorization callback URL" to `https://dex.<publicDomainTemplate>/callback`.
1. Use the generated `Client ID` and `Client Secret` in the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource.

If the GitHub organization is managed by a client:

1. Go to "Settings → Applications → Authorized OAuth Apps → `<name of created OAuth App>`" and click "Send Request" to submit an approval request.
1. Ask the client to approve the request that will be sent to their email.

Example configuration for integrating with GitHub:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: github
spec:
  type: Github
  displayName: My Company GitHub
  github:
    clientID: plainstring
    clientSecret: plainstring
```

### GitLab integration

You need to create a new application in your GitLab project.

To do this, follow these steps:

1. For self-hosted GitLab: go to "Admin Area → Applications → New application" and set the "Redirect URI (Callback url)" to `https://dex.<publicDomainTemplate>/callback`. Also, select the following scopes: `read_user`, `openid`.
1. For GitLab Cloud (gitlab.com): under the main account of the project, go to "User Settings → Applications → Add new application", set the "Redirect URI (Callback url)" to `https://dex.<publicDomainTemplate>/callback`, and select the **scopes**: `read_user`, `openid`.
1. Use the obtained `Application ID` and secret in the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource.

{% alert level="info" %}
For GitLab version 16 and above, enable the "Trusted" option when creating the application.  
This option is available under "Admin Area → Applications". Marking the app as trusted allows skipping the authorization step for users, which can be useful in controlled environments.
{% endalert %}

Example configuration for integrating with GitLab:

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

### Atlassian Crowd integration

In the relevant Atlassian Crowd project, you need to create a new Generic application.

To do this, follow these steps:

1. Go to "Applications → Add application".
1. Specify the obtained "Application Name" and "Password" in the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource.
1. When specifying groups in the DexProvider resource, make sure their names are written in lowercase.  
   This is necessary for correct group matching between Crowd and Deckhouse.

Example configuration for integrating with Atlassian Crowd:

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

### Bitbucket Cloud Integration

To configure authentication, you need to create a new OAuth consumer in the Bitbucket team menu.

Follow these steps:

1. Go to "Personal settings → Access management → OAuth consumers → Add consumer", and specify `https://dex.<publicDomainTemplate>/callback` as the "Callback URL".
1. Grant access:  
   - "Account: Read" → allows retrieval of basic user information (e.g., email, username).
   - "Workspace membership: Read" → allows retrieval of user workspace membership information.
1. Use the obtained `Key` and secret in the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource.

Example configuration for integrating with Bitbucket:

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
