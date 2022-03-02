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
