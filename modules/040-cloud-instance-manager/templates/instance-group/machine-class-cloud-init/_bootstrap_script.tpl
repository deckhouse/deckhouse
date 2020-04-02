{{- define "instance_group_machine_class_bashible_bootstrap_script" }}
  {{- $context := index . 0 }}
  {{- $ig := index . 1 }}

  {{- $bashible_bundle := $ig.instanceClass.bashible.bundle -}}
#!/bin/bash

set -Eeuom pipefail
shopt -s failglob

#Install necessary packages. Not in cloud config cause cloud init do not retry installation and silently fails.
{{- if hasPrefix "centos" $bashible_bundle }}
until yum install epel-release -y; do
  echo "Error installing epel-release"
  sleep 10
done
until yum install jq nc wget -y; do
  echo "Error installing packages"
  sleep 10
done
{{- else if hasPrefix "ubuntu" $bashible_bundle }}
export DEBIAN_FRONTEND=noninteractive
until apt install jq wget -y; do
  echo "Error installing packages"
  sleep 10
done
{{- end }}

BOOTSTRAP_DIR="/var/lib/bashible"

# Directory contains sensitive information
chmod 0700 $BOOTSTRAP_DIR

# Execute cloud provider specific bootstrap.
if [[ -f $BOOTSTRAP_DIR/cloud-provider-bootstrap-{{ $bashible_bundle }}.sh ]] ; then
  while true ; do
    if ! $BOOTSTRAP_DIR/cloud-provider-bootstrap-{{ $bashible_bundle }}.sh ; then
      >&2 echo "Failed to execute cloud provider specific bootstrap. Retry in 10 seconds."
      sleep 10
      continue
    fi

    break
  done
fi

# Start output bootstrap logs
output_log_port=8000
while true; do cat /var/log/cloud-init-output.log | nc -l $output_log_port; done &

patch_pending=true
while [ "$patch_pending" = true ] ; do
  for server in {{ $context.Values.cloudInstanceManager.internal.clusterMasterAddresses | join " " | quote }} ; do
    server_addr=$(echo $server | cut -f1 -d":")
    tcp_endpoint=$(ip ro get ${server_addr} | grep -Po '(?<=src )([0-9\.]+)')
    if curl -s --fail \
      --max-time 10 \
      -XPATCH \
      -H "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" \
      -H "Accept: application/json" \
      -H "Content-Type: application/json-patch+json" \
      --cacert "$BOOTSTRAP_DIR/ca.crt" \
      --data "[{\"op\":\"add\",\"path\":\"/status/bootstrapStatus\", \"value\": {\"description\": \"Use 'nc ${tcp_endpoint} ${output_log_port}' to get bootstrap logs.\", \"tcpEndpoint\": \"${tcp_endpoint}\"} }]" \
      "https://$server/apis/machine.sapcloud.io/v1alpha1/namespaces/d8-cloud-instance-manager/machines/$(hostname)/status" ; then

      echo "Successfully patched machine $(hostname) status."
      patch_pending=false
      break
    else
      >&2 echo "Failed to patch machine $(hostname) status."
      sleep 10
      continue
    fi
  done
done

until /var/lib/bashible/bashible.sh bootstrap; do
  echo "Error running bashible script. Retry in 10 seconds."
  sleep 10
done;

# Stop output bootstrap logs
kill -9 %1

if [[ -f "/var/lib/bashible/reboot" ]]; then
  echo "Reboot machine after bootstrap process completed"
  rm -f /var/lib/bashible/reboot
  (sleep 5; shutdown -r now) &
fi

{{ end }}
