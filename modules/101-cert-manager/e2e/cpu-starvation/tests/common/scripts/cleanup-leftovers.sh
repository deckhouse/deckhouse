#!/bin/sh

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Removes what an interrupted run of the CPU starvation scenarios left behind, in the cluster of the
# current kubectl context and on this machine:
#   - the background watchers and admission loops of e2e.sh;
#   - the stress Deployments of the suite (label e2e-test=cert-manager-cpu-starvation);
#   - the test namespaces of the suite (label e2e-test=cert-manager-cpu-starvation), through their
#     Project when multitenancy-manager made one of them;
#   - the ClusterIssuer of the tests;
#   - a controller pod resized in place, by a rollout restart of its Deployment;
#   - the state directories of the runs (tests/<SCENARIO>/reports/state-<NAMESPACE>) and the
#     temporary directories with the copies of the kubeconfig.
# Only objects with the label of the suite are deleted: other suites, for example the e2e tests of
# user-authn, also run in chainsaw-* namespaces and may use the label app=cpu-stress.
#
# Usage: cleanup-leftovers.sh

set -u

CM_NS=d8-cert-manager
ISSUER=e2e-cpu-starvation-selfsigned
SUITE_LABEL=e2e-test=cert-manager-cpu-starvation
SCRIPTS_DIR=$(cd "$(dirname "$0")" && pwd)
TESTS_DIR=$(dirname "$(dirname "$SCRIPTS_DIR")")
# Keep in sync with save_kubeconfig in e2e.sh.
KUBECONFIG_DIR_PREFIX=${TMPDIR:-/tmp}
KUBECONFIG_DIR_PREFIX=${KUBECONFIG_DIR_PREFIX%/}/cert-manager-cpu-starvation.

log() { printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }

kill_tree() {
  for CHILD_PID in $(pgrep -P "$1" 2>/dev/null || true); do
    kill_tree "$CHILD_PID"
  done
  kill "$1" 2>/dev/null || true
}

# remove_kubeconfig_dir DIR removes a temporary directory of e2e.sh with a copy of the kubeconfig.
remove_kubeconfig_dir() {
  case $1 in
    "$KUBECONFIG_DIR_PREFIX"*)
      if [ -d "$1" ]; then
        rm -rf "$1"
        log "removed the kubeconfig copy in $1"
      fi
      ;;
    *) log "skipping $1: not a kubeconfig directory of the suite" ;;
  esac
}

RESTART=

# 1. Background processes on this machine.
for DIR in "$TESTS_DIR"/*/reports/state-*/; do
  [ -d "$DIR" ] || continue
  DIR=${DIR%/}
  for PID_FILE in "$DIR"/bg-*.pid; do
    [ -f "$PID_FILE" ] || continue
    PID=$(cat "$PID_FILE")
    # Removing the pid file alone stops the loop; the kill does not wait for the next round.
    rm -f "$PID_FILE"
    if [ -n "$PID" ]; then
      kill_tree "$PID"
      log "stopped background process $PID ($PID_FILE)"
    fi
  done
  if [ "$(cat "$DIR/resized" 2>/dev/null)" = true ]; then
    RESTART="$RESTART $(cat "$DIR/target-deploy" 2>/dev/null || echo cert-manager)"
  fi
done
# Processes whose state directory is gone.
pkill -f "$SCRIPTS_DIR/watch-writes.sh" 2>/dev/null && log "stopped watchers without a state directory"
pkill -f "$SCRIPTS_DIR/e2e.sh admission-loop" 2>/dev/null && log "stopped admission loops without a state directory"

# 2. Stress Deployments of the suite.
STRESS=$(kubectl get deployments -A -l "$SUITE_LABEL" --request-timeout=60s \
  -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}') || {
  log "cannot list the stress Deployments; check the kubectl context"
  exit 1
}
for NS in $(printf '%s\n' "$STRESS" | sort -u); do
  kubectl -n "$NS" delete deployment -l "$SUITE_LABEL" --wait=true --timeout=120s --request-timeout=60s
done

# 3. Test namespaces of the suite. The chainsaw-* check guards against a label set by mistake.
NAMESPACES=$(kubectl get namespaces -l "$SUITE_LABEL" --request-timeout=60s \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}') || {
  log "cannot list the test namespaces; check the kubectl context"
  exit 1
}
for NS in $NAMESPACES; do
  case $NS in
    chainsaw-*) ;;
    *)
      log "skipping namespace $NS: not a Chainsaw test namespace"
      continue
      ;;
  esac
  if kubectl get projects.deckhouse.io "$NS" --request-timeout=60s >/dev/null 2>&1; then
    kubectl delete projects.deckhouse.io "$NS" --wait=true --timeout=4m --request-timeout=60s
  else
    kubectl delete namespace "$NS" --ignore-not-found --wait=true --timeout=4m --request-timeout=60s
  fi
done

# 4. ClusterIssuer of the tests.
kubectl delete clusterissuer "$ISSUER" --ignore-not-found --request-timeout=60s

# 5. A pod resized in place keeps the raised resources until it is recreated from the template.
for DEPLOY in $(printf '%s\n' "$RESTART" | tr ' ' '\n' | sort -u); do
  kubectl -n "$CM_NS" rollout restart "deployment/$DEPLOY" --request-timeout=60s &&
    kubectl -n "$CM_NS" rollout status "deployment/$DEPLOY" --timeout=300s
done

# 6. Copies of the kubeconfig and state directories.
for DIR in "$TESTS_DIR"/*/reports/state-*/; do
  [ -d "$DIR" ] || continue
  DIR=${DIR%/}
  KUBECONFIG_COPY=$(cat "$DIR/kubeconfig-path" 2>/dev/null || true)
  [ -z "$KUBECONFIG_COPY" ] || remove_kubeconfig_dir "$(dirname "$KUBECONFIG_COPY")"
  rm -rf "$DIR"
  log "removed $DIR"
done
# Copies whose state directory is gone.
for DIR in "$KUBECONFIG_DIR_PREFIX"*/; do
  [ -d "$DIR" ] || continue
  remove_kubeconfig_dir "${DIR%/}"
done

log "cleanup done"
