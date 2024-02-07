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

### Connector data patch

There is a bug in Dex that it saves connector data to the refresh token object and reads it first then the date from offline session.

Upstream PR - https://github.com/dexidp/dex/pull/2729.

### OIDC RootCA and InsecureSkipVerify

Allows OIDC connector to work with providers using self-signed certificates.

Upstream PR that should fix the problem in general - https://github.com/dexidp/dex/pull/1632.

### Robots.txt

Add robots.txt to avoid indexing by bots.

Upstream PR  - https://github.com/dexidp/dex/pull/2834

### 401 code for password auth

Return 401 instead of 200 if a password authentication attempt failed.

Upstream PR  - https://github.com/dexidp/dex/pull/2796
