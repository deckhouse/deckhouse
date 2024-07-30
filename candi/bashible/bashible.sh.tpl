#!/usr/bin/env bash

# Copyright 2021 Flant JSC
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
export LANG=C LC_NUMERIC=C
set -Eeo pipefail

function kubectl_exec() {
  kubectl --request-timeout 60s --kubeconfig=/etc/kubernetes/kubelet.conf ${@}
}

function bb-event-error-create() {
    # This function is used for creating event in the default namespace with reference of
    # bashible step and used events.k8s.io/v1 apiVersion.
    # eventName aggregates hostname with bashible step - sed keep only name and replace
    # underscore with dash due to regexp.
    # nodeName is used for both .name and .uid fields intentionally as putting a real node uid
    # has proven to have some side effects like missing events when describing objects
    # using kubectl versions 1.23.x (https://github.com/deckhouse/deckhouse/issues/4609).
    # All of stderr outputs are stored in the eventLog file.
    # step is used as argument for function call.
    # If event creation failed, error from kubectl suppressed.
    step="$1"
    eventName="$(echo -n "${D8_NODE_HOSTNAME}")-$(echo $step | sed 's#.*/##; s/_/-/g')"
    nodeName="${D8_NODE_HOSTNAME}"
    eventLog="/var/lib/bashible/step.log"
    kubectl_exec apply -f - <<EOF || true
        apiVersion: events.k8s.io/v1
        kind: Event
        metadata:
          name: bashible-error-${eventName}
        regarding:
          apiVersion: v1
          kind: Node
          name: ${nodeName}
          uid: ${nodeName}
        note: '$(tail -c 500 ${eventLog})'
        reason: BashibleStepFailed
        type: Warning
        reportingController: bashible
        reportingInstance: '${D8_NODE_HOSTNAME}'
        eventTime: '$(date -u +"%Y-%m-%dT%H:%M:%S.%6NZ")'
        action: "BashibleStepExecution"
EOF
}

function annotate_node() {
  echo "Annotate node ${D8_NODE_HOSTNAME} with annotation ${@}"
  attempt=0
  until error=$(kubectl_exec annotate node ${D8_NODE_HOSTNAME} --overwrite ${@} 2>&1); do
    attempt=$(( attempt + 1 ))
    if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
      >&2 echo "ERROR: Failed to annotate node ${D8_NODE_HOSTNAME} with annotation ${@} after ${MAX_RETRIES} retries. Last error from kubectl: ${error}"
      exit 1
    fi
    if [ "$attempt" -gt "2" ]; then
      >&2 echo "Failed to annotate node ${D8_NODE_HOSTNAME} with annotation ${@} after 3 tries. Last message from kubectl: ${error}"
      >&2 echo "Retrying..."
      attempt=0
    fi
    sleep 10
  done
  echo "Succesful annotate node ${D8_NODE_HOSTNAME} with annotation ${@}"
}

function get_secret() {
  secret="$1"

  if type kubectl >/dev/null 2>&1 && test -f /etc/kubernetes/kubelet.conf ; then
    attempt=0
    until kubectl_exec -n d8-cloud-instance-manager get secret "$secret" -o json; do
      attempt=$(( attempt + 1 ))
      if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
        >&2 echo "ERROR: Failed to get secret $secret with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
        exit 1
      fi
      >&2 echo "failed to get secret $secret with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
      sleep 10
    done
{{ if eq .runType "Normal" }}
  elif [ -f /var/lib/bashible/bootstrap-token ]; then
    token="$(</var/lib/bashible/bootstrap-token)"
    while true; do
      for server in {{ .normal.apiserverEndpoints | join " " }}; do
        url="https://$server/api/v1/namespaces/d8-cloud-instance-manager/secrets/$secret"
        if d8-curl -sS -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
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
    >&2 echo "failed to get secret $secret: can't find kubelet.conf or bootstrap-token"
    exit 1
  fi
}

