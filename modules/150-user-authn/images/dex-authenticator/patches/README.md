## Patches

### Cookie refresh

There is a problem when we are using nonpersistent Redis for session storage. If Redis was killed or flushed, the user should be authenticated again. That makes oauth2 proxy to be stateful application (or sort of).
Storing refresh token in cookie adds the possibility to restore access- and id- token even if there is no data in Redis.

Upstream PR - https://github.com/oauth2-proxy/oauth2-proxy/pull/313
