---
title: "CAPI machine-template contract v2"
description: How a cloud-provider module describes the CAPI machine template node-manager renders for ephemeral nodes.
---

This is the single source of truth for the file a cloud-provider module ships as
`capi/template.yaml`. **If something is not in this document, it is not in the contract**:
node-controller gives a template nothing beyond what is described here, and widening the contract
requires bumping `version`.

## What node-manager does with your file

For every NodeGroup zone, node-controller decides whether the machines need to be recreated, and
only when they do, it renders your template into a new infrastructure MachineTemplate object and
points the MachineDeployment at it. CAPI recreates machines because the *name* of the referenced
template changed — that is the CAPI contract, and it is why the objects are immutable.

Consequences you must design for:

- **Your template is rendered once per generation.** The rendered object is frozen for the life of
  that generation; node-controller never re-renders or patches it.
- **A new template text, or a new provider config, does not reach existing machines by itself.**
  It is picked up by the next generation — either when a user changes a rolloutField, or when an
  operator forces a rollout with the `manual-rollout-id` annotation on the NodeGroup.
- **The render must be a pure function of the context.** Anything else (clock, randomness, network,
  environment) is unavailable, and would make "did anything change?" unanswerable.

## File shape

```yaml
version: v2

# Required. InstanceClass spec fields whose change must recreate the machines,
# as dot-paths into the spec.
rolloutFields:
  - flavorName
  - rootDisk.size

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

The file is delivered in the provider's CAPI secret, exactly as before: for in-tree modules
`d8-cloud-provider-<type>-capi` in `kube-system`, for external modules any secret labelled
`cloud-provider.deckhouse.io/ephemeral-nodes-templates: capi` (see the ADR *External cloud provider
module*). Put the file in your module's `capi/` directory; the helm template that builds the secret
picks it up by basename.

Parsing is strict: an unknown key, an empty template, a missing `rolloutFields`, a malformed field
path or an unparsable template makes the whole contract invalid and the reconcile fails loudly.

## Render context — five roots, nothing else

| Root | Type | What it is |
|---|---|---|
| `.instanceClass` | map | Your `<Provider>InstanceClass` **spec**, verbatim. Numbers are JSON numbers (float64) — use `\| int` when you need integer output. |
| `.provider` | map | Your own subtree of the `d8-node-manager-cloud-provider` secret. You no longer spell your provider name inside your own file. |
| `.zone` | string | The zone this generation is rendered for. |
| `.nodeGroup.name` | string | NodeGroup name — for tags and labels inside `spec`. |
| `.cluster` | map | `{uuid, podSubnet}`. |

There is no `.Values`, no `.Chart`, no `.Files`, no `.Release`: node-manager is not helm, and the
v1 engine only emulated a values tree so that migrating templates would not have to be rewritten.

## Rules

1. **Render `apiVersion`, `kind` and `spec` only.** `metadata` is rejected: node-controller owns the
   name (it encodes the generation), the labels (`heritage`, `module`, `node-group` — every prune
   and cleanup selects on them) and the annotations (they hold the rollout snapshot).
2. **The rendered `apiVersion`/`kind` must match `capiMachineTemplateAPIVersion`/
   `capiMachineTemplateKind` from your registration secret**, otherwise the reconcile fails.
3. **Optional fields must be read with `get` or guarded with `hasKey`.** The sandbox runs with
   `missingkey=error`, so `.instanceClass.maybeAbsent` is a render error, not an empty string. This
   is deliberate: under v1 a typo rendered `<no value>` straight into the cloud.
4. **Be deterministic.** Rendering twice with the same context must produce the same object.
5. **Fail loudly when input is missing** — `fail "…"` or `required "…" value`. Do not paper over it
   with a default that silently creates the wrong machine. node-controller adds the NodeGroup, the
   InstanceClass and the zone to the message; do not repeat them.

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

Using one is a parse error, so it can never reach a cluster. The exact function set is pinned by a
golden test (`internal/machinetemplate/testdata/sandbox_functions.txt`), so a sprig upgrade that
adds a function is reviewed rather than silently accepted.

Traps worth knowing:

- `default` fires on **any** falsy value: `0`, `false` and `""` all take the default. That is the
  helm behaviour your v1 template already had; keep it in mind when a `0` is meaningful.
- `get dict "key"` returns an empty value for an absent key — that is what makes
  `get .instanceClass "diskType" | default "network-hdd"` work under `missingkey=error`.
- Ranging over a possibly-absent list: `{{- range (get .instanceClass "subnets" | default list) }}`.

## rolloutFields — what recreates machines

`rolloutFields` is your answer to one question: *which InstanceClass fields cannot be changed on a
live VM?* Only your team knows. Fields not in the list are still rendered into the object; they
simply do not, on their own, cause a new generation — the change reaches machines with the next
rollout.

How node-controller uses it:

- The **whole** InstanceClass spec is stored on the generation object
  (`node.deckhouse.io/applied-instance-class`, next to `applied-rollout-id` and
  `applied-generation`), and `rolloutFields` is applied only when comparing.
  So **removing** a field from the list never rolls machines. **Adding** one is free only while
  that field has not changed since the current generation was created: the snapshot holds the value
  from back then, and the widened criterion now compares against it. Add a field a user edited in
  the meantime and the next reconcile rolls that NodeGroup — check before widening the list, or
  pair it with a release note.
- Comparison is **by value** after both sides are normalized: `50` is `50` no matter which Go type
  or serialization it arrived in. There are no byte-level goldens to be afraid of any more.
- An absent field and an explicit `null` compare equal.
- `manual-rollout-id` on the NodeGroup is handled by node-controller for every provider; it is not
  part of your file.

Choosing the list:

- Include everything the cloud cannot change in place: flavor/CPU/memory, image, disks, networks,
  security groups, tags baked at creation.
- Exclude fields that describe something other than the VM. Real examples from the migration:
  openstack's `capacity` (a scale-from-zero hint for the autoscaler) and DVP's `etcdDisk` (a master
  parameter) are not listed — listing them would roll a whole fleet on an unrelated edit.
- When a cloud gains a live-change capability (hotplug memory, mutable tags), dropping the field
  from the list is a one-line release, and it will not roll anybody.

## machineDeployment.additionalFields

The MachineDeployment is built by node-controller, not by your template. If you need extra fields
in it, declare them here: the key is a dot-path inside `spec.template.spec`, the value is a
go-template rendered in the same sandbox, with the same five context roots, as the machine
template. Today one provider uses one entry:

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

Every `capi/template.yaml` in the repository is parsed by a test that globs for it, so a new
provider gets contract validation the moment it ships the file. Beyond that, the parity harness in
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
- [ ] `rolloutFields` covers everything the cloud cannot change on a live VM, and nothing that is
      not a property of the VM.
- [ ] Parity fixture added, both parity tests green.
