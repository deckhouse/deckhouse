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

{{- if eq .runType "Normal" }}
  {{- if .nodeUsers }}
node_users_json='{{ .nodeUsers | toJson}}'
  {{- end }}

# if reboot flag set due to disruption update (for example, in case of CRI change) we pass this step.
# this step runs normally after node reboot.
if bb-flag? disruption && bb-flag? reboot; then
  exit 0
fi

function error_check() {
  error_code=$?
  (( error_code != 0 ))
}

function kubectl_exec() {
  kubectl --request-timeout 60s --kubeconfig=/etc/kubernetes/kubelet.conf patch nodeusers.deckhouse.io "${1}" "${2}" "${3}" "${4}"
}

# $1 - username $2 - request data
function nodeuser_patch() {
  local username="$1"
  local data="$2"

  # Skip this step after multiple failures.
  # This step puts information "how to get bootstrap logs" into Instance resource.
  # It's not critical, and waiting for it indefinitely, breaking bootstrap, is not reasonable.
  local failure_count=0
  local failure_limit=3

  if type kubectl >/dev/null 2>&1 && test -f /etc/kubernetes/kubelet.conf ; then
    until kubectl_exec "${username}" --type=json --patch="${data}" --subresource=status; do
      failure_count=$((failure_count + 1))
      if [[ $failure_count -eq $failure_limit ]]; then
        bb-log-error "ERROR: Failed to patch NodeUser with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
        break
      fi
      bb-log-error "failed to NodeUser with kubectl --kubeconfig=/etc/kubernetes/kubelet.conf"
      sleep 10
    done
  elif [ -f /var/lib/bashible/bootstrap-token ]; then
    local patch_pending=true
    while [ "$patch_pending" = true ] ; do
      for server in {{ .normal.apiserverEndpoints | join " " }} ; do
        local server_addr=$(echo $server | cut -f1 -d":")
        until local tcp_endpoint="$(ip ro get ${server_addr} | grep -Po '(?<=src )([0-9\.]+)')"; do
          bb-log-info "The network is not ready for connecting to apiserver yet, waiting..."
          sleep 1
        done

        if curl -sS --fail -x "" \
          --max-time 10 \
          -XPATCH \
          -H "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" \
          -H "Accept: application/json" \
          -H "Content-Type: application/json-patch+json" \
          --cacert "$BOOTSTRAP_DIR/ca.crt" \
          --data "${data}" \
          "https://$server/apis/deckhouse.io/v1/nodeusers/${username}/status" ; then

          echo "Successfully patched NodeUser."
          patch_pending=false

          break
        else
          failure_count=$((failure_count + 1))

          if [[ $failure_count -eq $failure_limit ]]; then
            >&2 echo "Failed to patch NodeUser. Number of attempts exceeded. NodeUser patch will be skipped."
            patch_pending=false
            break
          fi

          >&2 echo "Failed to patch NodeUser. ${failure_count} of ${failure_limit} attempts..."
          sleep 10
          continue
        fi
      done
    done
  else
    >&2 echo "failed to patch NodeUser can't find kubelet.conf or bootstrap-token"
    exit 1
  fi
}

# $1 - username $2 - error message
function nodeuser_add_error() {
  local username="$1"
  local message="$2"
  local machine_name="${D8_NODE_HOSTNAME}"
  if [ -f ${BOOTSTRAP_DIR}/machine-name ]; then
    local machine_name="$(<${BOOTSTRAP_DIR}/machine-name)"
  fi

  nodeuser_patch "${username}" "[{\"op\":\"add\",\"path\":\"/status/errors/${machine_name}\",\"value\":\"${message}\"}]"
}

# $1 - username
function nodeuser_clear_error() {
  local username="$1"
  local machine_name="${D8_NODE_HOSTNAME}"
  if [ -f ${BOOTSTRAP_DIR}/machine-name ]; then
    local machine_name="$(<${BOOTSTRAP_DIR}/machine-name)"
  fi

  nodeuser_patch "${username}" "[{\"op\":\"remove\",\"path\":\"/status/errors/${machine_name}\"}]"

}

# $1 - user_name, $2 - extra_groups, $3 - password_hash
function modify_user() {
  local user_name="$1"
  local extra_groups="$2"
  local password_hash="$3"

  usermod -G "$extra_groups" "$user_name"

  local current_hash="$(getent shadow "$user_name" | awk -F ":" '{print $2}')"
  if [ "$password_hash" != "$current_hash" ]; then
    usermod -p "$password_hash" "$user_name"
    echo "Password hash was updated for user '$user_name'"
  fi
}

# $1 - user_name, $2 - base_path, $3 - main_group, $4 - ssh_keys
function put_user_ssh_key() {
  local user_name="$1"
  local base_path="$2"
  local main_group="$3"
  local ssh_keys="$4"
  local ssh_dir="$base_path/$user_name/.ssh"
  local ssh_new_keys="$(sed "s/\;/\n/g" <<< "$ssh_keys" | sort -u)"

  local ssh_curent_keys=""
  if [[ -f "$ssh_dir/authorized_keys" ]]; then
    local ssh_curent_keys="$(cat $ssh_dir/authorized_keys)"
  fi

  if [[ "${ssh_curent_keys}" != "${ssh_new_keys}" ]]; then
    mkdir -p "$ssh_dir"
    echo -n "$ssh_new_keys" > "$ssh_dir/authorized_keys"
    chown -R "$user_name:$main_group" "$ssh_dir"
    chmod 700 "$ssh_dir"
    chmod 600 "$ssh_dir/authorized_keys"
  fi
}

