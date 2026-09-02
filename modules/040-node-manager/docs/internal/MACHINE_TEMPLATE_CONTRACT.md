---
title: "CAPI machine-template contract v2"
description: How a cloud-provider module describes the CAPI machine template that node-manager renders for ephemeral nodes.
---

This is the single source of truth for `capi/template.yaml`, the file a cloud-provider module
ships. **If something is not in this document, it is not in the contract**: node-controller gives
a template nothing beyond what is described here, and widening the contract requires bumping
`version`.

## What node-manager does with your file

For every zone of a NodeGroup, node-controller first decides whether the machines must be
recreated. Only then does it render your template into a new infrastructure MachineTemplate and
point the MachineDeployment at it. CAPI recreates machines because the *name* of the referenced
template changed — that is the CAPI contract, and the reason the objects are immutable.

Design for three consequences:

- **Your template is rendered once per generation.** The rendered object is frozen for that
  generation's lifetime; node-controller never re-renders or patches it.
- **New template text does not reach existing machines by itself, and neither does a provider
  config change you did not declare.** The next generation picks it up. Four things create one: a
  user changes a `rolloutFields` field, a cluster-wide config field listed in
  `providerRolloutFields` changes, an operator sets the `manual-rollout-id` annotation on the
  NodeGroup, or a provider release adds to either list a field that had already been edited
  (see below).
- **The rendered object must depend on nothing but the context.** The template cannot reach the
  clock, random numbers, the network or the environment. If it could, "did anything change?"
  would have no answer — every render would produce something new.

## File shape

```yaml
version: v2

# Required. InstanceClass spec fields whose change must recreate the machines,
# as dot-paths into the spec.
rolloutFields:
  - flavorName
  - rootDisk.size

# Optional, and almost always absent. Fields of your subtree of the cloud-provider
# config whose change must recreate the machines, as dot-paths.
providerRolloutFields:
  - metadata

# Optional. Extra fields node-controller writes into the MachineDeployment it builds
# (the MachineDeployment is generic — your template does not render it).
# Key: dot-path inside spec.template.spec. Value: a go-template, same sandbox and
# same context as the machine template below.
machineDeployment:
  additionalFields:
    failureDomain: "{{ .zone }}"

# Required. The go-template of your infrastructure MachineTemplate.
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: OpenStackMachineTemplate
  spec:
    template:
      spec:
        flavor: {{ .instanceClass.flavorName | quote }}
```

Parsing is strict: an unknown key, an empty template, a missing `rolloutFields`, a malformed
field path or an unparsable template invalidates the whole contract, and the reconcile fails
loudly.

## Delivery

The file travels in the provider's CAPI secret, the same channel as before:

- in-tree modules: `d8-cloud-provider-<type>-capi` in `kube-system`;
- external modules: any secret labelled
  `cloud-provider.deckhouse.io/ephemeral-nodes-templates: capi` (see the ADR *External cloud
  provider module*).

Put the file in your module's `capi/` directory; the helm template that builds the secret picks
it up by basename.

### Where the provider config comes from

node-controller reads one thing for `.provider`: your subtree of the
`d8-node-manager-cloud-provider` secret. It does not read your `ProviderClusterConfiguration`, and
it does not read your ModuleConfig — those are inputs to your `registration.yaml`, which runs as a
helm template in the Deckhouse addon-operator, well before node-controller sees anything.

Two things follow:

- **Where your module keeps its source of truth is invisible here.** `ProviderClusterConfiguration`,
  ModuleConfig, or both during a migration with ModuleConfig winning — `registration.yaml` resolves
  that and publishes the result. dvp already moved its `sshPublicKey` to ModuleConfig and
  node-controller never noticed.
- **The keys you publish are the contract surface.** Renaming one breaks the template
  (`.provider.<oldKey>` renders empty) and, if it is a `providerRolloutFields` entry, the rollout
  comparison too — the declared field reads as absent, which recreates every machine of the
  provider. Treat reshaping the subtree as a migration, not a refactor.

## Render context — five roots, nothing else

| Root | Type | What it is |
|---|---|---|
| `.instanceClass` | map | Your `<Provider>InstanceClass` **spec**, verbatim. Numbers are JSON numbers (float64) — use `\| int` when you need integer output. |
| `.provider` | map | Your own subtree of the `d8-node-manager-cloud-provider` secret. You no longer spell your provider name inside your own file. |
| `.zone` | string | The zone this generation is rendered for. |
| `.nodeGroup.name` | string | NodeGroup name — for tags and labels inside `spec`. |
| `.cluster` | map | `{uuid, podSubnet}`. |

There is no `.Values`, no `.Chart`, no `.Files`, no `.Release`. node-manager is not helm; the v1
engine emulated a values tree only so that migrated templates would not need rewriting.

