# Infrastructure providers: download, unpack, validation

dhctl works with two kinds of cloud providers:

- **In-tree** (`aws`, `azure`, `gcp`, `vcd`, …): only their schemas, layouts and
  `terraform_versions.yml` ship in the deckhouse/candi image. The binaries they run are still
  downloaded at runtime, by this installer's digests: opentofu or terraform, and the
  terraform-manager plugin from the provider's in-tree image.
- **External** (`dvp`, `yandex`, …): everything they need — schemas, the terraform/opentofu plugin,
  the settings files and the validator binary — ships in a per-provider **OCI bundle**
  (`cloud-provider-<name>/terraform-manager`) that dhctl downloads and unpacks at runtime.

This document describes the external-provider path: when a bundle is downloaded, where it is
unpacked, how the validator is invoked, and how settings/plugins are read from it.

## When a bundle is downloaded

The decision is **schema-presence based**
(`config.IsInternalCloudProviderBundle`, `pkg/config/provider_bundle.go`). A provider needs no
bundle when the cluster is static (no provider) or when its cluster-config schema ships in candi
at `candi/cloud-providers/<provider>/openapi/cluster_configuration.yaml` (in-tree).

Every other provider gets a bundle. A parse of configuration documents
(`ensureProviderBundleFromConfig`) also gets one for an in-tree provider whose module the
documents pin: a `ModuleConfig` `cloud-provider-<name>` with `spec.source`, or a
`ModulePullOverride` `cloud-provider-<name>` with `spec.imageTag`. Otherwise dhctl would install
the pinned build while validating against the schemas of another build.

The bundle is downloaded only when `<download-root>/<provider>@<digest>` does not exist yet
(`image.EnsureUnpacked`). None of this depends on `candi/terraform_versions.yml`: removing a
provider's entry there does not change whether its bundle is fetched.

## Where downloading happens

| Entry point | Path | Registry source |
|---|---|---|
| `LoadConfigFromFile` → `ParseConfig` → `ensureProviderBundleFromConfig` | bootstrap, bootstrap abort, `config render …` | from the config's `registryDockerCfg` / default public registry |
| `ParseConfig` → `ensureProviderBundleFromConfig` | `config parse cluster-configuration --file`, the install-deckhouse bootstrap phase, `deckhouse create-deployment` | same |
| `ParseConfigFromDataEnsureProvider` → `ensureProviderBundleFromConfig` | commander parse that is handed no bundle directory | same |
| `commander.ParseMetaConfig` → `EnsureProviderBundleFromCluster` | commander check/converge/destroy | **upstream registry read from the target cluster** (registry-config) |
| `parseConfigFromCluster` → `updateProviderBundle` | parse from the cluster (`ParseConfigFromCluster`, `ParseConfigInCluster`) | `GetRegistryDataPreferUpstream`: the upstream registry outside the cluster, the cluster's registry inside it |

All of them end in `updateProviderBundle` → `unpackProviderBundle` → `loadBundleSchemas`
(`pkg/config/provider_bundle.go`). A bundle resolved through the module chain (below) is pulled
from the registry of the module's `ModuleSource` instead of the one in the table.

The out-of-cluster commander server must read the **upstream** registry from `registry-config`,
not the `registry.d8-system.svc` in-cluster mirror (unresolvable outside the cluster).

The bundle **digest** is decided before anything is downloaded:

