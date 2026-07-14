## Patches

### 001-go-mod.patch

Update dependencies

### 002-fix-reauth-catalog.patch

Fix stale service catalog after re-authentication. Override `ReauthFunc` to re-authenticate the main provider directly instead of using a throw-away client, so both the token and `EndpointLocator` are refreshed on re-auth.
