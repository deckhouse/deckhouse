#!/usr/bin/env bash

set -Eeo pipefail

function get_secret() {
  secret="$1"

  if type kubectl >/dev/null 2>&1 && test -f /etc/kubernetes/kubelet.conf ; then
    until kubectl --kubeconfig=/etc/kubernetes/kubelet.conf -n d8-cloud-instance-manager get secret $secret -o json; do
      >&2 echo "failed to get secret $secret with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
      sleep 10
    done
{{ if eq .runType "Normal" }}
  elif [ -f /var/lib/bashible/bootstrap-token ]; then
    while true; do
      for server in {{ .normal.apiserverEndpoints | join " " }}; do
        if curl -s -f -X GET "https://$server/api/v1/namespaces/d8-cloud-instance-manager/secrets/$secret" --header "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" --cacert "$BOOTSTRAP_DIR/ca.crt"
        then
          return 0
        else
          >&2 echo "failed to get secret $secret with curl https://$server..."
        fi
      done
      sleep 10
    done
{{ end }}
  else
    >&2 echo "failead to get secret $secret: can't find kubelet.conf or bootstrap-token"
    exit 1
  fi
}

function main() {
  export BOOTSTRAP_DIR="/var/lib/bashible"
  export BUNDLE_STEPS_DIR="$BOOTSTRAP_DIR/bundle_steps"
  export BUNDLE={{ .bundle }}

  mkdir -p "$BUNDLE_STEPS_DIR"

  # update bashible.sh itself
  if [ -z "${BASHIBLE_SKIP_UPDATE-}" ] && [ -z "${is_local-}" ]; then
    get_secret bashible-{{ .nodeGroup.name }}-${BUNDLE} | jq -r '.data."bashible.sh"' | base64 -d > $BOOTSTRAP_DIR/bashible-new.sh
    chmod +x $BOOTSTRAP_DIR/bashible-new.sh
    export BASHIBLE_SKIP_UPDATE=yes
    $BOOTSTRAP_DIR/bashible-new.sh --no-lock

    # At this step we already know that new version is functional
    mv $BOOTSTRAP_DIR/bashible-new.sh $BOOTSTRAP_DIR/bashible.sh
    exit 0
  fi

  if [ -z "${is_local-}" ]; then
    # update bashbooster library for idempotent scripting
    get_secret bashible-bashbooster -o json | jq -r '.data."bashbooster.sh"' | base64 -d > $BOOTSTRAP_DIR/bashbooster.sh

    # get steps from bundle secrets
    rm -rf $BUNDLE_STEPS_DIR/*
    bundle_collections="bashible-bundle-${BUNDLE}-{{ .kubernetesVersion }} bashible-bundle-${BUNDLE}-{{ .nodeGroup.name }}"
    for bundle_collection in $bundle_collections; do
      collection_data="$(get_secret $bundle_collection | jq -r '.data')"
      for step in $(jq -r 'to_entries[] | .key' <<< "$collection_data"); do
        jq -r --arg step "$step" '.[$step]' <<< "$collection_data" | base64 -d > "$BUNDLE_STEPS_DIR/$step"
      done
    done
  fi

  # Execute bashible steps
  rm -rf $BOOTSTRAP_DIR/.bb-workspace
  for step in $BUNDLE_STEPS_DIR/*; do
  echo ===
  echo === Step: $step
  echo ===
    until /bin/bash -eEo pipefail -c "export TERM=xterm-256color; unset CDPATH; cd $BOOTSTRAP_DIR; source /var/lib/bashible/bashbooster.sh; source $step"
    do
      >&2 echo "Failed to execute step "$step" ... retry in 10 seconds."
      sleep 10
      echo ===
      echo === Step: $step
      echo ===
    done
  done
}

while true ; do
  case ${1:-} in
    --local)
      export is_local=yes
      shift
      ;;
    "--no-lock")
      export no_lock=yes
      shift
      ;;
    *)
      break
      ;;
  esac
done

if [ -n "${no_lock-}" ]; then
  main
else
  (
    flock -n 200 || { >&2 echo "Can't acquire lockfile /var/lock/bashible."; exit 1; }
    main
  ) 200>/var/lock/bashible
fi
