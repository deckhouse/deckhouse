---
title: Authentication
permalink: en/architecture/iam/authentication.html
search: authentication, kubeconfig, user authentication, access control, kubernetes API access
description: Authentication features in Deckhouse Platform.
---

## Connecting to Kubernetes API using a generated kubeconfig

![Interaction scheme when connecting to the Kubernetes API using a generated kubeconfig](../../images/user-authn/kubeconfig_dex.svg)

1. **Initialization**. kube-apiserver reads the configuration of the OIDC provider (in this case — Dex): the `issuer` and the JWKS endpoint for token validation. In Kubernetes 1.30 and later, it reads it through the Dex Service inside the cluster.

1. **Kubeconfig generation**. The Deckhouse Platform (DP) web UI generates a kubeconfig file that includes an `ID token` and a `refresh token`. This file is used by `kubectl` or other Kubernetes clients.

1. **Authentication when accessing the API**. Upon receiving a request with an `ID token`, the `kube-apiserver` verifies the token's signature using keys from the JWKS endpoint. It then compares the `iss` (issuer) and `aud` (audience) claims in the token against the server configuration.

## Protection against credential brute-forcing in Dex

Dex limits failed sign-in attempts per client IP address on the password endpoints. Configure the limit with the [`rateLimit`](/modules/user-authn/configuration.html#parameters-ratelimit) parameter. It is turned off by default.

Dex also locks an account after failed sign-in attempts. Configure the lockout with the [`passwordPolicy.lockout`](/modules/user-authn/configuration.html#parameters-passwordpolicy-lockout) parameter. It applies to local users, and to LDAP and Crowd users if their types are listed in [`applyToConnectors`](/modules/user-authn/configuration.html#parameters-passwordpolicy-lockout-applytoconnectors).

## Authentication via DexAuthenticator

![How authentication via DexAuthenticator works](../../images/user-authn/dex_login.svg)

1. **Login process via Dex**. In most cases, Dex redirects the user to the login page of an external identity provider (e.g., GitHub, Okta, Keycloak), then expects the user to return to the `/callback` URL after successful authentication.  
   However, for providers like LDAP or Atlassian Crowd, this flow is not supported. Instead, the user enters their login and password in the Dex login form, and Dex performs authentication by calling the provider's API directly.

1. **Token and session storage**. DexAuthenticator sets a cookie containing the full `refresh token`, rather than issuing a short-lived ticket as with the `ID token`.  
   This is because Redis, used by DexAuthenticator, does not persist data to disk.  
   If the `ID token` is missing from Redis, the user can obtain a new one using the `refresh token` stored in the cookie.

1. **Passing the token to the application**. DexAuthenticator sets the `Authorization` HTTP header with the `ID token` from Redis.  
   This may be optional for some applications, such as Upmeter, where other authorization mechanisms are used.  
   However, for applications like the Kubernetes Dashboard, this behavior is critical, as the Dashboard forwards the `ID token` to access the Kubernetes API on behalf of the user.

## DP-specific Dex extensions

DP ships Dex with additional capabilities:

* Groups for static users (the [Group](/modules/user-authn/cr.html#group) object).
* Two-factor authentication (2FA) for static users (the [`staticUsers2FA`](/modules/user-authn/configuration.html#parameters-staticusers2fa) parameter).
* A password policy for static users: complexity, password history, rotation and lockout after failed sign-in attempts (the [`passwordPolicy`](/modules/user-authn/configuration.html#parameters-passwordpolicy) parameter).
* A forced password change at the next sign-in after an administrator resets the password.
* Lockout after failed sign-in attempts for LDAP and Atlassian Crowd users (the [`passwordPolicy.lockout.applyToConnectors`](/modules/user-authn/configuration.html#parameters-passwordpolicy-lockout-applytoconnectors) parameter).
* Per-IP rate limiting of failed sign-in attempts (the [`rateLimit`](/modules/user-authn/configuration.html#parameters-ratelimit) parameter).
* Locking and unlocking users with the [UserOperation](/modules/user-authn/cr.html#useroperation) object, also without a password policy.
* Lists of allowed users and groups in [DexClient](/modules/user-authn/cr.html#dexclient) and [DexAuthenticator](/modules/user-authn/cr.html#dexauthenticator) that Dex checks at sign-in and at every refresh-token request.
* Limits on the identities a provider may assert (the [`spec.allowedIdentities`](/modules/user-authn/cr.html#dexprovider-v1-spec-allowedidentities) parameter of DexProvider).
* Kerberos (SPNEGO) sign-in through the LDAP provider without the password form, with tickets from the `Authorization: Negotiate` header validated by the service keytab.
* Refresh tokens for the SAML provider.
* A custom CA certificate for the GitLab provider (the [`gitlab.rootCAData`](/modules/user-authn/cr.html#dexprovider-v1-spec-gitlab-rootcadata) parameter).

## High availability mode

In high availability mode ([`highAvailability`](/modules/user-authn/configuration.html#parameters-highavailability)), Dex and the other `user-authn` components run in several replicas, so sign-in keeps working if one pod fails. DexAuthenticator follows the same setting unless its own [`spec.highAvailability`](/modules/user-authn/cr.html#dexauthenticator-v1-spec-highavailability) is set.
