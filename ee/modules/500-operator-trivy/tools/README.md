# Tools for 500-operator-trivy

## prepare-sources-and-apply-patches.sh

Script: [`prepare-sources-and-apply-patches.sh`](prepare-sources-and-apply-patches.sh:1)

Purpose: clone upstream sources and apply Deckhouse patches **the same way CI/werf does** for `ee/modules/500-operator-trivy`, including the *nested source layout* where the operator build workspace is assembled as a single `/src` tree:

- base `/src` from `trivy-src-artifact` (contains `trivy/`, `trivy-db/`, and patch repos)
- then `trivy-operator` repo is cloned into `/src/trivy-operator` and overlaid into `/src` (like `mv /src/trivy-operator/* /src` in werf)

It reproduces logic from:
- [`ee/modules/500-operator-trivy/images/trivy/werf.inc.yaml`](../images/trivy/werf.inc.yaml:1)
- [`ee/modules/500-operator-trivy/images/operator/werf.inc.yaml`](../images/operator/werf.inc.yaml:1)
- [`ee/modules/500-operator-trivy/images/node-collector/werf.inc.yaml`](../images/node-collector/werf.inc.yaml:1)

### Defaults

- `SOURCE_REPO=https://github.com`
- `DECKHOUSE_PRIVATE_REPO=fox.flant.com` (used via SSH: `git@fox.flant.com:...`)
- `WORKDIR=./_work` (relative to current directory where you run the script)
- `COMPONENTS=all`
- `STRIP_GIT=1` (mimic CI cleanup like `rm -rf .../.git` in src artifacts)
- `CREATE_SHORTCUTS=1` (create convenience symlinks inside `WORKDIR`: `trivy-src` and `operator-src`)

Versions (as in werf):
- `TRIVY_VERSION=v0.55.2`
- `TRIVY_OPERATOR_VERSION=v0.22.0`
- `NODE_COLLECTOR_VERSION=v0.3.1`

### Usage examples

Run all components:

```bash
bash ee/modules/500-operator-trivy/tools/prepare-sources-and-apply-patches.sh
```

Run only selected components:

```bash
COMPONENTS=trivy,trivy-db WORKDIR=./_work \
  bash ee/modules/500-operator-trivy/tools/prepare-sources-and-apply-patches.sh
```

### Output layout (mirrors werf)

Inside `WORKDIR` the script creates directories that mimic werf image stages:

- `trivy-src-artifact/src/`
  - `trivy/`
  - `trivy-db/`
  - `trivy-patch/` (private)
  - `trivy-db-patch/` (private)

- `operator-src-artifact/src/`
  - starts as a copy of `trivy-src-artifact/src/`
  - then `trivy-operator` repo is cloned into `src/trivy-operator/` and its `*` files are moved into `src/` (same as werf `mv /src/trivy-operator/* /src`)
  - local patches from [`ee/modules/500-operator-trivy/images/operator/patches`](../images/operator/patches:1) are applied to `operator-src-artifact/src/`
  - `images/operator/bundle.tar.gz` is unpacked into `operator-src-artifact/src/local/`

### Notes

- The script is **idempotent** for each git repo directory: if it exists, it does `git fetch` + `git reset --hard` + `git clean -fdx` and re-applies patches.
- For `trivy` and `trivy-db` patchsets, patches are taken from private Deckhouse repos on `DECKHOUSE_PRIVATE_REPO`.
- For `trivy-operator` and `k8s-node-collector`, patches are taken from the local Deckhouse tree:
  - [`ee/modules/500-operator-trivy/images/operator/patches`](../images/operator/patches:1)
  - [`ee/modules/500-operator-trivy/images/node-collector/patches`](../images/node-collector/patches:1)