### `.provider`, concretely

It is the value of your own key in the registration secret, decoded from JSON. node-controller
picks the key by the value of `type` in the same secret.

```console
$ kubectl -n kube-system get secret d8-node-manager-cloud-provider -o yaml
data:
  type: ZHZw        # "dvp"
  dvp:  e30=        # "{}"
```

For dvp that makes `.provider` an empty map — the module publishes no configuration of its own, and
its template reads nothing from it. Reaching into it (`.provider.anything`) is a render error, as
for any absent key.

The same secret on a vcd cluster, with the `vcd` value decoded:

```json
{"sshPublicKey":"ssh-ed25519 AAAA…","organization":"org","virtualDataCenter":"vdc",
 "server":"https://vcd.example.com","username":"admin","password":"…","apiToken":"…",
 "metadata":{"owner":"platform"}}
```

so `.provider.metadata` is `{"owner":"platform"}` — the value the vcd template merges with the
InstanceClass `additionalMetadata`. Note what else is in there: the subtree carries credentials, so
never render `.provider` wholesale (`toYaml`, `range` over the whole map) — name the fields you
need.

## Rules

1. **Render `apiVersion`, `kind` and `spec` — nothing else.** `metadata` is rejected:
   node-controller owns the name (it encodes the generation), the labels (`heritage`, `module`,
   `node-group` — every prune and cleanup selects on them) and the annotations (they hold the
   rollout snapshot).
2. **The rendered `apiVersion`/`kind` must match `capiMachineTemplateAPIVersion`/
   `capiMachineTemplateKind` from your registration secret**, otherwise the reconcile fails.
3. **Read optional fields with `get`, or guard them with `hasKey`.** The sandbox runs with
   `missingkey=error`, so `.instanceClass.maybeAbsent` is a render error, not an empty string.
   This is deliberate: under v1 a typo rendered `<no value>` straight into the cloud.
4. **Keep the template predictable.** Rendering the same context twice must give the same
   object.
5. **Say so when input is missing** — `fail "…"` or `required "…" value`. Do not substitute a
   default: that quietly builds the wrong machine. node-controller adds the NodeGroup, the
   InstanceClass and the zone to the message, so do not repeat them.

## Functions

