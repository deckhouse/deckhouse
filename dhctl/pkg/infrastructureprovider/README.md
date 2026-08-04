# Infrastructure providers: download, unpack, validation

dhctl works with two kinds of cloud providers:

- **In-tree** (`aws`, `azure`, `gcp`, `yandex`, …): their schemas, `terraform_versions.yml`
  and validators are baked into the deckhouse/candi image. Nothing is downloaded at runtime.
- **External** (`dvp`, …): everything they need — schemas, the terraform/opentofu plugin,
  the settings files and the validator binary — ships in a per-provider **OCI bundle**
  (`cloud-provider-<name>/terraform-manager`) that dhctl downloads and unpacks at runtime.

This document describes the external-provider path: when a bundle is downloaded, where it is
unpacked, how the validator is invoked, and how settings/plugins are read from it.

## When a bundle is downloaded

The decision is **schema-presence based**, not config based
(`config.providerCandiPresent`, `pkg/config/base.go`):

```go
needProviderCandi := cloudProvider != "" && !providerCandiPresent(cloudProvider, globalOptions)
```

`providerCandiPresent(provider)` returns true when the provider is already usable on disk:

- its cluster-config schema exists at `candi/cloud-providers/<provider>/openapi/cluster_configuration.yaml`
  (in-tree), **or**
- the unpacked bundle already exists — the schema at
  `<download-root>/<provider>/openapi/cluster_configuration.yaml`, or, for external providers,
  the `<download-root>/<provider>/validator` binary.

So an external provider (no candi schema, bundle not yet unpacked) → `needProviderCandi = true`
→ the bundle is downloaded. This is independent of `candi/terraform_versions.yml`: removing a
provider's entry there does not change whether its bundle is fetched.

## Where downloading happens

| Entry point | Path | Registry source |
|---|---|---|
| `LoadConfigFromFile` → `EnsureProviderBundle` (`base.go:78`) | file bootstrap | from the config's `registryDockerCfg` / default public registry |
| `ParseConfigFromDataEnsureProvider` → `EnsureProviderBundle` (`base.go:542`) | commander data parse | same |
| `EnsureExternalProviderBundle` (`base.go`, via `commander.ParseMetaConfig`) | commander check/converge | **upstream registry read from the target cluster** (registry-config) |
| `parseConfigFromCluster` → `ensureProviderBundle` (`base.go:246`) | in-cluster parse (exporter, auto-converger, attacher) | `GetRegistryData` (in-cluster registry) |

The out-of-cluster commander server must read the **upstream** registry from `registry-config`,
not the `registry.d8-system.svc` in-cluster mirror (unresolvable outside the cluster).

The bundle **digest** is resolved from `images_digests.json`, not from `terraform_versions.yml`
(`resolveProviderBundleDigest` → `digests.GetImage("cloudProvider<Name>", "terraformManager")`).

## Where the bundle is unpacked, and its structure

Everything lands under the **download root** (`GlobalOptions.DownloadDir`, `--download-dir`,
default `/tmp/dhctl`). Paths come from `pkg/infrastructureprovider/providerdir`:

```
<download-root>/
  <provider>@<digest>/          ← ProviderDigestDir: the unpacked bundle (immutable, digest-pinned)
    terraform-manager/
      terraform_versions.yml     ← provider settings (single-provider fragment; no `terraform:` key — inherits from candi)
      plan_rules.yml             ← vmResource rule (which resource change is a VM change)
      terraform-provider-<x>      ← the opentofu/terraform plugin binary
    validator                    ← the external validator binary
    openapi/                     ← cluster_configuration.yaml, cloud_discovery_data.yaml
    crd/  layouts/  terraform-modules/  candi/  cni-bootstrap.yml
  <provider>@<digest>.partial/   ← transient: the download target before it is atomically renamed
  <provider> -> <provider>@<digest>   ← ProviderDir: symlink to the current digest dir
```

`unpackProviderBundle` (`base.go`) downloads into `<provider>@<digest>.partial`, renames it to
`<provider>@<digest>`, loads its schemas, then points the `<provider>` symlink at it
(`switchProviderSymlink`). Everything downstream reads **through the `<provider>` symlink**, so
stale digest dirs of previously delivered versions and unfinished `.partial` dirs are ignored.

The download root outlives a single dhctl run; in e2e it is mounted into every scenario
container. Bundles are digest-addressed, so a re-run reuses an already-unpacked bundle.

## How settings are read from the bundle

`fsprovider.loadOrGetStore` builds the provider settings store:

- the **candi** `terraform_versions.yml` is parsed and cached per process;
- **bundles** are merged fresh on every call (`mergeBundleSettings`) by globbing
  `<download-root>/*/terraform-manager/terraform_versions.yml` **through the `<provider>` symlink**
  (`@`-containing digest dirs are skipped). A provider already known from candi is left as-is
  (candi is authoritative); a bundle-only provider (e.g. `dvp` once its candi entry is removed)
  is taken from the bundle, and its `plan_rules.yml` is attached as the `vmResource` rule.

