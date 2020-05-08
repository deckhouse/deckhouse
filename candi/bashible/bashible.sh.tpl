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
  export BUNDLE="{{ .bundle }}"
  export CONFIGURATION_CHECKSUM_FILE="/var/lib/bashible/configuration_checksum"
  export CONFIGURATION_CHECKSUM="{{ .configurationChecksum | default "" }}"

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

{{ if eq .runType "Normal" }}
  if [[ "$(<$CONFIGURATION_CHECKSUM_FILE)" == "$CONFIGURATION_CHECKSUM" ]]; then
    echo "Configuration is in sync, nothing to do."
    exit 0
  fi
{{ end }}

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

{{ if eq .runType "Normal" }}
  if [ -f $CONFIGURATION_CHECKSUM_FILE ]; then
    >&2 echo "Setting update.node.deckhouse.io/waiting-for-approval= annotation on our Node..."
    attempt=0
    until
      node_data="$(
        kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get node "${HOSTNAME}" -o json | jq '
        {
          "resourceVersion": .metadata.resourceVersion,
          "isApproved": (.metadata.annotations | has("update.node.deckhouse.io/approved")),
          "isWaitingForApproval": (.metadata.annotations | has("update.node.deckhouse.io/waiting-for-approval"))
        }
      ')" &&
       jq -ne --argjson n "$node_data" '(($n.isApproved | not) and ($n.isWaitingForApproval)) or ($n.isApproved)' >/dev/null
    do
      attempt=$(( attempt + 1 ))
      if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
        >&2 echo "ERROR: Can't set update.node.deckhouse.io/waiting-for-approval= annotation on our Node."
        exit 1
      fi
      kubectl \
        --kubeconfig=/etc/kubernetes/kubelet.conf annotate node "${HOSTNAME}" \
        --resource-version="$(jq -nr --argjson n "$node_data" '$n.resourceVersion')" \
        update.node.deckhouse.io/waiting-for-approval= || { echo "Retry setting update.node.deckhouse.io/waiting-for-approval= annotation on our Node in 10sec..."; sleep 10; }
    done

    >&2 echo "Waiting for update.node.deckhouse.io/approved= annotation on our Node..."
    attempt=0
    until
      kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get node "${HOSTNAME}" -o json | \
      jq -e '.metadata.annotations | has("update.node.deckhouse.io/approved")' >/dev/null
    do
      attempt=$(( attempt + 1 ))
      if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
        >&2 echo "ERROR: Can't get annotation 'update.node.deckhouse.io/approved' from our Node."
        exit 1
      fi
      echo "Steps are waiting for approval to start:"
      echo "kubectl annotate node ${HOSTNAME} update.node.deckhouse.io/approved="
      echo "Retry in 10sec..."
      sleep 10
    done
  fi
{{ end }}

  # Execute bashible steps
  rm -rf $BOOTSTRAP_DIR/.bb-workspace
  for step in $BUNDLE_STEPS_DIR/*; do
    echo ===
    echo === Step: $step
    echo ===
    attempt=0
    until /bin/bash -eEo pipefail -c "export TERM=xterm-256color; unset CDPATH; cd $BOOTSTRAP_DIR; source /var/lib/bashible/bashbooster.sh; source $step"
    do
      attempt=$(( attempt + 1 ))
      if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
        >&2 echo "ERROR: Failed to execute step $step. Retry limit is over."
        exit 1
      fi
      >&2 echo "Failed to execute step "$step" ... retry in 10 seconds."
      sleep 10
      echo ===
      echo === Step: $step
      echo ===
    done
  done

{{ if eq .runType "Normal" }}
  kubectl --kubeconfig=/etc/kubernetes/kubelet.conf annotate node ${HOSTNAME} --overwrite node.deckhouse.io/configuration-checksum="${CONFIGURATION_CHECKSUM}"
  echo "$CONFIGURATION_CHECKSUM" > $CONFIGURATION_CHECKSUM_FILE
{{ end }}
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
    "--max-retries")
      export MAX_RETRIES="$2"
      shift
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
