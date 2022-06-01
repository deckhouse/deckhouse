## Patches

### Client allowed groups

With this patch, Dex can authorize requests for specific `OAuth2Client`s based on user's groups. 
We use it in Dex authenticators to make `allowedGroups` option to work.

This problem is not solved in upstream, and our patch will not be accepted.

### Static user groups

Allows setting groups for the `User` kind. It makes convenient authenticating as user alongside having another IdP.

This problem is not solved in upstream, and our patch will not be accepted.

### Gitlab refresh tokens

Previously, Dex assumed that Gitlab access tokens had no expiration date. Gitlab fixed this issue. Now it has the option to set TTL for tokens.
The patch fixes this by storing Gitlab refresh tokens for future token updates.

https://github.com/dexidp/dex/pull/2352

### Call connector refresh method only once

If Dex receives many concurrent requests, it will send refresh request to an external provider for each.
It does not work for providers that only allow refreshing once, e.g., Gitlab (because Gitlab rotates refresh token).

Now Dex uses annotation lock for Kubernetes storage to achieve this behavior.
Only one request does actual refreshing, others will wait for refreshing completion to read the result from the Kubernetes storage.

GitHub issue: TBA (since the problem is too complicated)