function main() {
  if getent group sudo >/dev/null; then
    sudo_group="sudo"
  elif getent group wheel >/dev/null; then
    sudo_group="wheel"
  else
    bb-log-error "Cannot find sudo group"
    nodeuser_add_error "${user_name}" "Cannot find sudo group"
    exit 0
  fi

  main_group="100" # users
  home_base_path="/home/deckhouse"
  default_shell="/bin/bash"
  comment="created by deckhouse"

  mkdir -p $home_base_path

  for uid in $(jq -rc '.[].spec.uid' <<< "$node_users_json"); do
    user_name="$(jq --arg uid $uid -rc '.[] | select(.spec.uid==($uid | tonumber)) | .name' <<< "$node_users_json")"
    password_hash="$(jq --arg uid $uid -rc '.[] | select(.spec.uid==($uid | tonumber)) | .spec.passwordHash' <<< "$node_users_json")"
    ssh_public_keys="$(jq --arg uid $uid -rc '.[] | select(.spec.uid==($uid | tonumber)) | [.spec.sshPublicKeys[]?] + (if .spec.sshPublicKey then [.spec.sshPublicKey] else [] end) | join(";")' <<< "$node_users_json")"
    extra_groups="$(jq --arg uid "$uid" --arg sudo_group "$sudo_group" -rc '.[] | select(.spec.uid==($uid | tonumber)) | [.spec.extraGroups[]?] + (if .spec.isSudoer then [$sudo_group] else [] end) | join(",")' <<< "$node_users_json")"

    nodeuser_add_error "${user_name}" "None"

    # check for uid > 1000
    if [ $uid -le 1000 ]; then
      bb-log-error "Uid for user $user_name must be > 1000"
      nodeuser_add_error "${user_name}" "Uid for user $user_name must be > 1000"
      exit 0
    fi

    # Check user existence
    if id $uid 1>/dev/null 2>/dev/null; then
      user_info="$(getent passwd $uid)"
      # check comment
      if [[ "$(cut -d ":" -f5 <<< "$user_info")" != "$comment" ]]; then
        bb-log-error "User with UID $uid was created before by someone else"
        nodeuser_add_error "${user_name}" "User with UID $uid was created before by someone else"
        exit 0
      fi
      # check username
      if [[ "$(cut -d ":" -f1 <<< "$user_info")" != "$user_name" ]]; then
        bb-log-error "Username of user with UID $uid was changed by someone else"
        nodeuser_add_error "${user_name}" "Username of user with UID $uid was changed by someone else"
        exit 0
      fi
      # check mainGroup
      if [[ "$(cut -d ":" -f4 <<< "$user_info")" != "$main_group" ]]; then
        bb-log-error "User GID of user with UID $uid was changed by someone else"
        nodeuser_add_error "${user_name}" "User GID of user with UID $uid was changed by someone else"
        exit 0
      fi
      # check homeDir
      if [[ "$(cut -d ":" -f6 <<< "$user_info")" != "$home_base_path/$user_name" ]]; then
        bb-log-error "User home dir of user with UID $uid was changed by someone else"
        nodeuser_add_error "${user_name}" "User home dir of user with UID $uid was changed by someone else"
        exit 0
      fi
      # All ok, modify user
      error_message=$(modify_user "$user_name" "$extra_groups" "$password_hash" 2>&1)
      if error_check
      then
        nodeuser_add_error "${user_name}" "${error_message}"
        exit 0
      fi
      error_message=$(put_user_ssh_key "$user_name" "$home_base_path" "$main_group" "$ssh_public_keys" 2>&1)
      if error_check
      then
        nodeuser_add_error "${user_name}" "${error_message}"
        exit 0
      fi
    else
      # Adding user
      error_message=$(useradd -b "$home_base_path" -g "$main_group" -G "$extra_groups" -p "$password_hash" -s "$default_shell" -u "$uid" -c "$comment" -m "$user_name" 2>&1)
      if error_check
      then
        nodeuser_add_error "${user_name}" "${error_message}"
        exit 0
      fi
      error_message=$(put_user_ssh_key "$user_name" "$home_base_path" "$main_group" "$ssh_public_keys" 2>&1)
      if error_check
      then
        nodeuser_add_error "${user_name}" "${error_message}"
        exit 0
      fi
    fi
    nodeuser_clear_error "${user_name}"
  done

  # Remove users which exist locally but does not exist in secret
  local_users_uids="$(getent passwd | grep "$comment" | cut -d ":" -f3 || true)"
  secret_users_uids="$(jq -r '.[].spec.uid' <<< "$node_users_json")"
  for local_user_id in $local_users_uids; do
    is_user_id_found="false"
    for secret_user_id in $secret_users_uids; do
      if [ "$local_user_id" -eq "$secret_user_id" ]; then
        is_user_id_found="true"
        break
      fi
    done
    if [[ "$is_user_id_found" == "false" ]]; then
      if [ "$local_user_id" -gt "1000" ]; then
        userdel -r "$(id -nu $local_user_id)"
      else
        bb-log-error "Strange user with UID: $local_user_id, cannot delete it"
        exit 0
      fi
    fi
  done
}

set +e
main
set -e
{{- end  }}

