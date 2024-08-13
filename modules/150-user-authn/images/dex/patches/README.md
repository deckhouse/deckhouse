## Patches

### Client allowed groups

With this patch, Dex can authorize requests for specific `OAuth2Client`s based on user's groups.
We use it in Dex authenticators to make `allowedGroups` option to work.

This problem is not solved in upstream, and our patch will not be accepted.

### Static user groups

Allows setting groups for the `User` kind. It makes convenient authenticating as user alongside having another IdP.

This problem is not solved in upstream, and our patch will not be accepted.

### Gitlab refresh context

Refresh can be called only one. By propagating a context of the user request, refresh can accidentally canceled.

To avoid this, this patch makes refresh requests to declare and utilize their own contexts.
