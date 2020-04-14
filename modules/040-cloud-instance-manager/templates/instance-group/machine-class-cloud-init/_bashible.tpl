{{- define "instance_group_machine_class_bashible_bashible_script" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
#!/usr/bin/env bash

set -Eeom pipefail

BOOTSTRAP_DIR="/var/lib/bashible"
BUNDLE_STEPS_DIR="$BOOTSTRAP_DIR/bundle_steps"
mkdir -p "$BUNDLE_STEPS_DIR"

BASHIBLE_BUNDLE="$(cat $BOOTSTRAP_DIR/bundle)"

# How to authenticate in kubernetes API
if [ -f /var/lib/bashible/bootstrap-token ] ; then
  wget_auth=(--ca-certificate="$BOOTSTRAP_DIR/ca.crt" --header="Authorization: Bearer $(</var/lib/bashible/bootstrap-token)")
else
  wget_auth=(--ca-certificate=/etc/kubernetes/pki/ca.crt --certificate=/var/lib/kubelet/pki/kubelet-client-current.pem)
fi

if [ -f "/etc/kubernetes/kubernetes-api-proxy/nginx.conf" ] ; then
  servers="kubernetes:6445"
else
  servers={{ $context.Values.cloudInstanceManager.internal.clusterMasterAddresses | join " " | quote }}
fi

# Get bashible steps from Secret resources
bundle_collections="bundle-${BASHIBLE_BUNDLE} bundle-${BASHIBLE_BUNDLE}-{{ $ng.name }}"
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
      cat $BOOTSTRAP_DIR/${bundle_collection}.json | jq '."'$step'"' -r | base64 -d > "$BUNDLE_STEPS_DIR/$step"
      chmod +x "$BUNDLE_STEPS_DIR/$step"
    done

    break
  done
done

# Execute bashible steps
for step in $BUNDLE_STEPS_DIR/*; do
  while true; do
    if ! (
      unset CDPATH
      cd $BOOTSTRAP_DIR

      if [ -f /var/lib/bashible/bashbooster.sh ]; then source /var/lib/bashible/bashbooster.sh; fi

      set -Eeo pipefail

      . $step
    ) ; then
      >&2 echo "Failed to execute step "$step". Retry in 10 seconds."
      sleep 10
      continue
    fi

    break
  done
done
{{ end }}
