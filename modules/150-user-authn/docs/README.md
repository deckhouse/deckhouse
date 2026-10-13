---
title: "Module user-authn"
description: "A unified authentication system for Deckhouse Platform, integrated with Kubernetes and Deckhouse Platform web interfaces."
---

The module sets up a unified authentication system integrated with Kubernetes and Web interfaces used in other modules (Grafana, Dashboard, etc.).

## Main features

The module provides the following features:

- Single sign-on to the web interfaces of Deckhouse Platform (DP) modules and to the Kubernetes API through Dex
- Sign-in through external providers: GitHub, GitLab, Bitbucket Cloud, Atlassian Crowd, LDAP (with Kerberos SSO), OIDC and SAML 2.0
- Static users and groups with a password policy, account lockout and two-factor authentication
- Authentication for any web application in the cluster with DexAuthenticator, and OAuth2 clients for applications that support OIDC (DexClient)
- Kubeconfig generation in the Deckhouse web UI for access with `kubectl` and OIDC tokens
- Basic authentication to the Kubernetes API through one Crowd, OIDC or LDAP provider

## Components

The module consists of the following components:

- [dex](https://github.com/dexidp/dex) — is a federated OpenID Connect provider that acts as an identity service for static users and can be linked to one or more ID providers (e.g., SAML providers, GitHub, and GitLab);
- `dex-authenticator` (based on [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy)) authenticates requests to an application published through an Ingress or, with the `alb` module, through Gateway API.

Management of static users is performed using the [User](cr.html#user) and [Group](cr.html#group) resources:

- The User object stores user information, including the email address and a password hash (the plain-text password is not stored);
- The Group object defines a list of users grouped together for easier access management.

User management operations — creating and deleting users, managing groups, resetting passwords, resetting 2FA, and locking/unlocking — can be performed via the CLI using the [`d8 iam`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam) command group or via the [UserOperation](cr.html#useroperation) resource. The `UserOperation` resource is single-use and is automatically deleted 24 hours after completion.

The following external authentication protocols/providers are supported:

- GitHub;
- GitLab;
- BitBucket Cloud;
- Crowd;
- LDAP;
- OIDC;
- SAML 2.0.

You can use several external authentication providers simultaneously.

## Integration features

OIDC authentication in `kube-apiserver` is configured automatically by the [`control-plane-manager`](/modules/control-plane-manager/) module. For details, see [Configuring kube-apiserver](faq.html#configuring-kube-apiserver).

{% alert level="warning" %}
When the module is enabled, the web interfaces of DP modules authenticate users through Dex instead of HTTP Basic Auth.
{% endalert %}

### Basic authentication for the Kubernetes API requests

[Basic authentication](https://en.wikipedia.org/wiki/Basic_access_authentication) for Kubernetes API requests is available for the Crowd, OIDC and LDAP providers (the `enableBasicAuth` parameter in the [crowd](cr.html#dexprovider-v1-spec-crowd-enablebasicauth), [oidc](cr.html#dexprovider-v1-spec-oidc-enablebasicauth) or [ldap](cr.html#dexprovider-v1-spec-ldap-enablebasicauth) section). It can be enabled for only one provider in the cluster. For a Crowd or OIDC provider, Basic Authentication bypasses the [identity limits of the provider](usage.html#limiting-the-identities-of-a-provider), so only a SuperAdmin can enable it.

> You can also connect to the Kubernetes API [via other supported external providers](#generating-kubeconfig-files).

### Application integration

To add authentication to any web application in the cluster, create a [DexAuthenticator](cr.html#dexauthenticator) in the namespace of the application and add the authentication annotations to its Ingress or HTTPRoute. DexAuthenticator can limit the groups and emails that are allowed to sign in and the networks from which users can authenticate.

If the application supports OIDC itself, create a [DexClient](cr.html#dexclient) in its namespace instead. The module registers an OAuth2 client in Dex and creates the `dex-client-<NAME>` Secret with the client secret in the same namespace. DexClient can limit the groups and emails that are allowed to sign in and list the clients whose tokens the application trusts (`trustedPeers`).

### Generating kubeconfig files

After signing in, a user can generate a kubeconfig for kubectl and other Kubernetes tools in the [Deckhouse web UI](/products/kubernetes-platform/documentation/v1/user/web/ui.html).
The kubeconfig uses an OIDC token. If the authentication provider supports session extension, the kubeconfig also contains a refresh token, and the session is extended automatically.

The [`kubeconfigGenerator`](configuration.html#parameters-kubeconfiggenerator) parameter adds other ways to reach the API server, each with its own address and CA certificate, for example, for access through a bastion host or a VPN.

## Additional Dex capabilities

The module ships Dex with additional capabilities:

* Groups for static users (the [Group](cr.html#group) object)
* Two-factor authentication (2FA) for static users (the [`staticUsers2FA`](configuration.html#parameters-staticusers2fa) parameter)
* A password policy for static users: complexity, password history, rotation and lockout after failed sign-in attempts (the [`passwordPolicy`](configuration.html#parameters-passwordpolicy) parameter)
* A forced password change at the next sign-in after an administrator resets the password
* Lockout after failed sign-in attempts for LDAP and Atlassian Crowd users (the [`passwordPolicy.lockout.applyToConnectors`](configuration.html#parameters-passwordpolicy-lockout-applytoconnectors) parameter)
* Per-IP rate limiting of failed sign-in attempts (the [`rateLimit`](configuration.html#parameters-ratelimit) parameter)
* Locking and unlocking users with the [UserOperation](cr.html#useroperation) object, also without a password policy
* Lists of allowed users and groups in [DexClient](cr.html#dexclient) and [DexAuthenticator](cr.html#dexauthenticator) that Dex checks at sign-in and at every refresh-token request
* Limits on the identities a provider may assert (the [`spec.allowedIdentities`](cr.html#dexprovider-v1-spec-allowedidentities) parameter of DexProvider)
* Kerberos (SPNEGO) sign-in through the LDAP provider without the password form, with tickets from the `Authorization: Negotiate` header validated by the service keytab
* Refresh tokens for the SAML provider
* A custom CA certificate for the GitLab provider (the [`gitlab.rootCAData`](cr.html#dexprovider-v1-spec-gitlab-rootcadata) parameter)

## High availability mode

In high availability mode ([`highAvailability`](configuration.html#parameters-highavailability)), Dex and the other module components run in several replicas, so sign-in keeps working if one pod fails. DexAuthenticator follows the same setting unless its own [`spec.highAvailability`](cr.html#dexauthenticator-v1-spec-highavailability) is set.