- When the provider module is external (`ModuleDocs.providerIsExternal`, read from the
  configuration documents or from the cluster's `Module` object), the module chain answers
  (`resolveModuleProviderBundle` → `moduleBundleDigest`, `pkg/config/modulesource.go`). The tag
  of the module image comes from a `ModulePullOverride`, from the version the cluster's `Module`
  reports, or from the release image of the release channel. A HEAD request turns the tag into the
  image digest, and the `images_digests.json` of that image names the `terraformManager` bundle.
  The answer is recorded in `<download-root>/cloud-provider-<name>@<image digest>/provider-bundle-digest`,
  so a module image is read once per digest. An error here never falls back to the installer's
  digests: the installer's build is not the one the cluster runs.
- Otherwise the digest is this installer's `images_digests.json` entry
  (`digests.GetImage("cloudProvider<Name>", "terraformManager")`).

Commander records the bundle digest in the cluster state under the key `provider-bundle-digest`.
When the bundle cannot be delivered (the cluster or the registry is gone), only destroy goes on,
with the recorded bundle, and only while it is still unpacked on this machine
(`UseUnpackedProviderBundle`). A destroy that takes its `MetaConfig` from the state cache reloads
the schemas of the bundle that `MetaConfig` was parsed with (`RestoreProviderBundle`), and that
directory must still exist.

A CLI destroy has no recorded digest on its first parse. It needs the registry when the bundle is
not unpacked on this machine yet, and always when the provider module comes from a `ModuleSource`:
only a HEAD request to that registry tells which bundle the cluster runs.

## Where images are unpacked, and the bundle structure

Every image dhctl downloads lands under the **download root** (`GlobalOptions.DownloadDir`,
`--download-dir`, default `/tmp/dhctl`) in a directory named by its digest. The name comes from
`providerdir.DigestDir`, the unpacking from `image.EnsureUnpacked`:

```text
<download-root>/
  candi@<digest>/deckhouse/…      ← candi of this installer
  opentofu@<digest>/opentofu      ← infrastructure tools of this installer
  terraform@<digest>/terraform
  plugins/                        ← plugins dir of a dhctl image that has none baked in (plugin step 1)
  <provider>@<digest>/            ← the provider's terraform-manager image (immutable, digest-pinned)
    terraform-manager/
      terraform_versions.yml     ← provider settings (single-provider fragment; no `terraform:` key — inherits from candi)
      plan_rules.yml             ← vmResource rule (which resource change is a VM change)
      terraform-provider-<x>      ← the opentofu/terraform plugin binary
    validator                    ← the external validator binary
    openapi/                     ← cluster_configuration.yaml, cloud_discovery_data.yaml
    crd/  layouts/  terraform-modules/  candi/  cni-bootstrap.yml
  cloud-provider-<x>@<digest>/provider-bundle-digest
                                 ← which bundle the module image of that digest names
  <name>@<digest>.partial-*/     ← transient: an unpack in progress, renamed into place when done;
                                   one left by a killed run goes once it is an hour old (see below)
  <name>@<digest>.lock.0, .lock.1 ← lock files: whoever unpacks that digest holds one of them
```

No image is taken as "whatever is unpacked in the download root". A consumer knows the digest it
needs and gets the directory of exactly that digest:

- candi, opentofu, terraform and an in-tree provider's terraform-manager image take the digest
  from this installer's `images_digests.json`.
- A provider bundle takes it from the module chain only when the module is external or pinned.
  Otherwise `resolveProviderBundleRef` takes it from this installer's `images_digests.json`,
  which names the bundle of every provider this installer ships, DVP included. Either way its
  directory travels with the parsed configuration as `MetaConfig.ProviderBundleDir`.

Two operations with different bundle builds can share one download root: each reads its own
directory. The download root outlives a single dhctl run, so a re-run unpacks nothing again (it
still makes a HEAD request for the module image, and on a release channel reads the release
image). When the download root is the tmp dir itself (the default), the tmp cleaner keeps every
`<name>@<digest>` directory, the `.partial-*` directories younger than an hour (another process
may still be unpacking there) and the lock files. In any download root, the next unpack of a digest
that holds `.lock.0` removes that digest's `.partial-*` directories older than an hour.

Several dhctl processes can share the download root too (`dhctl server` runs every request in a
process of its own). When they need the same digest, one downloads and the rest wait for it: the
lock is a kernel `flock` on a file beside the directory, so it disappears together with a holder
that dies. A holder that is alive but stuck is waited for `lockWait` per lock file, up to twice
`lockWait` in all. A lock file that cannot be locked at all (a file system without `flock`) is
skipped with a warning.

The unpacked tree does not depend on the lock: every unpack goes into a uniquely named staging
directory and is renamed into place. Only the holder of `.lock.0` removes stale staging
directories, so a process that gave up waiting never deletes the one a slow holder still writes.

## How settings are read from the bundle

`fsprovider.loadOrGetStore` builds the provider settings store:

- the **candi** `terraform_versions.yml` is parsed and cached per process;
- the **bundle** in `MetaConfig.ProviderBundleDir` is merged fresh on every call
  (`mergeBundleSettings`) from `<bundle>/terraform-manager/terraform_versions.yml`. A provider
  already known from candi is left as-is (candi is authoritative); a bundle-only provider (e.g.
  `dvp`) is taken from the bundle, and its `plan_rules.yml` is attached as the `vmResource` rule.

The bundle directory comes from the `MetaConfig` and is fixed when the provider DI is created
(`fsprovider.GetDi`). Which providers a bundle describes is read from its fragment, not from the
directory name.

## How the terraform/opentofu plugin is found

`fsprovider.pluginsProvider.DownloadPlugin` (`plugins.go`) tries, in order:

1. the pre-baked plugins dir;
2. **`<MetaConfig.ProviderBundleDir>/terraform-manager/<binary>`** — the plugin from the bundle the
   configuration was parsed with. When a bundle dir is set, the plugin must be there: a missing
   one is an error naming the path, never a fall-through to step 3;
3. only when no bundle dir is set, the provider is in-tree: its terraform-manager image, named by
   this installer's digests, is unpacked into `<provider>@<digest>` and the plugin is taken from there.

Step 2 lets converge run without registry credentials on the MetaConfig — the plugin is already
on disk from the bundle.

## How the validator is called

Provider selection is in `meta_config_validator_provider.go` (`selectValidator`):

- `""` → no validation;
- `vcd` → its in-tree validator (`NewMetaConfigValidator`);
- otherwise → if `<MetaConfig.ProviderBundleDir>/validator` exists and is executable, the
  **external binary validator** (`external.Validate`); if the provider's schema is in candi, a
  default prefix check; otherwise an error.

The external validator runs the bundle's `validator` binary as a subprocess and calls it over
gRPC. Contract in full:
**[`go_lib/dhctl-provider-protocol/README.md`](../../../go_lib/dhctl-provider-protocol/README.md)**
(types in `go_lib/dhctl-provider-protocol/api/validate/v1`). Summary:

- **Invocation:** `<MetaConfig.ProviderBundleDir>/validator serve --network=tcp --address=127.0.0.1:0`.
  The kernel picks the port, the validator announces the endpoint it bound, dhctl reads that
  line out of the process output, calls it once and stops it with `SIGTERM`.
- **Transport:** gRPC, no TLS. dhctl logs both streams line by line as they arrive, at DEBUG,
  so they land in the debug log file rather than the terminal.
- **Input** (`validatev1.Input`, JSON inside `input_json`): `providerName`, `operation`
  (`bootstrap`/`converge`/`destroy`), `clusterPrefix`, `layout`, `providerClusterConfiguration`,
  and `vars` (`CloudProviderVars`: module `settings`, `nodeGroups`, `instanceClasses`, credential
  `secrets`) — the only channel for provider resources.
- **Output** (`validatev1.ValidateResponse`): `errors` block the operation, `warnings` are logged;
  an empty response means valid. A failure of the validator itself is a gRPC status
  (`InvalidArgument`, `Unimplemented`, `Internal`, `Unavailable`), never a violation. dhctl fails closed:
  any status other than `OK` blocks the operation. Validation **never mutates** the config.

`vcd`'s `legacyMode` rewrite is the one provider-side config mutation; it is **not** part of
validation — it is an explicit `vcd.EnsureLegacyMode` call in the infrastructure layer
(`cloud_provider.go`) before the provider is built.

## Worked example: DVP

DVP (Deckhouse Virtualization Platform) is the reference external provider. A converge with
`--download-dir /tmp/dhctl` leaves this on disk after the bundle is delivered:

```
/tmp/dhctl/
  dvp@sha256:09ae6685aed973ab2baa05f03d29ce77068852eb3c8db31c7f9dff7ab2014ad5/
    terraform-manager/
      terraform_versions.yml
      plan_rules.yml
      terraform-provider-kubernetes      # the opentofu plugin
    validator                            # the external validator binary
    openapi/{cluster_configuration.yaml,cloud_discovery_data.yaml}
    crd/  layouts/standard/  terraform-modules/  candi/module-openapi
  candi@sha256:<digest>/deckhouse/candi/ # extracted candi image (no dvp entry)
```

`terraform-manager/terraform_versions.yml` (the single-provider fragment — note there is no
`terraform:` key; it is inherited from candi):

```yaml
opentofu: 1.12.0
kubernetes:                # the key is the terraform provider id (hashicorp/kubernetes)
  namespace: hashicorp
  cloudName: DVP           # → the store is keyed by lowercased cloudName: "dvp"
  type: kubernetes
  version: "2.38.0"
  artifact: terraform-provider-kubernetes
  destinationBinary: terraform-provider-kubernetes
  vmResourceType: kubernetes_manifest
  useOpentofu: true
```

`terraform-manager/plan_rules.yml` — attached as the provider's `vmResource` rule, so a converge
only calls a `VirtualMachine` delete a VM change (not every `kubernetes_manifest` — disks, IPs):

```yaml
vmResource:
  type: kubernetes_manifest
  fieldEquals:
    path: manifest.kind
    value: VirtualMachine
```

### Validator invocation

dhctl runs `/tmp/dhctl/dvp@sha256:<digest>/validator serve --network=tcp --address=127.0.0.1:<port>`
and calls `dhctl.provider.validate.v1.ValidateService/Validate` with this payload in `input_json`:

```json
{
  "providerName": "dvp",
  "operation": "bootstrap",
  "clusterPrefix": "my-cluster",
  "layout": "Standard",
  "providerClusterConfiguration": {
    "apiVersion": "deckhouse.io/v1",
    "kind": "DVPClusterConfiguration",
    "layout": "Standard",
    "provider": { "kubeconfigDataBase64": "eyJ...", "namespace": "team-d8-candi" }
  },
  "vars": {
    "settings": { "region": "default" },
    "nodeGroups": { "worker": { "replicas": 1 } },
    "instanceClasses": { "worker": { "virtualMachine": { "cpu": { "cores": 4 } } } },
    "secrets": { "credentials": { "kubeconfig": "..." } }
  }
}
```

A valid configuration is an empty response. A failure comes back as
`errors: [{path: "...", code: "...", message: "namespace team-d8-candi not found"}]`, which dhctl
renders as `<path>: <message>` per line. A binary that exits before answering — an old one that
does not know `serve`, for instance — blocks the operation.

### End-to-end flow (converge)

1. `parseConfigFromCluster` / `commander.ParseMetaConfig` reads the registry from the target
   cluster, resolves the bundle digest and unpacks the bundle into `/tmp/dhctl/dvp@sha256:<digest>`
   (`updateProviderBundle` → `unpackProviderBundle` → `loadBundleSchemas`). That directory becomes
   `MetaConfig.ProviderBundleDir`. Nothing is written into `deckhouse/candi`.
2. The provider is built (`fsprovider.GetDi`): `mergeBundleSettings` reads
   `<MetaConfig.ProviderBundleDir>/terraform-manager/terraform_versions.yml` (+ `plan_rules.yml`)
   → `store["dvp"]`.
3. `selectValidator` finds `<MetaConfig.ProviderBundleDir>/validator` → spawns it on a loopback
   port and runs the validate protocol above.
4. `DownloadPlugin` links `<MetaConfig.ProviderBundleDir>/terraform-manager/terraform-provider-kubernetes`
   as the opentofu plugin — no registry pull needed.

> Steps 2–4 read `MetaConfig.ProviderBundleDir`, not the download root, so they use exactly the
> bundle step 1 validated the configuration against, even when the download root holds several
> `dvp@<digest>` directories.
