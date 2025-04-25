---
title: "OIDC Provider Keycloak"
permalink: en/stronghold/documentation/user/auth/jwt/oidc-providers/keycloak.html
lang: en
description: OIDC provider configuration for Keycloak
---

## Keycloak

1. Select/create a Realm and Client. Select a Client and visit Settings.
1. Client Protocol: openid-connect
1. Access Type: confidential
1. Standard Flow Enabled: On
1. Configure Valid Redirect URIs.
1. Save.
1. Visit Credentials. Select Client ID and Secret and note the generated secret.
