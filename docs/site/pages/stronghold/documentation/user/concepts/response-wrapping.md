---
title: "Response wrapping"
permalink: en/stronghold/documentation/user/concepts/response-wrapping.html
lang: en
description: Response wrapping in Cubbyhole storage for secure distribution.
---

In many deployment scenarios of Deckhouse Stronghold, clients interact directly with Stronghold and use the returned secrets. However, in some cases, it may be more appropriate to separate privileges so that one trusted party interacts with most of the Stronghold API and then passes the secrets to the final consumer.

The more intermediaries involved in secret transmission, the higher the risk of accidental disclosure, especially if the secret is transmitted in plaintext. For example, you may need to deliver a private TLS key to a machine where, due to security policy, the decryption key can't be stored in persistent storage. In this case, encrypting the key before transfer is not possible.

To address such scenarios, Stronghold provides the _response wrapping_ feature. Instead of returning the response directly to the HTTP client, Stronghold stores it in the [`cubbyhole`](../secrets-engines/cubbyhole.html). Access to the stored content is granted only through a one-time token that Stronghold returns to the client.

From a logical perspective, the response is wrapped in a token that must be unwrapped to access the data. From a functional perspective, the token authorizes access to Stronghold's "key holder" and allows the data to be decrypted.

This method provides a reliable mechanism for secure information exchange across various environments. Response wrapping solves three key tasks:

- **Hiding sensitive information**. The value transmitted over the network is not the secret itself but a token to access it. Even if intercepted or logged, it does not contain confidential information.
- **Abuse detection**. A token can only be unwrapped by a single party. If a client receives a token that cannot be unwrapped, it is a reason to initiate a security incident investigation. Before unwrapping, the client can verify the token's origin through Stronghold.
- **Expiration enforcement**. A token has its own time-to-live (TTL), separate from the TTL of the secret (and usually much shorter). If the client does not unwrap the token in time, it expires.

## Response wrapping tokens

When wrapping is applied, Stronghold does not return the original API response directly. Instead, the client receives the following token metadata:

- TTL — token lifetime.
- Token — token value.
- Creation time — time when the token was generated.
- Creation path — API endpoint that triggered the response generation.
- Wrapped token ID — indicates the wrapped token (for example, during authentication or token creation). This is useful for orchestration systems (such as Nomad), as it allows token lifetime management without disclosing the token.

Currently, Stronghold does not support signing of response wrapping tokens, since it does not provide significant additional protection. If the server endpoint is correct, token validation is performed by interacting with Stronghold itself.
A signed token would not eliminate the need to verify it with the server, because the token does not contain the sensitive data itself—it is only a mechanism to access it. Therefore, Stronghold will not return the data without confirming the token's validity.

Even if an attacker redirects a client to a fake server, they can substitute the public signing key.
In theory, you could cache a previously valid key, but that would also require caching the previously valid address (in most cases, the Stronghold address will not change or will be set via service discovery).

Thus, Stronghold relies on the fact that the token does not contain sensitive data and therefore does not require signing.

## Operations with wrapping tokens

The following operations are available under the `sys/wrapping/` path:

- **Lookup** (`sys/wrapping/lookup`): Returns the token's creation time, path, and TTL. The path does not require authentication.
  The token holder can always require its properties via this endpoint.
- **Unwrap** (`sys/wrapping/unwrap`): Unwraps the token, returning the stored response in its original API format.
- **Rewrap** (`sys/wrapping/rewrap`): Rewraps already wrapped data into a new token. This is useful for secrets with long lifetimes. For example, an organization may want (or be required by security policy) to have the backend `pki` root CA key returned in a long-lived wrapping token—ensuring the key is never exposed (which can be verified via token lookup)—but still needs access to the key to sign a CRL if the `pki` mount is changed or lost. Security policies often require secret rotation, and this operation enables it without risk of exposure.
- **Wrap** (`sys/wrapping/wrap`): Returns data wrapped in a token.
  > Even if access to this path is restricted, wrapping can still be performed through other Stronghold API methods.

## Creating a wrapping token

Response wrapping is performed per request and is triggered by specifying the desired TTL.
The TTL is set by the client using the `X-Vault-Wrap-TTL` header and can be either an integer (in seconds) or a string (`15s`, `20m`, `25h`).
In the Stronghold CLI, this is done with the `-wrap-ttl` option.
In the Go API, the [`SetWrappingLookupFunc`](https://godoc.org/github.com/hashicorp/vault/api#Client.SetWrappingLookupFunc) function is used. It instructs the API under which conditions wrapping should be requested by matching the operation and path with the desired TTL.

Wrapping process:

1. The original HTTP response is serialized.
2. A one-time token is generated with the TTL set by the client.
3. The response is stored in the `cubbyhole` associated with the token.
4. A new response is generated with additional fields containing token information (ID, TTL, and path).
5. The new response is returned to the client.

{% alert level="info" %}
The minimum and maximum TTL of wrapping tokens is controlled by [policies](policy.html).
{% endalert %}

## Verifying a wrapping token

To reduce abuse risks, it is recommended to validate wrapping tokens using the following procedure:

1. If the client expected a token but did not receive one, it may have been intercepted by an attacker. Start an incident investigation.
1. Perform a lookup on the token. If the token has expired or been revoked, this does not always mean a leak (for example, the client started too late). However, investigation is still required. Using the Stronghold audit log, check whether the token was unwrapped.
1. Compare the token’s creation path with the expected one. For example, if you expected a TLS key or certificate after unwrapping, the path should likely be `pki/issue/...`. A mismatch may indicate token substitution (most likely if the path starts with `cubbyhole` or `sys/wrapping/wrap`).
   > Pay special attention to the `kv` secrets engine. If you expect the secret to come from `secret/foo`, but an attacker passes a token with the path `secret/bar`, simply checking the `secret/` prefix is insufficient.
1. After verifying the prefix, unwrap the token. If unwrapping fails, initiate an incident investigation.

By following these steps, you can be sure that only the intended client has seen the data inside the wrapping token, and any attempt at substitution or interception will be detected.
