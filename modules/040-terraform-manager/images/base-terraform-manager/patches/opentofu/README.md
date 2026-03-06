## 001-go-mod.patch

bump packages version for fix cve

## 002-force-persist-state.patch

By default, opentofu persists state every 20 seconds.
And it is minimum if we try to redeclare with env opentofu
will return error. We set default to 0 and remove additional
checks for this duration.

## 003-skip-some-depends_on-for-data-sources.patch

In some cases, when we add new resource to depends_on meta-argument, we can get
destructive changes. For example, when we added `kubernetes_resource_ready_v1` resource
to cloud provider dvp we should add this resource as dependency to data source, otherwise
opentofu reads not ready resource. In updating deckhouse, user could get destructive plan 
with recreating all vm's. To avoid it, we patched opentofu.
In this patch, we provide `TF_SKIP_DEPS_FOR_DATA_SOURCES_PROVIDER` for filter providers that
use skipping depend on's in data sources (now we need it in dvp, and we should not affect another
providers). Also, we handle `TF_SKIP_DEPS_FOR_DATA_SOURCES` that contains `;` separated
resources for skip in data sources. Unfortunately we cannot get full diff of data source
in place when we check deps. Opentofu produce diff change (and re-read) for data source if depended on
resource has no operations (it called pending update). And now, we are doing our check
when if we have only one pending dependencies, if has multiple - mark as pending update.
