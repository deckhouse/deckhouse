---
title: Istio module maintenance
searchable: false
---

Upstream Istio ships as a Helm chart, which we converted to our own format, dropping unnecessary parts and replacing their crutches with ours.

Each Istio release contains:

* The `istioctl` binary with embedded Helm charts (not used by Deckhouse for deployment, but handy for utilities).
* An image with the Sail operator and the `Istio` CR (`sailoperator.io/v1`) — **only for versions below 1.27.9** (`supportsOperator: true`), i.e. currently only for Istio 1.25. The legacy `IstioOperator` CR (`install.istio.io/v1alpha1`) is no longer created: it remains only to clean up objects inherited from the discontinued version 1.21.
* A set of images with Istio components (istiod, proxyv2, cni, …).
* Upstream Helm charts.

## Adding a new Istio version

### Common steps (any version)

1. Images in `images/` — following the example of the previous minor.
2. The version in `oss.yaml`.
3. The shared set of CRDs in `crds/vendor/`. Configuration CRDs are taken from Istio 1.29.6, while operator/Sail CRDs are kept for compatibility with Istio 1.25. To update them, run `go run ./update-crds` from `modules/110-istio/tools`; the parameters are described in [`crds/vendor/README.md`](../../crds/vendor/README.md).
4. Validating the CRD bundle — a mandatory step after any changes in `crds/vendor/`, including manual edits of the files:

   ```shell
   cd modules/110-istio/tools
   go run ./update-crds --check
   ```

   The check does not access the network and validates the already committed bundle: one object per file, the complete set of CRD names, exactly one storage version per CRD, and the preservation of legacy versions in `served`.
5. **Without the operator:** `_rules_v-<major>-<minor>.tpl` + a branch in `_istiod_clusterroles.tpl`.
6. Grafana — [`istio-grafana-dashboards.sh`](istio-grafana-dashboards.sh).
7. **Without the operator:** the `files/<revision>/` directory — see below.
8. [`template_tests/module_test.go`](../../template_tests/module_test.go).

---

## Version without the operator: the `files/<revision>/` directory

Read this only if `supportsOperator: false`. Otherwise injection happens via the IOP/Istio CR ([`istios.yaml`](../../templates/control-plane/iop/istios.yaml)).

`<revision>` = `versionMap.<ver>.revision` (1.29.x → `v1x29`).

### What it is

Deckhouse creates the **`istio-sidecar-injector-<revision>`** ConfigMap ([`configmap-inject.yaml`](../../templates/control-plane/configmap-inject.yaml)) from the directory:

```none
files/<revision>/
├── static/   sidecar-injection-template.yaml, gateway-injection-template.yaml
└── templates/   sidecar-injection-values.yaml, sidecar-injection-config.yaml
        │                    │
        └─► data.values (JSON)   data.config (YAML)
```

The mesh (`istio-<revision>`) lives in [`configmap-mesh.yaml`](../../templates/control-plane/configmap-mesh.yaml), **not** in `files/`.

The sample to copy from is **`files/v1x29/`**.

### How to add a new revision (1.30 as an example)

**A. The module as a whole** — images, oss, CRDs, hooks, `_rules_v-1-30.tpl`, tests (as in the common steps above).

**B. The `files/<revision>` directory** — usually 3 commands and that's it

```bash
# 1. Clone the required Istio tag
git clone --depth 1 --branch 1.30.0 <ISTIO_REPO.git> /tmp/istio
UP=/tmp/istio/manifests/charts/istio-control/istio-discovery

# 2. Copy the whole previous revision
cp -r files/v1x29 files/v1x30

# 3. Replace only the upstream template bodies (no edits)
cp "$UP/files/injection-template.yaml"         files/v1x30/static/sidecar-injection-template.yaml
cp "$UP/files/gateway-injection-template.yaml" files/v1x30/static/gateway-injection-template.yaml
```

**For most minor bumps, this is enough.**
`templates/sidecar-injection-values.yaml` and `sidecar-injection-config.yaml` are already present in the copy — **leave them alone** unless the static templates require new settings.

**C. Control plane** (not `files/`) — istiod, webhooks, mesh: the templates in `templates/control-plane/`, the env in [`deployment.yaml`](../../templates/control-plane/deployment.yaml).

### When to edit `templates/` (rarely)

| File | When to change it |
|------|----------------|
| `sidecar-injection-values.yaml` | A new static template references a `.Values.…` that is missing from our JSON. Or the D8 logic is being changed deliberately (images, CNI, sidecar ranges). |
| `sidecar-injection-config.yaml` | Upstream changed `defaultTemplates`, the inject selectors, or new template names are needed. The **`d8-*`** blocks are Deckhouse-only and are usually copied as is. |

**How to check whether values need updating:** after `cp`-ing the static files, open the new `static/*.yaml` and search for `.Values.` — if a field is used without a fallback default in the template, add it to `sidecar-injection-values.yaml` (with the D8 helpers, as in `v1x29`).

**How to check the config:** open the upstream
`$UP/templates/istiod-injector-configmap.yaml`, the `config:` section (up to `templates:`) — and compare it with the top of our `sidecar-injection-config.yaml`. If Istio hasn't changed the policy/selectors, leave it alone.

### Important: don't compare our JSON with the upstream JSON

In `istiod-injector-configmap.yaml`, upstream assembles a **broad** `data.values` (almost the entire `global` from the chart values + `gateways` + `sidecarInjectorWebhook`).

Ours is a **narrow** JSON: only what the inject templates need plus Deckhouse settings (registry, the `d8-istio` namespace, CNI, …). The webhook config lives in **`config`**, and the mesh lives in a **separate CM**.

That's why the diff between the "upstream render" and "our render" for the **same tag** is always large — **this is normal**, not a sign of an error and not a checklist of things to fix.

When doing a bump, look at:
1. **The diff of the two static files** (the old vs. the new upstream one) — what changed in inject.
2. **New `.Values.*` in the static files** — whether `values` needs to be extended.
3. **The tests** — `go test` in `template_tests/`.

### Where things come from upstream

Istio tag → `manifests/charts/istio-control/istio-discovery/`:

| What we need | Upstream file |
|-----------|---------------|
| static sidecar | `files/injection-template.yaml` |
| static gateway | `files/gateway-injection-template.yaml` |
| for reference: what Istio puts into the CM | `templates/istiod-injector-configmap.yaml` |

[`istios.yaml`](../../templates/control-plane/iop/istios.yaml) — how it used to be with the operator; for the operator-free case, **do not copy it as a whole**, just take a look at the D8 logic.

### `files/` checklist

- [ ] `cp -r files/<prev> files/<revision>`
- [ ] `static/` — 2 files from upstream as-is
- [ ] if necessary — edits in `templates/` (see the table above)
- [ ] `template_tests/module_test.go`

---

## Grafana dashboards

The [`istio-grafana-dashboards.sh`](istio-grafana-dashboards.sh) script → JSON in `monitoring/grafana-dashboards/istio/`:

* Clones the required Istio version.
* `irate` → `rate`, `Resolution` → `1/1`, removes Min Step.
* Graphs → Staircase (Stack+Percent may need manual fixing).
* datasource → null, fixes the links to dashboards.
