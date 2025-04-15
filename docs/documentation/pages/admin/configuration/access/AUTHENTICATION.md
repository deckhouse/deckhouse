---
title: "Authentication"
permalink: en/admin/configuration/access/authentication.html
---

## Overview

Authentication is the process of verifying a user's identity. In the Deckhouse Kubernetes Platform (DKP), end-to-end authentication is implemented, allowing user verification across all DKP interfaces and cluster resources. A cluster user can also leverage DKP to enable authentication within their own application.

Authentication in DKP is based on a federated OIDC provider. You can learn more about the authentication architecture in DKP in the [Architecture](#TODO---link-to-authentication-architecture) section.

At the core of DKP's authentication mechanism is the federated OpenID Connect provider `Dex`. Depending on the DKP configuration, authentication can use either the [internal user database](#local-authentication) (local authentication) or [external identity providers](#integration-with-external-authentication-providers). Connecting an external identity provider allows the use of existing credentials (e.g., LDAP, GitLab, GitHub, etc.) to access the system. It also enables the use of a single set of credentials to authenticate across multiple DKP clusters.

From the perspective of a cluster user or application developer, the way authentication is configured in DKP does not matter — the authentication interface and methods for enabling authentication in an application are the same.

DKP allows you to:

- Authenticate using local (static) [users and groups](#local-authentication) created in the cluster;
- [Integrate](#integration-with-external-auth-providers) with external identity systems;
- Enable [authentication in any web application](#general-integration-scheme) running in the cluster;
- Provide [authenticated access to the Kubernetes API](#accessing-the-kubernetes-api-via-load-balancer) via a load balancer.

## Accessing the Kubernetes API via load balancer

DKP also allows using authentication when accessing the Kubernetes API. In this case, users can generate a `kubeconfig` file via the DKP web interface for secure access to the Kubernetes API through a load balancer (Ingress controller).

To configure access, follow these steps:

1. Enable Kubernetes API publishing. Edit the `user-authn` module settings (create the ModuleConfig resource if it doesn't exist):

   ```console
   kubectl edit moduleconfig user-authn
   ```

   Add the following to the `settings` section:

   ```yaml
   publishAPI:
     enabled: true
   ```

1. Open the kubeconfig web interface. After enabling the `publishAPI` parameter in the `user-authn` module, DKP will automatically activate the kubeconfig generation web interface. It is available at:

   ```console
   https://kubeconfig.<publicDomainTemplate>
   ```

   For example, if `publicDomainTemplate` is `%s.kube.my`, the URL will be `https://kubeconfig.kube.my`.

1. Generate the `kubectl` configuration. After logging in to the kubeconfig interface, the user will receive a set of commands to configure `kubectl`. These commands can be copied and pasted into the terminal. Authentication will be performed using an OIDC token issued by Dex. If the provider supports session renewal, the configuration will include a `refresh token`, allowing access to be extended without re-authentication.

1. Configure multiple API endpoints. In the `user-authn` module configuration, you can define multiple connection points (kube-apiserver), each with its own description and CA certificate. This is useful if the cluster is accessible through different networks — for example, via VPN or a public IP:

   ```yaml
   settings:
     kubeconfigGenerator:
     - id: direct
       masterURI: https://159.89.5.247:6443
       description: "Direct access to kubernetes API"
   ```

### How access to the Kubernetes API is secured

In Deckhouse Kubernetes Platform, you can securely expose the Kubernetes API externally using an Ingress controller while maintaining full access control. API exposure and authentication are configured via the `user-authn` module. You can configure:

- A list of trusted IP addresses or networks allowed to access the API;
- A list of user groups permitted to authenticate;
- The Ingress controller through which access is provided.

To configure access:

1. Enable API publishing.
1. Set up access restrictions. In the module configuration, you can define:
   - A list of allowed network addresses (`allowedSourceRanges`);
   - A list of user groups allowed to connect to the Kubernetes API (`allowedUserGroups`);
   - The Ingress controller to use for API publishing (`ingressClass`).
1. Use the kubeconfig web interface. Users can securely access the Kubernetes API via a kubeconfig generated through the web interface (`https://kubeconfig.<publicDomainTemplate>`). This kubeconfig will include an OIDC token and connection settings through the Ingress.

What is automatically configured when API publishing is enabled:

- Deckhouse will automatically set the required arguments for the kube-apiserver;
- A CA certificate will be generated and added to the kubeconfig;
- Login through Dex with OIDC support will be configured.

## Integration with external authentication providers

Connecting an external authentication provider allows users to use a single set of credentials to authenticate across multiple DKP clusters. DKP supports connecting more than one authentication provider at the same time.

DKP supports integration with the following external providers and authentication protocols:

- [LDAP (e.g., Active Directory)](#ldap-integration);
- [OIDC (e.g., Okta, Keycloak, Gluu, Blitz Identity Provider)](#oidc-openid-connect-integration);
- [GitHub](#github-integration);
- [GitLab](#gitlab-integration);
- [Bitbucket Cloud](#bitbucket-cloud-integration);
- [Atlassian Crowd](#atlassian-crowd-integration);

{% alert level="info" %}
Password security policies (such as complexity requirements, expiration, history, two-factor authentication, etc.) are fully managed by the external authentication provider. Deckhouse does not manage passwords and does not interfere with provider-side policy enforcement.
{% endalert %}

### General integration scheme

1. Create an OAuth application with your authentication provider:
   - Specify the Redirect URI in the form of `https://dex.<publicDomainTemplate>/callback`;
   - Obtain the `clientID` and `clientSecret`.

     > **Warning**. When specifying the Redirect URI, substitute the actual `publicDomainTemplate` value without `%s`.  
     For example, if `publicDomainTemplate: '%s.sandbox1.deckhouse-docs.flant.com'` is set,  
     the actual URI would be `https://dex.sandbox20.deckhouse-docs.flant.com/callback`.

     > You can find the Dex address (URI) using the following command:

       ```console
       kubectl -n d8-user-authn get ingress dex -o jsonpath="{.spec.rules[*].host}"
       ```

1. Create a DexProvider resource based on the specifics of your chosen provider.
1. Enable the `user-authn` module (if it is not already enabled).

   The `user-authn` module can be enabled via the admin web interface or using the CLI.  
   The following example demonstrates enabling it through the CLI (requires `kubectl` configured for the cluster).

   Check the status of the module:

   ```shell
   kubectl get module user-authn
   ```

   Example output:

   ```console
   kubectl get module user-authn
   NAME         WEIGHT   SOURCE     PHASE   ENABLED   READY
   user-authn   150      Embedded   Ready   True      True
   ```

   Enable the module via CLI:

   ```shell
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable user-authn
   ```

1. Configure the module.

   - Open the `user-authn` module settings (create the `user-authn` ModuleConfig resource if it doesn't exist):

     ```shell
     kubectl edit mc user-authn
     ```

   - Specify the required module parameters in the `spec.settings` section.  
     For more details on possible configuration options, see the [module reference](#TODO).

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

Authentication via an OIDC provider requires client registration (or application creation). Follow your provider’s documentation for this process — for example [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration), or [Blitz](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html).

Specify the obtained `clientID` and `clientSecret` in the DexProvider resource.

{% alert level="info" %}
When registering an application with any OIDC provider, you must specify a redirect URI.  
To integrate with DexProvider, use the following format:  
`https://dex.<publicDomainTemplate>/callback`,  where `publicDomainTemplate` is the DNS name template for your cluster, defined in the `global` module.
{% endalert %}

{% alert level="info" %}
To ensure proper token revocation upon logout and to enforce re-authentication, set the `prompt` parameter to `login`.  
This guarantees that the user will be asked to re-enter credentials during re-authentication.
{% endalert %}

To configure fine-grained access control for users to applications:

- Add the `allowedUserGroups` parameter to the `ModuleConfig` of the target application;
- Assign the appropriate user groups, ensuring that the group names match both in the provider and in Deckhouse.

#### Keycloak

After selecting a `realm` for configuration, adding a user under [Users](https://www.keycloak.org/docs/latest/server_admin/index.html#assembly-managing-users_server_administration_guide), and creating a client under [Clients](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide) with [authentication](https://www.keycloak.org/docs/latest/server_admin/index.html#capability-config) enabled (required for generating the `clientSecret`), complete the following steps:

- Under [Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes), create a scope named `groups`, and assign the predefined `groups` mapper to it (Client scopes → Client scope details → Mappers → Add predefined mappers).
- In the previously created client, link this scope via the [Client scopes tab](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) (Clients → Client details → Client Scopes → Add client scope).
- In the client configuration fields "Valid redirect URIs", "Valid post logout redirect URIs", and "Web origins", specify:  
  `https://dex.<publicDomainTemplate>/*`,  
  where `publicDomainTemplate` refers to the DNS name template of your cluster as defined in the `global` module.

Example DexProvider configuration for integration with Keycloak:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: keycloak
spec:
  type: OIDC
  displayName: My Company Keycloak
  oidc:
    issuer: https://keycloak.my-company.com/realms/myrealm # Use your realm name.
    clientID: plainstring
    clientSecret: plainstring
    getUserInfo: true
    scopes:
      - openid
      - profile
      - email
      - groups
```

#### Blitz Identity Provider

Example DexProvider configuration for integration with Blitz Identity Provider:

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
      groups: your_claim # Claim for retrieving user groups, groups must be configured on the Blitz Identity Provider side.
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

Example DexProvider configuration for integration with Okta:

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

Example configuration for Prometheus:

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

To configure authentication, create a read-only service account in your LDAP directory. This account will be used to perform search queries within the LDAP directory.

In the DexProvider resource, specify the following parameters:

- `bindDN`: The full Distinguished Name (DN) of the created service account.  
  Example: `cn=readonly,dc=example,dc=org`.
- `bindPW`: The password for the specified `bindDN`.

{% alert level="info" %}
If your LDAP server allows anonymous access for performing search queries, the `bindDN` and `bindPW` parameters can be omitted. However, using authenticated access is recommended to enhance security.

The `bindPW` must be provided in plain text. Dex does not support hashed passwords in this field.
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

You need to create a new OAuth application in your GitHub organization.

Follow these steps:

- Navigate to "Settings" → "Developer settings" → "OAuth Apps" → "New OAuth App", and specify the "Authorization callback URL" as `https://dex.<publicDomainTemplate>/callback`.
- Use the generated `Client ID` and `Client Secret` in the DexProvider resource.

If the GitHub organization is managed by a customer:

- Go to "Settings" → "Applications" → "Authorized OAuth Apps" → `<name of created OAuth App>` and click "Send Request" to request approval.
- Ask the customer to confirm the approval request sent to their email.

Example configuration for GitHub integration:

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

Follow these steps:

- **Self-hosted GitLab**: Go to "Admin Area" → "Applications" → "New application" and specify the "Redirect URI (Callback URL)" as `https://dex.<publicDomainTemplate>/callback`. Select the scopes: `read_user`, `openid`.
- **gitlab.com (cloud)**: Under the main project account, go to "User Settings" → "Applications" → "Add new application" and specify the "Redirect URI (Callback URL)" as `https://dex.<publicDomainTemplate>/callback`. Select the scopes: `read_user`, `openid`.
- Use the generated "Application ID" and "Secret" in the DexProvider resource.

{% alert level="info" %}
For GitLab version 16 and higher, enable the "Trusted" option when creating the application. This option is available in "Admin Area" → "Applications". Marking the application as trusted allows skipping the authorization step for users, which can be useful in controlled environments.
{% endalert %}

Example configuration for GitLab integration:

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

### Atlassian Crowd Integration

You need to create a new Generic application in the appropriate Atlassian Crowd project.

Follow these steps:

- Go to "Applications" → "Add application";
- Use the generated "Application Name" and "Password" in the DexProvider resource;
- When specifying groups in the DexProvider, ensure the group names are in lowercase. This is required for proper group mapping between Crowd and Deckhouse.

Example configuration for integration with Atlassian Crowd:

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

### Bitbucket Cloud Integration

To configure authentication, you need to create a new "OAuth consumer" in your Bitbucket team's settings.

Follow these steps:

- Go to "Personal settings" → "Access management" → "OAuth consumers" → "Add consumer", and set the "Callback URL" to `https://dex.<publicDomainTemplate>/callback`;
- Grant the following permissions:  
  - "Account → Read": allows retrieval of basic user information (e.g., email, username);  
  - "Workspace membership → Read": allows retrieval of the user's workspace memberships;
- Use the generated "Key" and "Secret" in the DexProvider resource.

Example configuration for integration with Bitbucket:

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

## Local authentication

In addition to external authentication providers, DKP also supports local authentication.  
Local authentication means creating `User` and `Group` objects in the cluster for static users and groups.

1. Creating a static user.

   To create a static user, define a `User` resource.

   Example resource (note the usage of [ttl](https://deckhouse.io/documentation/v1/modules/user-authn/cr.html#user-v1-spec-ttl)):

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

   Choose a password and provide its hashed value in the `password` field.  
   The password is stored in encrypted form using `bcrypt`.

   You can generate the hash using the following command:

   ```console
   echo "$password" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
   ```

1. Adding a user to a group.

   To group static users, create a `Group` resource.

   Example:

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

   The `members` field lists users in the group (`kind`: `User`, with the corresponding user name).

   After creating the group and adding users, you must configure [authorization](authorization.html).
