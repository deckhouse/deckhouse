## user-authn CRDs

This directory contains **two groups** of CRDs:

- **Deckhouse CRDs** (`deckhouse.io`, `dexauthenticators.deckhouse.io`, etc.)
  - These are maintained in this repository.
- **Dex Kubernetes storage CRDs** (`*.dex.coreos.com`)
  - These files are **copied from upstream Dex** (repo `dexidp/dex`, directory `scripts/manifests/crds`).
  - **Do not edit them manually.**
  - Stored under `crds/external/` to avoid rendering as Deckhouse module CRDs.
  - Exception: `useridentities.yaml` and `authsessions.yaml`. Dex v2.46 registers these CRDs itself at start
    but does not ship them in `scripts/manifests/crds`, so they are written here by hand in the same format
    (`runtime_crds` in `pull_dex_crds.sh`). The check compares them as soon as the Dex tree ships them.

### Why Dex CRDs are here

Some hooks (e.g. UserOperation / 2FA reset logic) subscribe to Dex storage objects such as:

- `offlinesessionses.dex.coreos.com`
- `refreshtokens.dex.coreos.com`
- `useridentities.dex.coreos.com` (the controller copies OfflineSessions and Password data into it)

On fresh clusters these CRDs may be absent at the moment hooks start. Shipping Dex CRDs with the module
prevents bootstrap issues like “CRD ... not found”.

### Update Dex CRDs from upstream

Dex CRDs must stay in sync with the Dex version we build in `modules/150-user-authn/images/dex/werf.inc.yaml`.

To update:

```bash
cd modules/150-user-authn/crds
./pull_dex_crds.sh
```

### CI / build-time check (“autoverification”)

During Dex image build we run a check that our `*.dex.coreos.com` CRDs are **identical** to upstream Dex CRDs
for the currently used Dex tag.

If you see a failure like “Dex CRD mismatch …”, run `./pull_dex_crds.sh` and commit the updated CRDs.

