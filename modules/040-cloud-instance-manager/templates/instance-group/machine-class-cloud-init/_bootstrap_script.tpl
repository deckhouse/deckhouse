{{- define "instance_group_machine_class_cloud_init_bootstrap_script" }}
  {{- $context := index . 0 }}
  {{- $ig := index . 1 }}
  {{- $zone_name := index . 2 }}

  {{- $cloud_init_steps_version := $ig.instanceClass.cloudInitSteps.version | default $context.Values.cloudInstanceManager.internal.cloudInitSteps.version -}}
#!/bin/bash

set -Eeuo pipefail
shopt -s inherit_errexit
shopt -s failglob

BOOTSTRAP_DIR="/var/lib/machine-bootstrap"

# Directory contains sensitive information
chmod 0700 $BOOTSTRAP_DIR

# Execute cloud provider specific bootstrap.
if [[ -f $BOOTSTRAP_DIR/cloud-provider-bootstrap-{{ $cloud_init_steps_version }}.sh ]] ; then
  while true ; do
    if ! $BOOTSTRAP_DIR/cloud-provider-bootstrap-{{ $cloud_init_steps_version }}.sh ; then
      >&2 echo "Failed to execute cloud provider specific bootstrap. Retry in 10 seconds."
      sleep 10
      continue
    fi

    break
  done
fi

# Download and extract cloud init steps
mkdir -p $BOOTSTRAP_DIR/steps
for steps_collection in steps-{{ $cloud_init_steps_version }} steps-{{ $cloud_init_steps_version }}-{{ $ig.name }} ; do
  while true ; do
    for server in {{ $context.Values.cloudInstanceManager.internal.clusterMasterAddresses | join " " }} ; do
      if curl -s --fail \
        --max-time 10 \
        -H "Authorization: Bearer {{ $context.Values.cloudInstanceManager.internal.bootstrapToken }}" \
        -H "Accept: application/json" \
        --cacert "$BOOTSTRAP_DIR/ca.crt" \
        "https://$server:6443/api/v1/namespaces/d8-cloud-instance-manager/secrets/cloud-init-${steps_collection}" | jq .data > $BOOTSTRAP_DIR/${steps_collection}.json ; then

        if [[ ! -s $BOOTSTRAP_DIR/${steps_collection}.json ]] ; then
          echo "Successfully downloaded cloud init steps collection "$steps_collection" from https://$server:6443/."
          break
        fi
      else
        >&2 echo "Failed to download cloud init steps collection "$steps_collection" from https://$server:6443/."
      fi
    done

    if [[ ! -s $BOOTSTRAP_DIR/${steps_collection}.json ]] ; then
      >&2 echo "Failed to download cloud init steps collection "$steps_collection" from all servers. Retry in 10 seconds."
      sleep 10
      continue
    fi

    steps=$(cat $BOOTSTRAP_DIR/${steps_collection}.json | jq '. // {} | keys | .[]' -r)
    for step in $steps; do
      cat $BOOTSTRAP_DIR/${steps_collection}.json | jq '."'$step'"' -r | base64 -d > $BOOTSTRAP_DIR/steps/$step
    done

    break
  done
done

# Execute cloud init steps
for step in $(ls -1 $BOOTSTRAP_DIR/steps/ | sort); do
  while true; do
    if ! (
      set -Eeuo pipefail
      shopt -s inherit_errexit
      shopt -s failglob

      . $BOOTSTRAP_DIR/steps/$step
    ) ; then
      >&2 echo "Failed to execute step "$step". Retry in 10 seconds."
      sleep 10
      continue
    fi

    break
  done
done
{{ end }}
