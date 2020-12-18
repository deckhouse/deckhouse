#!/bin/bash

set -Eeo pipefail
shopt -s inherit_errexit
shopt -s failglob

test_failed=""

function cleanup() {
  cleanup_exit_code=0

  if [[ -z "$master_ip" ]]; then
    candictl bootstrap-phase abort --config "$cwd/configuration.yaml" --yes-i-am-sane-and-i-understand-what-i-am-doing || cleanup_exit_code="$?"
  else
    {
      candictl destroy --ssh-agent-private-keys "$ssh_private_key_path" --ssh-user "$ssh_user" --ssh-host "$master_ip" \
        --yes-i-am-sane-and-i-understand-what-i-am-doing || \
      candictl bootstrap-phase abort --config "$cwd/configuration.yaml" --yes-i-am-sane-and-i-understand-what-i-am-doing
    } || cleanup_exit_code="$?"
  fi

  {
    chmod -R 777 /tmp
    chmod -R 777 /deckhouse
  } || cleanup_exit_code="$?"

  if [[ -n "$test_failed" ]]; then
    exit 1
  fi

  exit "$cleanup_exit_code"
}
trap cleanup EXIT

function fail_test() {
  test_failed="true"
}
trap fail_test ERR

root_wd="$(pwd)/testing/cloud_layouts"
cwd="$root_wd/$PROVIDER/$LAYOUT"
if [[ ! -d "$cwd" ]]; then
  >&2 echo "There is no cloud layout configuration by path: $cwd"
  exit 1
fi

ssh_private_key_path="$cwd/sshkey"
ssh_public_key_path="$cwd/sshkey.pub"
rm -f "$ssh_private_key_path"
ssh-keygen -b 2048 -t rsa -f "$ssh_private_key_path" -q -N "" <<< y
ssh_public_key=$(<"$ssh_public_key_path")

if [[ -z "$KUBERNETES_VERSION" ]]; then
  # shellcheck disable=SC2016
  >&2 echo 'Provide ${KUBERNETES_VERSION}!'
  exit 1
fi
if [[ -z "$DEV_BRANCH" ]]; then
  # shellcheck disable=SC2016
  >&2 echo 'Provide ${DEV_BRANCH}!'
  exit 1
fi
if [[ -z "$PREFIX" ]]; then
  # shellcheck disable=SC2016
  >&2 echo 'Provide ${PREFIX}!'
  exit 1
fi

if [[ "$PROVIDER" == "Yandex.Cloud" ]]; then
  # shellcheck disable=SC2016
  env CLOUD_ID="$(base64 -d <<< "$LAYOUT_YANDEX_CLOUD_ID")" FOLDER_ID="$(base64 -d <<< "$LAYOUT_YANDEX_FOLDER_ID")" \
      SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON")" SSH_PUBLIC_KEY="$ssh_public_key" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CLOUD_ID} ${FOLDER_ID} ${SERVICE_ACCOUNT_JSON} ${SSH_PUBLIC_KEY}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  ssh_user="ubuntu"
elif [[ "$PROVIDER" == "GCP" ]]; then
  # shellcheck disable=SC2016
  env SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_GCP_SERVICE_ACCOUT_KEY_JSON")" SSH_PUBLIC_KEY="$ssh_public_key" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${SERVICE_ACCOUNT_JSON} ${SSH_PUBLIC_KEY}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  ssh_user="user"
elif [[ "$PROVIDER" == "AWS" ]]; then
  # shellcheck disable=SC2016
  env AWS_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_ACCESS_KEY")" AWS_SECRET_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_SECRET_ACCESS_KEY")" \
      SSH_PUBLIC_KEY="$ssh_public_key" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${AWS_ACCESS_KEY} ${AWS_SECRET_ACCESS_KEY} ${SSH_PUBLIC_KEY}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  ssh_user="ubuntu"
