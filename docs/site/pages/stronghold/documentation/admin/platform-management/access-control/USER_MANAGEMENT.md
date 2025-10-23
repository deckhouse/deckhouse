---
title: "User management"
permalink: en/stronghold/documentation/admin/platform-management/access-control/user-management.html
---

## Description

DVP supports both internal user and group management,
as well as integration with external authentication providers and protocols, such as:

- [GitHub](#github)
- [GitLab](#gitlab)
- [Crowd](#atlassian-crowd)
- [Bitbucket Cloud](#bitbucket-cloud)
- [LDAP](#ldap)
- [OIDC](#oidc-openid-connect)

You can connect multiple external authentication providers at the same time.

Users can access DVP web interfaces, such as Grafana and Console,
and use command-line utilities like `d8` or `kubectl` to interact with the DVP APIs,
considering the granted access permissions.

For details on granting permissions to users and groups, refer to [Role Model](./role-model.html).

## Create a user

To create a static user, use the User resource.

Before creating a user, generate a password hash using the following command:

```shell
# To avoid saving the password in the command history, begin the command line with a space character
# Replace example_password with your password
 echo -n 'example_password' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
```

Alternatively, you can use [Bcrypt](https://bcrypt-generator.com/).

Example of a manifest for creating a user:

```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: joe
spec:
  email: joe@example.com # Used in RoleBinding and ClusterRoleBinding to assign user permissions
  password: 'JDJ5JDEwJG5qNFZUWW9vVHBQZUsxV1ZaNWtOcnVzTXhDb3ZHcWNFLnhxSHhoMUM0aG9zVVJubUJkZjJ5'
  ttl: 24h # (Optional) Sets the lifetime of the user account
```

## Create a user group

To create a user group, use the Group resource.

Example of a manifest for creating a user group:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: vms-admins
spec:
  # A list of users
  members:
  - kind: User
    name: joe
  name: vms-admins # Used in RoleBinding and ClusterRoleBinding to assign user group permissions
```

## Create a configuration file for remote access

To control the cluster remotely using command-line utilities like `d8` or `kubectl`,
create a configuration file:

1. In the ModuleConfig resource of the `user-authn` module, enable access to the DVP API by setting the `.spec.settings.publishAPI.enabled` parameter to `true`.
1. Using the kubeconfigurator web interface, generate a `kubeconfig` file for remote access to the cluster.
    The `kubeconfig` name is reserved for accessing the web interface that generates the `kubeconfig` file.
    The access URL is determined by the value of the `publicDomainTemplate` parameter.

    To find out the address for accessing the service, run the following command:

    ```shell
    d8 k get ingress -n d8-user-authn
    # NAME                   CLASS   HOSTS                              ADDRESS                            PORTS     AGE
    # ...
    # kubeconfig-generator   nginx   kubeconfig.example.com             172.25.0.2,172.25.0.3,172.25.0.4   80, 443   267d
    # ...
    ```

1. Go to the provided address and log in using the email and password you specified when creating a user.

## Configuration of external providers

To configure an external provider, use the DexProvider resource.

### GitHub

Example of a manifest for configuring a provider to integrate with GitHub:

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

In a [GitHub organization](https://docs.github.com/en/organizations), create a new application:

1. Go to **Settings** → **Developer settings** → **OAuth Apps** → **Register a new OAuth application**.
1. In the **Authorization callback URL** field, enter:
   `https://dex.<modules.publicDomainTemplate>/callback`.

Specify `Client ID` and `Client Secret` that you receive in the custom `DexProvider` resource.

If the GitHub organization is managed by a client, do the following:

1. Go to **Settings** -> **Applications** -> **Authorized OAuth Apps**.
1. Find the created application using its name and click **Send Request** to confirm.
1. Ask the client to confirm the request sent to their email.

Once you go through these steps, your application will be ready for use as an authentication provider via GitHub.

### GitLab

Example of a manifest for configuring a provider to integrate with GitLab:

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

> `groups` in the above example is a list of allowed GitLab group filters specified by their paths and not by names. The user token will contain a set intersection of GitLab groups and groups from this list. If the set is empty, the authorization will be considered unsuccessful. The user token will contain all GitLab groups if the parameter is not set.

To create an application in GitLab, follow the steps below.

For a self-managed GitLab instance:

1. Go to **Admin area** → **Application** → **New application**.
2. In the **Redirect URI (Callback URL)** field, enter the address:  
   `https://dex.<modules.publicDomainTemplate>/callback`.
3. Select the following scopes:
   - `read_user`
   - `openid`

For a GitLab SaaS instance:

1. Using the Owner or Maintainer role account, go to **User Settings** → **Applications** → **New application**.
1. In the Redirect URI (Callback URL) field, enter the address:  
   `https://dex.<modules.publicDomainTemplate>/callback`.
1. Select the following scopes:
   - `read_user`
   - `openid`

For GitLab 16.0 or newer:

1. When creating the application, mark it as **trusted**.
    Trusted applications are automatically authorized on GitLab OAuth flow.
1. Use `Application ID` and `Secret` that you receive in the custom `DexProvider` resource.

### Atlassian Crowd

Example of a manifest for configuring a provider to integrate with Atlassian Crowd:

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

To create a generic application in Atlassian Crowd, follow these steps:

1. Go to **Applications** → **Add application**.
1. Use `Application Name` and `Password` that you receive in the [DexProvider](/modules/user-authn/cr.html#dexprovider) resource.
1. Specify CROWD groups in lowercase for the `DexProvider` resource.

### Bitbucket Cloud

Example of a manifest for configuring a provider to integrate with Bitbucket:

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

To set up authentication in Bitbucket, follow these steps:

1. In the workspace menu, create a new OAuth consumer.
1. Go to **Settings** → **OAuth consumers** → **New application** and set the following address in the **Callback URL** field: `https://dex.<modules.publicDomainTemplate>/callback`.
1. Allow access for `Account: Read` and `Workspace membership: Read`.
1. Specify `Key` and `Secret` that you receive in the custom `DexProvider` resource.

### LDAP

Example of a manifest for configuring a provider to integrate with Active Directory:

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

To set up authentication in LDAP, follow these steps:

1. Create a read-only user (service account) in LDAP.
1. Specify the user path and password you receive in the `bindDN` and `bindPW` parameters of the custom `DexProvider` resource.
1. If LDAP has anonymous read access configured, these settings can be skipped.
1. In the `bindPW` parameter, specify the password in plain text. Hashed password can't be used.

### OIDC (OpenID Connect)

Authentication via an OIDC provider requires registering a client or creating an application.
To do this, follow a guide from a respective provider: [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration), or [Blitz](https://docs.identityblitz.com/latest/integration-guide/oidc-app-enrollment.html).

Specify `clientID` and `clientSecret` that you receive in the custom `DexProvider` resource.

Below are several manifests with configuration examples.

#### Okta

Example of a manifest for configuring a provider to integrate with Okta:

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

When [registering an application](https://docs.identityblitz.com/latest/integration-guide/oidc-app-enrollment.html) with Blitz Identity Provider, specify the URL to redirect users after authorization.
When using `DexProvider`, specify `https://dex.<publicDomainTemplate>/`, where `publicDomainTemplate` is the cluster DNS name template [configured](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) in the `global` module.

Example of a manifest for configuring a provider to integrate with Blitz Identity Provider:

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
      groups: your_claim # Claim to receive user groups. User groups are configured on the Blitz Identity Provider's side
    clientID: clientID
    clientSecret: clientSecret
    getUserInfo: true
    insecureSkipEmailVerified: true # If email verification isn't required, set to true
    insecureSkipVerify: false
    issuer: https://yourdomain.idblitz.com/blitz
    promptType: consent
    scopes:
    - profile
    - openid
    userIDKey: sub
    userNameKey: email
  type: OIDC
```

To ensure proper logout from applications, involving token callbacks and requiring re-authorization, set the `promptType` parameter to `login`.

To ensure detailed user access to applications, do the following:

1. Add the `allowedUserGroups` parameter to ModuleConfig of the respective application.
1. Assign groups to the user. Group names must match those configured in Blitz and Deckhouse.

Example of adding groups for the Prometheus module:

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
