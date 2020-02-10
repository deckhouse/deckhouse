{{- define "instance_group_machine_class_bashible_bashible_script" }}
  {{- $context := index . 0 }}
  {{- $ig := index . 1 }}
  {{- $zone_name := index . 2 }}

  {{- $bashible_bundle := $ig.instanceClass.bashible.bundle -}}
#!/bin/bash

set -Eeuom pipefail
shopt -s failglob

BOOTSTRAP_DIR="/var/lib/bashible"

# Download and extract bashible bundle
if [[ $# -eq 1 && "x$1" == "xbootstrap" ]] ; then
  bundle_dir="bundle-bootstrap"
  bundle_collections="bundle-{{ $bashible_bundle }} bundle-{{ $bashible_bundle }}-bootstrap bundle-{{ $bashible_bundle }}-{{ $ig.name }} bundle-{{ $bashible_bundle }}-{{ $ig.name }}-bootstrap"
  wget_auth=(--ca-certificate="$BOOTSTRAP_DIR/ca.crt" --header="Authorization: Bearer {{ $context.Values.cloudInstanceManager.internal.bootstrapToken }}")
else
  bundle_dir="bundle"
  bundle_collections="bundle-{{ $bashible_bundle }} bundle-{{ $bashible_bundle }}-{{ $ig.name }}"
  wget_auth=(--ca-certificate=/etc/kubernetes/pki/ca.crt --certificate=/var/lib/kubelet/pki/kubelet-client-current.pem)
fi
mkdir -p "$BOOTSTRAP_DIR/$bundle_dir"
for bundle_collection in $bundle_collections ; do
  while true ; do
    for server in {{ $context.Values.cloudInstanceManager.internal.clusterMasterAddresses | join " " }} ; do
      if wget -O /dev/stdout -q --timeout=10 \
        "${wget_auth[@]}" \
        --header="Accept: application/json" \
        "https://$server:6443/api/v1/namespaces/d8-cloud-instance-manager/secrets/bashible-${bundle_collection}" | jq .data > $BOOTSTRAP_DIR/${bundle_collection}.json ; then

        if [[ -s $BOOTSTRAP_DIR/${bundle_collection}.json ]] ; then
          echo "Successfully downloaded bashible collection "$bundle_collection" from https://$server:6443/."
          break
        fi
      else
        >&2 echo "Failed to download bashible collection "$bundle_collection" from https://$server:6443/."
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
