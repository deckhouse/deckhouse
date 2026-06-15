---
title: Authentication
permalink: en/architecture/iam/authentication.html
search: authentication, kubeconfig, user authentication, access control, kubernetes API access
description: Authentication features in Deckhouse Kubernetes Platform.
---

## Connecting to Kubernetes API using a generated kubeconfig

![Interaction scheme when connecting to the Kubernetes API using a generated kubeconfig](../../images/user-authn/kubeconfig_dex.svg)

1. **Initialization**. Before the kube-apiserver starts, it requests the configuration endpoint of the OIDC provider (in this case — Dex) to retrieve the `issuer` and JWKS endpoint settings for token validation.

1. **Kubeconfig generation**. The Deckhouse Kubernetes Platform (DKP) web UI generates a kubeconfig file that includes an `ID token` and a `refresh token`. This file is used by `kubectl` or other Kubernetes clients.

1. **Authentication when accessing the API**. Upon receiving a request with an `ID token`, the `kube-apiserver` verifies the token's signature using keys from the JWKS endpoint. It then compares the `iss` (issuer) and `aud` (audience) claims in the token against the server configuration.

## Protection against credential brute-forcing in Dex

Each user is allowed a maximum of 20 login attempts. Once this limit is reached, an additional attempt is allowed every 6 seconds.

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

## DKP-specific Dex extensions

DKP uses a modified version of Dex that supports:

* Groups for static user accounts and the Bitbucket Cloud provider (via the [`bitbucketCloud`](/modules/user-authn/cr.html#dexprovider-v1-spec-bitbucketcloud) parameter).
* Passing the `group` claim to clients.
* The `obsolete tokens` mechanism, which helps prevent race conditions when refreshing tokens in OIDC clients.

## High availability mode

DKP supports a high-availability mode via the `highAvailability` setting.  
When enabled, multiple authenticator instances are deployed with redundancy to ensure continuous service.  
If any of the authenticators fail, active user authentication sessions are preserved and remain uninterrupted.