Merging fresh each call keeps a long-lived process (dhctl-server, converge exporter) from
returning a store built before the bundle was delivered.

## How the terraform/opentofu plugin is found

`fsprovider.pluginsProvider.DownloadPlugin` (`plugins.go`) tries, in order:

1. the pre-baked plugins dir;
2. `<download-root>/terraform-manager/<binary>` (a previously downloaded terraform-manager image);
3. **`<download-root>/<provider>/terraform-manager/<binary>`** — the plugin from the unpacked bundle;
4. otherwise it lazily pulls the terraform-manager image.

Step 3 lets converge run without registry credentials on the MetaConfig — the plugin is already
on disk from the bundle.

## How the validator is called

Provider selection is in `meta_config_validator_provider.go` (`selectValidator`):

- `""` → no validation;
- `yandex`, `vcd` → their in-tree validators (`NewMetaConfigValidator`);
- otherwise → if `<download-root>/<provider>/validator` exists, the **external binary validator**
  (`external.NewBinaryValidator`); if the provider's schema is in candi, a default prefix check;
  otherwise an error.

The external validator runs the bundle's `validator` binary as a subprocess. Contract in full:
**[`go_lib/dhctl-provider-protocol/PROTOCOL.md`](../../../go_lib/dhctl-provider-protocol/PROTOCOL.md)**
(types in `go_lib/dhctl-provider-protocol/types.go`). Summary:

- **Invocation:** `<download-root>/<provider>/validator validate`.
- **Transport:** JSON `ValidateRequest` on **stdin**, JSON `ValidateResponse` on **stdout**;
  stderr is ignored; exit code is always `0` (non-zero = the binary crashed).
- **Input** (`ValidateInput`): `providerName`, `operation` (`bootstrap`/`converge`/`destroy`),
  `clusterPrefix`, `layout`, `providerClusterConfiguration`, and `vars` (`CloudProviderVars`:
  module `settings`, `nodeGroups`, `instanceClasses`, credential `secrets`) — the only channel
  for provider resources.
- **Output** (`ValidateResponse`): `{}` on success, `{"error": "..."}` on a validation failure.
  Validation **never mutates** the config.

`vcd`'s `legacyMode` rewrite is the one provider-side config mutation; it is **not** part of
validation — it is an explicit `vcd.EnsureLegacyMode` call in the infrastructure layer
(`cloud_provider.go`) before the provider is built.

## Worked example: DVP

DVP (Deckhouse Virtualization Platform) is the reference external provider. A converge with
`--download-dir /tmp/dhctl` leaves this on disk after the bundle is delivered:

```
/tmp/dhctl/
  dvp -> /tmp/dhctl/dvp@sha256:09ae6685aed973ab2baa05f03d29ce77068852eb3c8db31c7f9dff7ab2014ad5
  dvp@sha256:09ae6685aed973ab2baa05f03d29ce77068852eb3c8db31c7f9dff7ab2014ad5/
    terraform-manager/
      terraform_versions.yml
      plan_rules.yml
      terraform-provider-kubernetes      # 54M — the opentofu plugin
    validator                            # 8.8M — the external validator binary
    openapi/{cluster_configuration.yaml,cloud_discovery_data.yaml}
    crd/  layouts/standard/  terraform-modules/  candi/module-openapi
  deckhouse/candi/                       # extracted candi image (11 providers, no dvp entry)
  cache/                                 # image blobs
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

dhctl runs `/tmp/dhctl/dvp/validator validate` and writes to its stdin:

```json
{
  "input": {
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
}
```

Success — stdout `{}`, exit `0`. Validation failure — stdout `{"error": "namespace team-d8-candi
not found"}`, exit `0`. A non-zero exit means the binary itself crashed.

### End-to-end flow (converge)

1. `parseConfigFromCluster` / `ParseMetaConfig` reads the registry from the target cluster and
   calls `ensureProviderBundle("dvp", <digest>, ...)` → unpacks into `dvp@sha256:<digest>` and
   points the `dvp` symlink at it. Nothing is written into `deckhouse/candi`.
2. The provider is built (`fsprovider.GetDi`): `mergeBundleSettings` reads
   `/tmp/dhctl/dvp/terraform-manager/terraform_versions.yml` (+ `plan_rules.yml`) → `store["dvp"]`.
3. `selectValidator("dvp")` finds `/tmp/dhctl/dvp/validator` → runs the validate protocol above.
4. `DownloadPlugin` links `/tmp/dhctl/dvp/terraform-manager/terraform-provider-kubernetes` as the
   opentofu plugin — no registry pull needed.

> The download dir in steps 1 and 2 must be the same. Commander operations thread the server's
> `GlobalOptions` (`--download-dir`) into `ParseMetaConfig`; a mismatch would unpack the bundle in
> one dir and look for its settings in another (`CloudProviderSettings not found`).
