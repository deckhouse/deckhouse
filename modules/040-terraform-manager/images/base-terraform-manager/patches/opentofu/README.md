## 001-go-mod.patch

bump packages version for fix cve

## 002-force-persist-state.patch

By default, opentofu persists state every 20 seconds.
And it is minimum if we try to redeclare with env opentofu
will return error. We set default to 0 and remove additional
checks for this duration.
