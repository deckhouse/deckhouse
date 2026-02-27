#!/usr/bin/env bash
set -Eeuo pipefail

# This script reproduces (as close as possible) the CI/werf cloning+patching logic for ee/modules/500-operator-trivy.
# See:
# - ee/modules/500-operator-trivy/images/trivy/werf.inc.yaml
# - ee/modules/500-operator-trivy/images/operator/werf.inc.yaml
# - ee/modules/500-operator-trivy/images/node-collector/werf.inc.yaml

log() { printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*" >&2; }
warn() { log "WARN: $*"; }
die() { log "ERROR: $*"; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1";
}

# Defaults requested by task.
SOURCE_REPO="${SOURCE_REPO:-https://github.com}"
DECKHOUSE_PRIVATE_REPO="${DECKHOUSE_PRIVATE_REPO:-fox.flant.com}"
WORKDIR="${WORKDIR:-./_work}"

# Versions as in werf.
TRIVY_VERSION="${TRIVY_VERSION:-v0.55.2}"
TRIVY_OPERATOR_VERSION="${TRIVY_OPERATOR_VERSION:-v0.22.0}"
NODE_COLLECTOR_VERSION="${NODE_COLLECTOR_VERSION:-v0.3.1}"

# CI-like layout by default: remove .git dirs from upstream sources where werf does.
# Set STRIP_GIT=0 if you want to keep .git directories for easier patch authoring.
STRIP_GIT="${STRIP_GIT:-1}"

# Convenience symlinks to match CI mental model (/src). Safe to disable.
CREATE_SHORTCUTS="${CREATE_SHORTCUTS:-1}"

# If COMPONENTS is empty, treat it as default.
COMPONENTS_RAW="${COMPONENTS:-all}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OPERATOR_PATCHES_DIR="${SCRIPT_DIR}/../images/operator/patches"
NODE_COLLECTOR_PATCHES_DIR="${SCRIPT_DIR}/../images/node-collector/patches"
OPERATOR_BUNDLE_TGZ="${SCRIPT_DIR}/../images/operator/bundle.tar.gz"

# Mirrors werf layout (nested source trees):
# - trivy-src-artifact creates /src with trivy + trivy-db + patch repos.
# - operator-src-artifact is based on trivy-src-artifact and then overlays trivy-operator sources into the same /src.
TRIVY_SRC_ARTIFACT_DIR="${WORKDIR}/trivy-src-artifact"
TRIVY_SRC_ROOT="${TRIVY_SRC_ARTIFACT_DIR}/src"

OPERATOR_SRC_ARTIFACT_DIR="${WORKDIR}/operator-src-artifact"
OPERATOR_SRC_ROOT="${OPERATOR_SRC_ARTIFACT_DIR}/src"

TRIVY_PATCH_REPO="git@${DECKHOUSE_PRIVATE_REPO}:deckhouse/trivy.git"
TRIVY_DB_REPO="git@${DECKHOUSE_PRIVATE_REPO}:deckhouse/3p/aquasecurity/trivy-db.git"
TRIVY_DB_PATCH_REPO="git@${DECKHOUSE_PRIVATE_REPO}:deckhouse/trivy-db.git"

TRIVY_OPERATOR_REPO="${SOURCE_REPO}/aquasecurity/trivy-operator.git"
TRIVY_REPO="${SOURCE_REPO}/aquasecurity/trivy.git"
NODE_COLLECTOR_REPO="${SOURCE_REPO}/aquasecurity/k8s-node-collector.git"

# Werf trigger for rebuild (see ee/modules/500-operator-trivy/images/trivy/werf.inc.yaml).
TRIVY_PATCH_TRIGGER_COMMIT="${TRIVY_PATCH_TRIGGER_COMMIT:-2ed377e076313b526bdd082d444b6118d610afb7}"

normalize_components() {
  # output: one per line
  local raw="$1"
  raw="${raw// /}"
  if [[ -z "$raw" || "$raw" == "all" ]]; then
    printf '%s\n' trivy trivy-db trivy-operator k8s-node-collector
    return 0
  fi

  # support aliases
  local item
  IFS=',' read -r -a items <<<"$raw"
  for item in "${items[@]}"; do
    case "$item" in
      trivy) printf '%s\n' trivy ;;
      trivy-db|trivydb) printf '%s\n' trivy-db ;;
      trivy-operator|operator) printf '%s\n' trivy-operator ;;
      k8s-node-collector|node-collector|nodecollector) printf '%s\n' k8s-node-collector ;;
      *) die "Unknown component in COMPONENTS: '$item'. Allowed: all,trivy,trivy-db,trivy-operator,k8s-node-collector" ;;
    esac
  done
}

component_selected() {
  local name="$1"
  local c
  while read -r c; do
    [[ "$c" == "$name" ]] && return 0
  done <<<"$SELECTED_COMPONENTS"
  return 1
}

