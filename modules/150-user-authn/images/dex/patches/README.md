## Patches

### 001-go-mod.patch

Update go mod for fix vuln's

### 002-bytes-and-string-certificates.patch

TODO: add description

### 003-client-filters.patch

With this patch, Dex can authorize requests for specific `OAuth2Client`s based on username and user's groups.
We use it in Dex authenticators to make `allowedUsers` and `allowedGroups` option to work.

This problem is not solved in upstream, and our patch will not be accepted.

### 004-fix-offline-session-updates.patch

Offline session is not created if the skip approval option is toggled. In this case Dex looses connector data and cannot refresh tokens.

Upstream PR: https://github.com/dexidp/dex/pull/3828

### 005-gitlab-refresh-context.patch

Refresh can be called only one. By propagating a context of the user request, refresh can accidentally canceled.

To avoid this, this patch makes refresh requests to declare and utilize their own contexts.

### 006-static-user-groups.patch

Allows setting groups for the `User` kind. It makes convenient authenticating as user alongside having another IdP.

This problem is not solved in upstream, and our patch will not be accepted.
