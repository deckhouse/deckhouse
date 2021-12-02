## Patches

### Client allowed groups

With this patch, Dex can authorize requests for specific `OAuth2Client`s based on user's groups. 
We use it in Dex authenticators to make `allowedGroups` option to work.

This problem is not solved in upstream, and our patch will not be accepted.

### Static user groups

Allows setting groups for the `User` kind. It makes convenient authenticating as user alongside having another IdP.

This problem is not solved in upstream, and our patch will not be accepted.

### Concurrent requests fix

Rotating refresh token with concurrent requests may lead to invalidation if the token.
As for now, Dex updates the lastUsed field of OfflineSession on every refresh token touching. 
It causes unnecessary conflict errors for etcd and Kubernetes storages.

Upstream PR - https://github.com/dexidp/dex/pull/2300/