# Clone or update existing repo directory.
# Usage: git_sync <dir> <url> <ref>
# <ref> can be: branch/tag name, or commit sha.
git_sync() {
  local dir="$1" url="$2" ref="$3"

  if [[ -d "$dir/.git" ]]; then
    log "Updating repo: $dir (url=$url ref=$ref)"
    ( cd "$dir" && \
      git remote set-url origin "$url" && \
      git fetch --tags --prune origin && \
      git reset --hard && \
      git clean -fdx )
  else
    log "Cloning repo: $url -> $dir (ref=$ref)"
    rm -rf "$dir"
    mkdir -p "$(dirname "$dir")"
    # If ref looks like a 40-hex commit, we cannot use --branch.
    if [[ "$ref" =~ ^[0-9a-f]{7,40}$ ]]; then
      git clone "$url" "$dir"
    else
      git clone --depth 1 --branch "$ref" "$url" "$dir"
    fi
  fi

  ( cd "$dir" && \
    if [[ "$ref" =~ ^[0-9a-f]{7,40}$ ]]; then
      git checkout --detach "$ref"
    else
      # Make sure branch/tag is checked out.
      git checkout -f "$ref"
    fi \
  )
}

apply_patches_glob() {
  local repo_dir="$1" glob="$2"
  local count

  shopt -s nullglob
  # shellcheck disable=SC2206
  local patches=( $glob )
  shopt -u nullglob

  count="${#patches[@]}"
  if [[ "$count" -eq 0 ]]; then
    warn "No patches found: $glob"
    return 0
  fi

  log "Applying $count patch(es) to $repo_dir from: $glob"
  ( cd "$repo_dir" && git apply --verbose --whitespace=fix "${patches[@]}" )
}

apply_patches_dir_sorted() {
  local repo_dir="$1" patches_dir="$2"

  [[ -d "$patches_dir" ]] || die "Patches dir not found: $patches_dir"

  local patches=()
  while IFS= read -r -d '' f; do patches+=("$f"); done < <(find "$patches_dir" -maxdepth 1 -type f -name '*.patch' -print0 | sort -z)

  if [[ "${#patches[@]}" -eq 0 ]]; then
    warn "No .patch files in: $patches_dir"
    return 0
  fi

  log "Applying ${#patches[@]} patch(es) to $repo_dir from dir: $patches_dir"
  ( cd "$repo_dir" && git apply --verbose --whitespace=fix "${patches[@]}" )
}

ssh_hint_private_repo() {
  # Non-fatal check: in CI it's expected to work; locally it might be missing.
  local host="$1"
  if ssh -o BatchMode=yes -o ConnectTimeout=5 -T "git@${host}" 2>/dev/null; then
    log "SSH access check OK for git@${host}"
  else
    warn "SSH access check failed for git@${host}. If cloning private repos fails, ensure your SSH key is added and you can run: ssh -T git@${host}"
  fi
}

prepare_trivy_db_into_src_root() {
  mkdir -p "$TRIVY_SRC_ROOT"
  log "Preparing trivy-db in: $TRIVY_SRC_ROOT"

  git_sync "$TRIVY_SRC_ROOT/trivy-db" "$TRIVY_DB_REPO" "$TRIVY_VERSION"
  git_sync "$TRIVY_SRC_ROOT/trivy-db-patch" "$TRIVY_DB_PATCH_REPO" "$TRIVY_VERSION"
  apply_patches_glob "$TRIVY_SRC_ROOT/trivy-db" "$TRIVY_SRC_ROOT/trivy-db-patch/patches/${TRIVY_VERSION}/*.patch"
}

prepare_trivy_into_src_root() {
  mkdir -p "$TRIVY_SRC_ROOT"
  log "Preparing trivy in: $TRIVY_SRC_ROOT"

  git_sync "$TRIVY_SRC_ROOT/trivy" "$TRIVY_REPO" "$TRIVY_VERSION"

  # In werf this repo is cloned non-shallow and then checked out to a pinned commit (werf trigger).
  # Using commit ref here avoids issues when the commit is not reachable from a shallow clone.
  git_sync "$TRIVY_SRC_ROOT/trivy-patch" "$TRIVY_PATCH_REPO" "$TRIVY_PATCH_TRIGGER_COMMIT"

  apply_patches_glob "$TRIVY_SRC_ROOT/trivy" "$TRIVY_SRC_ROOT/trivy-patch/patches/${TRIVY_VERSION}/*.patch"

  # Keep sources close to CI, but do not delete anything outside repositories.
  rm -rf "$TRIVY_SRC_ROOT/trivy/docs" "$TRIVY_SRC_ROOT/trivy/integration" || true
  # remove testdata dirs (CI does this for image build size reduction)
  if command -v find >/dev/null 2>&1; then
    find "$TRIVY_SRC_ROOT/trivy" -type d -name testdata -prune -exec rm -rf {} + 2>/dev/null || true
  fi
}

prepare_trivy_src_artifact() {
  # Mimics ee/modules/500-operator-trivy/images/trivy/werf.inc.yaml src-artifact stage.
  log "== Artifact: trivy-src-artifact =="
  mkdir -p "$TRIVY_SRC_ARTIFACT_DIR"

  # In CI trivy-src-artifact always contains both trivy and trivy-db.
  prepare_trivy_db_into_src_root
  prepare_trivy_into_src_root

  # Mimic: rm -rf /src/trivy/.git /src/trivy-db/.git
  if [[ "$STRIP_GIT" == "1" ]]; then
    rm -rf "$TRIVY_SRC_ROOT/trivy/.git" "$TRIVY_SRC_ROOT/trivy-db/.git" || true
  fi
}

