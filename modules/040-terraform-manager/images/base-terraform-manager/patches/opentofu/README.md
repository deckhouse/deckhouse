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

Also, for dvp cloud provider we added skip changes for calculate pending data source deps.
In this patch we skip changes in labels and annotations.
We need it because changes in labels and annotations provide pending deps for data sources.
When opentofu re-read data source, we got full re-read object
```
 + object      = (known after apply)
```
for ip for example.
Because we get IP-address for virtual machine from data source, because address will available
only after full creating (k8s controller set address for resource) and IP-address has in 
destructive hash for virtual machines and IP data source not fully known before re-reading,
we get destructive changes for virtual machine.
Unfortunately, we cannot patch provider, because opentofu makes a decision about
re-reading data source depends on resource change.
Also, this changes have direct link to provider. We do not come up with generic solution.
Generic solution can add some meta-attributes in opentofu data source resource, but it complex
for implementation.
