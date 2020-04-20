{{- define "instance_group_machine_class_bashible_bashible_script" }}
  {{- $context := index . 0 }}
  {{- $ig := index . 1 }}

  {{- $bashible_bundle := $ig.instanceClass.bashible.bundle -}}
#!/bin/bash

function get_secret() {
  secret="$1"

  if type kubectl >/dev/null 2>&1 && test -f /etc/kubernetes/kubelet.conf ; then
    until kubectl --kubeconfig=/etc/kubernetes/kubelet.conf -n d8-cloud-instance-manager get secret $secret -o json; do
      >&2 echo "failed to get secret $secret with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
      sleep 10
    done
  elif [ -f /var/lib/bashible/bootstrap-token ]; then
    while true; do
      for server in {{ $context.Values.cloudInstanceManager.internal.clusterMasterAddresses | join " " }}; do
        if curl -s -f -X GET "https://$server/api/v1/namespaces/d8-cloud-instance-manager/secrets/$secret" --header "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" --cacert "$BOOTSTRAP_DIR/ca.crt"
        then
          return 0
        else
          >&2 echo "failed to get secret $secret with curl https://$server..."
        fi
      done
      sleep 10
    done
  else
    >&2 echo "failead to get secret $secret: can't find kubelet.conf or bootstrap-token"
    exit 1
  fi
}

set -Eeuom pipefail
shopt -s failglob

BOOTSTRAP_DIR="/var/lib/bashible"

# update bashible.sh itself
if [ -z "${BASHIBLE_SKIP_UPDATE-}" ]; then
  get_secret bashible-{{ $ig.name }}-{{ $bashible_bundle | trimSuffix "-1.0" }} | jq -r '.data."bashible.sh"' | base64 -d > $BOOTSTRAP_DIR/bashible-new.sh
  chmod +x $BOOTSTRAP_DIR/bashible-new.sh
  export BASHIBLE_SKIP_UPDATE=yes
  $BOOTSTRAP_DIR/bashible-new.sh ${@}

  # At this step we already know that new version is functional
  mv $BOOTSTRAP_DIR/bashible-new.sh $BOOTSTRAP_DIR/bashible.sh
  exit 0
fi

# Download and extract bashible bundle
if [[ $# -eq 1 && "x$1" == "xbootstrap" ]] ; then
  bundle_dir="bundle-bootstrap"
  bundle_collections="bundle-{{ $bashible_bundle }} bundle-{{ $bashible_bundle }}-bootstrap bundle-{{ $bashible_bundle }}-{{ $ig.name }} bundle-{{ $bashible_bundle }}-{{ $ig.name }}-bootstrap"
  wget_auth=(--ca-certificate="$BOOTSTRAP_DIR/ca.crt" --header="Authorization: Bearer $(</var/lib/bashible/bootstrap-token)")
else
  bundle_dir="bundle"
  bundle_collections="bundle-{{ $bashible_bundle }} bundle-{{ $bashible_bundle }}-{{ $ig.name }}"
  wget_auth=(--ca-certificate=/etc/kubernetes/pki/ca.crt --certificate=/var/lib/kubelet/pki/kubelet-client-current.pem)
fi
mkdir -p "$BOOTSTRAP_DIR/$bundle_dir"
if [ -f "/etc/kubernetes/kubernetes-api-proxy/nginx.conf" ] ; then
  servers="kubernetes:6445"
else
  servers={{ $context.Values.cloudInstanceManager.internal.clusterMasterAddresses | join " " | quote }}
fi
for bundle_collection in $bundle_collections ; do
  while true ; do
    for server in ${servers} ; do
      if wget -O /dev/stdout -q --timeout=10 \
        "${wget_auth[@]}" \
        --header="Accept: application/json" \
        "https://$server/api/v1/namespaces/d8-cloud-instance-manager/secrets/bashible-${bundle_collection}" | jq .data > $BOOTSTRAP_DIR/${bundle_collection}.json ; then

        if [[ -s $BOOTSTRAP_DIR/${bundle_collection}.json ]] ; then
          echo "Successfully downloaded bashible collection "$bundle_collection" from https://$server/."
          break
        fi
      else
        >&2 echo "Failed to download bashible collection "$bundle_collection" from https://$server/."
      fi
    done

    if [[ ! -s $BOOTSTRAP_DIR/${bundle_collection}.json ]] ; then
      >&2 echo "Failed to download bashible collection "$bundle_collection" from all servers. Retry in 10 seconds."
      sleep 10
      continue
    fi

    steps=$(cat $BOOTSTRAP_DIR/${bundle_collection}.json | jq '. // {} | keys | .[]' -r)
    for step in $steps; do
      cat $BOOTSTRAP_DIR/${bundle_collection}.json | jq '."'$step'"' -r | base64 -d > "$BOOTSTRAP_DIR/$bundle_dir/$step"
    done

    break
  done
done

# Execute bashible steps
for step in $(ls -1 $BOOTSTRAP_DIR/$bundle_dir/ | sort); do
  while true; do
    if ! (
      set -Eeuo pipefail
      shopt -s failglob

      . $BOOTSTRAP_DIR/$bundle_dir/$step
    ) ; then
      >&2 echo "Failed to execute step "$step". Retry in 10 seconds."
      sleep 10
      continue
    fi

    break
  done
done
{{ end }}
