## user-authn CRDs

This directory contains **two groups** of CRDs:

- **Deckhouse CRDs** (`deckhouse.io`, `dexauthenticators.deckhouse.io`, etc.)
  - These are maintained in this repository.
- **Dex Kubernetes storage CRDs** (`*.dex.coreos.com`)
  - These files are **copied from the Dex we build**: the Dex release from `oss.yaml` (repo `dexidp/dex`,
    directory `scripts/manifests/crds`) with the patches of `images/dex/patches` applied.
    `100-storage-schema.patch` adds `useridentities.yaml` and `authsessions.yaml`, which Dex registers itself at
    start but does not ship upstream.
  - **Do not edit them manually.**
  - Stored under `crds/external/` to avoid rendering as Deckhouse module CRDs.

### Why Dex CRDs are here

Some hooks (e.g. UserOperation / 2FA reset logic) subscribe to Dex storage objects such as:

- `offlinesessionses.dex.coreos.com`
- `refreshtokens.dex.coreos.com`
- `useridentities.dex.coreos.com` (the controller copies OfflineSessions and Password data into it)

On fresh clusters these CRDs may be absent at the moment hooks start. Shipping Dex CRDs with the module
prevents bootstrap issues like “CRD ... not found”.

### Update Dex CRDs

Dex CRDs must stay in sync with the Dex we build in `modules/150-user-authn/images/dex/werf.inc.yaml`.

To update, clone the Dex release, apply the patches and copy the CRDs (needs git and network):

```bash
cd modules/150-user-authn/crds
./pull_dex_crds.sh
```

To copy them from a Dex source tree that already has the patches applied:

```bash
./pull_dex_crds.sh --dex-dir /path/to/dex/scripts/manifests/crds
```

### CI / build-time check (“autoverification”)

During Dex image build we run a check that our `*.dex.coreos.com` CRDs are **identical** to the CRDs of the
patched Dex source tree.

If you see a failure like “Dex CRD mismatch …”, run `./pull_dex_crds.sh` and commit the updated CRDs.