elif [[ "$PROVIDER" == "Azure" ]]; then
  # shellcheck disable=SC2016
  env SUBSCRIPTION_ID="$LAYOUT_AZURE_SUBSCRIPTION_ID" CLIENT_ID="$LAYOUT_AZURE_CLIENT_ID" \
      CLIENT_SECRET="$LAYOUT_AZURE_CLIENT_SECRET"  TENANT_ID="$LAYOUT_AZURE_TENANT_ID" SSH_PUBLIC_KEY="$ssh_public_key" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${SSH_PUBLIC_KEY} ${TENANT_ID} ${CLIENT_SECRET} ${CLIENT_ID} ${SUBSCRIPTION_ID}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  ssh_user="azureuser"
elif [[ "$PROVIDER" == "OpenStack" ]]; then
  # shellcheck disable=SC2016
  env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" \
  SSH_PUBLIC_KEY="$ssh_public_key" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${SSH_PUBLIC_KEY} ${OS_PASSWORD}'  \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"
  ssh_user="ubuntu"
elif [[ "$PROVIDER" == "vSphere" ]]; then
  # shellcheck disable=SC2016
  env VSPHERE_PASSWORD="$(base64 -d <<<"$LAYOUT_VSPHERE_PASSWORD")" \
  SSH_PUBLIC_KEY="$ssh_public_key" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${SSH_PUBLIC_KEY} ${VSPHERE_PASSWORD}'  \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"
  ssh_user="ubuntu"
else
  >&2 echo "ERROR: Unknown provider \"$PROVIDER\""
  exit 1
fi

candictl bootstrap --yes-i-want-to-drop-cache --ssh-agent-private-keys "$ssh_private_key_path" --ssh-user "$ssh_user" \
--resources "$cwd/resources.yaml" --config "$cwd/configuration.yaml" | tee "$cwd/bootstrap.log"

# TODO: parse not the output of terraform, but last output of candictl
if ! master_ip="$(grep -Po '(?<=master_ip_address_for_ssh = ).+$' "$cwd/bootstrap.log")"; then
  >&2 echo "ERROR: can't parse master_ip from bootstrap.log, attempting to abort bootstrap"
  test_failed="true"
  exit 1
fi

>&2 echo 'Waiting 10 minutes until Machine provisioning finishes'
sleep 600

for ((i=0; i<3; i++)); do
  if ssh -o GlobalKnownHostsFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo -i /bin/bash <<"ENDSSH"; then
set -Eeuo pipefail

for ((i=0; i<10; i++)); do
  smoke_mini_addr=$(kubectl -n d8-upmeter get ep smoke-mini -o json | jq -re '.subsets[].addresses[0] | .ip') && break
  >&2 echo "Attempt to get Endpoints for smoki-mini #$i failed. Sleeping 30 seconds..."
  sleep 30
done

if [[ -z "$smoke_mini_addr" ]]; then
  >&2 echo "Couldn't get smoke-mini's address from Endpoints in 15 minutes."
  exit 1
fi

for ((i=0; i<10; i++)); do
  for path in api disk dns prometheus; do
    result="$(curl -m 5 -sS "${smoke_mini_addr}/${path}")"
    printf -v "$path" "%s" "$result"
  done

  cat <<EOF
Kubernetes API check: $([ "$api" == "ok" ] && echo "success" || echo "failure")
Disk check: $([ "$disk" == "ok" ] && echo "success" || echo "failure")
DNS check: $([ "$dns" == "ok" ] && echo "success" || echo "failure")
Prometheus check: $([ "$prometheus" == "ok" ] && echo "success" || echo "failure")
EOF
    if [[ "$api" == "ok" && "$disk" == "ok" && "$dns" == "ok" && "$prometheus" == "ok" ]]; then
      exit 0
  fi

  sleep 30
done

>&2 echo 'Timeout waiting for checks to succeed'
exit 1
ENDSSH
    test_failed=""
    break
  else
    test_failed="true"

    >&2 echo "SSH #$i failed. Sleeping 30 seconds..."
    sleep 30
  fi
done

exit 0