The sandbox is [sprig](https://masterminds.github.io/sprig/) plus helm's `toYaml`, `fromYaml`,
`toJson`, `fromJson`, `required`. Removed, and unavailable by design:

| Group | Examples |
|---|---|
| clock | `now`, `date`, `dateInZone`, `ago`, `toDate`, `duration` |
| randomness | `randAlphaNum`, `randBytes`, `shuffle`, `uuidv4` |
| crypto / secrets | `genPrivateKey`, `genCA`, `genSelfSignedCert`, `encryptAES`, `bcrypt`, `htpasswd` |
| host environment | `env`, `expandenv`, `getHostByName` |
| helm-only | `include`, `tpl`, `lookup` |

Using one is a parse error, so it can never reach a cluster. The exact function set is pinned by
a golden test (`internal/machinetemplate/testdata/sandbox_functions.txt`), so a sprig upgrade
that adds a function is reviewed rather than silently accepted.

Traps worth knowing:

- `default` fires on **any** falsy value: `0`, `false` and `""` all take the default. That is
  the helm behaviour your v1 template already had; keep it in mind when a `0` is meaningful.
- `get dict "key"` returns an empty value for an absent key — that is what makes
  `get .instanceClass "diskType" | default "network-hdd"` work under `missingkey=error`.
- Ranging over a possibly-absent list: `{{- range (get .instanceClass "subnets" | default list) }}`.

## rolloutFields — what recreates machines

`rolloutFields` is your answer to one question: *which InstanceClass fields cannot be changed on
a live VM?* Only your team knows. Fields not in the list are still rendered into the object;
they simply do not, on their own, cause a new generation — the change reaches machines with the
next rollout.

How node-controller uses the list:

- The **whole** InstanceClass spec is snapshotted on the generation object
  (`node.deckhouse.io/applied-instance-class`, next to `applied-rollout-id` and
  `applied-generation`); `rolloutFields` filters the comparison, not the snapshot.
- **Removing** a field from the list therefore never rolls machines.
- **Adding** a field is free only while it has not changed since the current generation was
  created: the snapshot holds the value from back then, and the widened criterion compares
  against it. If a user edited the field in the meantime, the next reconcile rolls that
  NodeGroup — check before widening the list, or pair the change with a release note.
- Comparison is **by value** after both sides are normalized: `50` is `50` no matter which Go
  type or serialization it arrived in. There are no byte-level goldens to be afraid of any more.
- An absent field and an explicit `null` compare equal.
- `manual-rollout-id` on the NodeGroup is handled by node-controller for every provider; it is
  not part of your file.

Choosing the list:

- Include everything the cloud cannot change in place: flavor/CPU/memory, image, disks,
  networks, security groups, tags baked in at creation.
- Exclude fields that describe something other than the VM. Two real examples from the
  migration: openstack's `capacity` (a scale-from-zero hint for the autoscaler) and DVP's
  `etcdDisk` (a master parameter) are not listed — an unrelated edit would otherwise roll a
  whole fleet.
- When a cloud gains a live-change capability (hotplug memory, mutable tags), dropping the field
  from the list is a one-line release that rolls nobody.

## providerRolloutFields — the same question for the cluster-wide config

Your template renders from two inputs, so the rollout decision reads two. `providerRolloutFields`
names the fields of *your* subtree of the cloud-provider config whose change must recreate
machines. Values are compared exactly as for `rolloutFields`, but the snapshot is not the same:
**only the declared fields are recorded** on the object
(`node.deckhouse.io/applied-provider-config`). The provider subtree of
`d8-node-manager-cloud-provider` carries the cloud credentials — vcd's `password` and `apiToken`,
yandex's `serviceAccountJSON`, huaweicloud's `accessKey`, openstack's `connection.password` — and a
MachineTemplate is read far more widely than that Secret.

That reduction has one consequence `rolloutFields` does not: **adding an entry can roll machines**,
because a field the old snapshot never recorded compares as absent against its current value. Check
the value in the cluster before widening the list, or pair the change with a release note.

Leave it out unless you mean it. A cloud-provider config field applies to every NodeGroup in the
cluster, so listing one turns a single edit into a fleet-wide rollout. Six of the seven migrated
providers declare nothing here: their config feeds the machine, but changing it was never a reason
to recreate one.

List a field when a user-visible promise depends on it. The one in-tree example is vcd's
`metadata`, which `VCDClusterConfiguration` documents as recreating CloudEphemeral nodes: the v1
checksum hashed it, and leaving it out would have turned that promise into silence while the
template kept rendering the new value into every machine created later.

## machineDeployment.additionalFields

node-controller builds the MachineDeployment; your template does not. To add fields to it,
declare them here: the key is a dot-path inside `spec.template.spec`, the value is a go-template
rendered in the same sandbox with the same five context roots as the machine template. Today one
provider uses one entry:

```yaml
machineDeployment:
  additionalFields:
    failureDomain: "{{ .zone }}"
```

The value is compiled at load time, so a broken expression fails the same way a broken machine
template does. The result is always written as a string.

## Migrating from v1

| v1 | v2 |
|---|---|
| `.Values.nodeManager.internal.cloudProvider.<type>.…` | `.provider.…` |
| `.nodeGroup.instanceClass.…` | `.instanceClass.…` |
| `.zoneName` | `.zone` |
| `.Values.global.discovery.clusterUUID` / `.podSubnet` | `.cluster.uuid` / `.cluster.podSubnet` |
| `.nodeGroup.name` | `.nodeGroup.name` (unchanged) |
| `metadata:` block + `include "helm_lib_module_labels"` | — (node-controller stamps metadata) |
| `instance-class.checksum` | `rolloutFields` |
| `machine-deployment-spec-patch.yaml` | `machineDeployment.additionalFields` |
| direct path to an optional field | `get` / `hasKey` (missingkey=error) |

Switching a live cluster does **not** roll machines: on the first v2 reconcile node-controller
finds the template the MachineDeployment already references (a v1 checksum-named object), writes
the snapshot into its annotations and adopts it as the current generation. Generation numbering
(`<ng>-<zone-hash>-gen1`) starts at the next real change.

## Checking your template

Every `capi/template.yaml` in the repository is picked up by a globbing test, so a new provider
gets contract validation the moment the file ships. Beyond that, the parity harness in
`modules/040-node-manager/images/node-controller/src/internal/machinetemplate/provider_parity_test.go`
takes one fixture per provider and requires:

1. the v2 template to render exactly what the archived v1 template rendered, and
2. the `rolloutFields` decision to match the v1 checksum decision for every mutated field, with
   every deliberate difference listed and justified in the test.

Add your provider there before merging a migration. Self-check list:

- [ ] `version: v2`, non-empty `template`, non-empty `rolloutFields`.
- [ ] No `metadata` in the rendered object.
- [ ] `apiVersion`/`kind` match the registration secret.
- [ ] Every optional field read through `get`/`hasKey`.
- [ ] No clock/random/env/crypto function (it would not parse anyway).
- [ ] `rolloutFields` covers everything the cloud cannot change on a live VM, and nothing that
      is not a property of the VM.
- [ ] Parity fixture added, both parity tests green.