prepare_operator_src_artifact() {
  # Mimics ee/modules/500-operator-trivy/images/operator/werf.inc.yaml src-artifact stage.
  log "== Artifact: operator-src-artifact (based on trivy-src-artifact) =="

  # operator-src-artifact is based on trivy-src-artifact; ensure base exists.
  prepare_trivy_src_artifact

  rm -rf "$OPERATOR_SRC_ARTIFACT_DIR"
  mkdir -p "$OPERATOR_SRC_ARTIFACT_DIR"
  cp -a "$TRIVY_SRC_ROOT" "$OPERATOR_SRC_ARTIFACT_DIR/" # => operator-src-artifact/src

  # Then clone operator into /src/trivy-operator and overlay its root to /src.
  git_sync "$OPERATOR_SRC_ROOT/trivy-operator" "$TRIVY_OPERATOR_REPO" "$TRIVY_OPERATOR_VERSION"
  rm -rf "$OPERATOR_SRC_ROOT/trivy-operator/.git"

  # mimic: mv /src/trivy-operator/* /src (no dotfiles)
  shopt -s nullglob
  mv "$OPERATOR_SRC_ROOT/trivy-operator/"* "$OPERATOR_SRC_ROOT/"
  shopt -u nullglob
  rm -rf "$OPERATOR_SRC_ROOT/trivy-operator"

  apply_patches_dir_sorted "$OPERATOR_SRC_ROOT" "$OPERATOR_PATCHES_DIR"

  # mimic: mkdir ./local && tar zxvf /bundle.tar.gz -C ./local
  if [[ -f "$OPERATOR_BUNDLE_TGZ" ]]; then
    mkdir -p "$OPERATOR_SRC_ROOT/local"
    tar zxf "$OPERATOR_BUNDLE_TGZ" -C "$OPERATOR_SRC_ROOT/local"
  else
    warn "Operator bundle not found (skipping unpack): $OPERATOR_BUNDLE_TGZ"
  fi

  # mimic: ln -s ./trivy-db ./original-trivy-db
  ( cd "$OPERATOR_SRC_ROOT" && ln -snf ./trivy-db ./original-trivy-db )

  if [[ "$CREATE_SHORTCUTS" == "1" ]]; then
    ln -snf "operator-src-artifact/src" "$WORKDIR/operator-src" || true
  fi
}

main() {
  need_cmd git
  need_cmd ssh
  need_cmd tar

  SELECTED_COMPONENTS="$(normalize_components "$COMPONENTS_RAW" | awk '!seen[$0]++')"

  log "Workdir: $WORKDIR"
  log "SOURCE_REPO=$SOURCE_REPO"
  log "DECKHOUSE_PRIVATE_REPO=$DECKHOUSE_PRIVATE_REPO"
  log "COMPONENTS=${COMPONENTS_RAW} (normalized: $(tr '\n' ',' <<<"$SELECTED_COMPONENTS" | sed 's/,$//'))"
  log "Versions: TRIVY_VERSION=$TRIVY_VERSION, TRIVY_OPERATOR_VERSION=$TRIVY_OPERATOR_VERSION, NODE_COLLECTOR_VERSION=$NODE_COLLECTOR_VERSION"

  mkdir -p "$WORKDIR"

  if component_selected trivy || component_selected trivy-db || component_selected trivy-operator; then
    ssh_hint_private_repo "$DECKHOUSE_PRIVATE_REPO"
  fi

  # In werf, trivy and trivy-db are produced together inside trivy-src-artifact (/src/...)
  # and then reused as a base image for trivy-operator src-artifact.
  if component_selected trivy || component_selected trivy-db || component_selected trivy-operator; then
    prepare_trivy_src_artifact
    if [[ "$CREATE_SHORTCUTS" == "1" ]]; then
      ln -snf "trivy-src-artifact/src" "$WORKDIR/trivy-src" || true
    fi
  fi

  if component_selected trivy-operator; then
    prepare_operator_src_artifact
  fi

  if component_selected k8s-node-collector; then
    log "== Component: k8s-node-collector =="
    git_sync "$WORKDIR/k8s-node-collector" "$NODE_COLLECTOR_REPO" "$NODE_COLLECTOR_VERSION"
    apply_patches_dir_sorted "$WORKDIR/k8s-node-collector" "$NODE_COLLECTOR_PATCHES_DIR"
  fi

  log "Done. Prepared sources are in: $WORKDIR"
  if [[ -d "$TRIVY_SRC_ROOT" ]]; then
    log "- trivy-src-artifact sources: $TRIVY_SRC_ROOT"
  fi
  if [[ -d "$OPERATOR_SRC_ROOT" ]]; then
    log "- operator-src-artifact sources: $OPERATOR_SRC_ROOT"
  fi
}

main "$@"