function get_bundle() {
  resource="$1"
  name="$2"

  if type kubectl >/dev/null 2>&1 && test -f /etc/kubernetes/kubelet.conf ; then
    attempt=0
    until kubectl_exec get "$resource" "$name" -o json; do
      attempt=$(( attempt + 1 ))
      if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
        >&2 echo "ERROR: Failed to get $resource $name with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
        exit 1
      fi
      >&2 echo "failed to get $resource $name with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
      sleep 10
    done
{{ if eq .runType "Normal" }}
  elif [ -f /var/lib/bashible/bootstrap-token ]; then
    token="$(</var/lib/bashible/bootstrap-token)"
    while true; do
      for server in {{ .normal.apiserverEndpoints | join " " }}; do
        url="https://$server/apis/bashible.deckhouse.io/v1alpha1/${resource}s/${name}"
        if d8-curl -sS -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
        then
         return 0
        else
          >&2 echo "failed to get $resource $name with curl https://$server..."
        fi
      done
      sleep 10
    done
{{ end }}
  else
    >&2 echo "failed to get $resource $name: can't find kubelet.conf or bootstrap-token"
    exit 1
  fi
}

function current_uptime() {
  cat /proc/uptime | cut -d " " -f1
}

function main() {
  export PATH="/opt/deckhouse/bin:/usr/local/bin:$PATH"
  export BOOTSTRAP_DIR="/var/lib/bashible"
  export BUNDLE_STEPS_DIR="$BOOTSTRAP_DIR/bundle_steps"
  export BUNDLE="{{ .bundle }}"
  export CONFIGURATION_CHECKSUM_FILE="$BOOTSTRAP_DIR/configuration_checksum"
  export UPTIME_FILE="$BOOTSTRAP_DIR/uptime"
  export CONFIGURATION_CHECKSUM="{{ .configurationChecksum | default "" }}"
  export FIRST_BASHIBLE_RUN="no"
  export NODE_GROUP="{{ .nodeGroup.name }}"
  export TMPDIR="/opt/deckhouse/tmp"
{{- if .registry }}
  export REGISTRY_ADDRESS="{{ .registry.address }}"
  export SCHEME="{{ .registry.scheme }}"
  export REGISTRY_PATH="{{ .registry.path }}"
  export REGISTRY_AUTH="$(base64 -d <<< "{{ .registry.auth | default "" }}")"
{{- end }}
{{- if .packagesProxy }}
  export PACKAGES_PROXY_ADDRESSES="{{ .packagesProxy.addresses | join "," }}"
  export PACKAGES_PROXY_TOKEN="{{ .packagesProxy.token }}"
{{- end }}
{{- if .proxy }}
  {{- if .proxy.httpProxy }}
  export HTTP_PROXY={{ .proxy.httpProxy | quote }}
  export http_proxy=${HTTP_PROXY}
  {{- end }}
  {{- if .proxy.httpsProxy }}
  export HTTPS_PROXY={{ .proxy.httpsProxy | quote }}
  export https_proxy=${HTTPS_PROXY}
  {{- end }}
  {{- if .proxy.noProxy }}
  export NO_PROXY={{ .proxy.noProxy | join "," | quote }}
  export no_proxy=${NO_PROXY}
  {{- end }}
{{- else }}
  unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy
{{- end }}
{{- if and (ne .nodeGroup.nodeType "Static") (ne .nodeGroup.nodeType "CloudStatic" )}}
  export D8_NODE_HOSTNAME=$(hostname -s)
{{- else }}
  export D8_NODE_HOSTNAME=$(hostname)
{{- end }}

  if type kubectl >/dev/null 2>&1 && test -f /etc/kubernetes/kubelet.conf ; then
    if tmp="$(kubectl_exec get node ${D8_NODE_HOSTNAME} -o json | jq -r '.metadata.labels."node.deckhouse.io/group"')" ; then
      NODE_GROUP="$tmp"
      if [ "${NODE_GROUP}" == "null" ] ; then
        >&2 echo "failed to get node group. Forgot set label 'node.deckhouse.io/group'"
      fi
    fi
  fi

  if [ -f /var/lib/bashible/first_run ] ; then
    FIRST_BASHIBLE_RUN="yes"
  fi

  mkdir -p "$BUNDLE_STEPS_DIR" "$TMPDIR"

  # update bashible.sh itself
  if [ -z "${BASHIBLE_SKIP_UPDATE-}" ] && [ -z "${is_local-}" ]; then
    get_bundle bashible "${BUNDLE}.${NODE_GROUP}" | jq -r '.data."bashible.sh"' > $BOOTSTRAP_DIR/bashible-new.sh
    if [ ! -s $BOOTSTRAP_DIR/bashible-new.sh ] ; then
      >&2 echo "ERROR: Got empty $BOOTSTRAP_DIR/bashible-new.sh."
      exit 1
    fi
    read -r first_line < $BOOTSTRAP_DIR/bashible-new.sh
    if [[ "$first_line" != '#!/usr/bin/env bash' ]] ; then
      >&2 echo "ERROR: $BOOTSTRAP_DIR/bashible-new.sh is not a bash script."
      exit 1
    fi
    chmod +x $BOOTSTRAP_DIR/bashible-new.sh
    export BASHIBLE_SKIP_UPDATE=yes
    $BOOTSTRAP_DIR/bashible-new.sh --no-lock

    # At this step we already know that new version is functional
    mv $BOOTSTRAP_DIR/bashible-new.sh $BOOTSTRAP_DIR/bashible.sh
    sync $BOOTSTRAP_DIR/bashible.sh
    exit 0
  fi

{{ if eq .runType "Normal" }}
  if [[ -f $CONFIGURATION_CHECKSUM_FILE ]] && [[ "$(<$CONFIGURATION_CHECKSUM_FILE)" == "$CONFIGURATION_CHECKSUM" ]] && [[ -f $UPTIME_FILE ]] && [[ "$(<$UPTIME_FILE)" < "$(current_uptime)" ]] 2>/dev/null; then
    echo "Configuration is in sync, nothing to do."
    annotate_node node.deckhouse.io/configuration-checksum=${CONFIGURATION_CHECKSUM}
    current_uptime > $UPTIME_FILE
    exit 0
  fi
  rm -f "$CONFIGURATION_CHECKSUM_FILE"
{{ end }}

  if [ -z "${is_local-}" ]; then
    # update bashbooster library for idempotent scripting
    get_secret bashible-bashbooster | jq -r '.data."bashbooster.sh"' | base64 -d > $BOOTSTRAP_DIR/bashbooster.sh

    # Get steps from bashible apiserver

    rm -rf "$BUNDLE_STEPS_DIR"/*

    ng_steps_collection="$(get_bundle nodegroupbundle "${BUNDLE}.${NODE_GROUP}" | jq -rc '.data')"

    for step in $(jq -r 'to_entries[] | .key' <<< "$ng_steps_collection"); do
      jq -r --arg step "$step" '.[$step] // ""' <<< "$ng_steps_collection" > "$BUNDLE_STEPS_DIR/$step"
    done

  fi

  # Execute bashible steps
  for step in $BUNDLE_STEPS_DIR/*; do
    echo ===
    echo === Step: $step
    echo ===
    attempt=0
    sx=""
    until /bin/bash -"$sx"eEo pipefail -c "export TERM=xterm-256color; unset CDPATH; cd $BOOTSTRAP_DIR; source /var/lib/bashible/bashbooster.sh; source $step" 2> >(tee /var/lib/bashible/step.log >&2)
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
      if [ "$attempt" -gt 2 ]; then
        sx=x
      fi
      {{- if ne .runType "ClusterBootstrap" }}
      bb-event-error-create "$step"
      {{- end }}
    done
  done

{{ if eq .runType "Normal" }}
  annotate_node node.deckhouse.io/configuration-checksum=${CONFIGURATION_CHECKSUM}

  echo "$CONFIGURATION_CHECKSUM" > $CONFIGURATION_CHECKSUM_FILE
  current_uptime > $UPTIME_FILE
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
