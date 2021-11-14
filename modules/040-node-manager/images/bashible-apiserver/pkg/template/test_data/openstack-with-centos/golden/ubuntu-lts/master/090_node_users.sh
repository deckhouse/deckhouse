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

# if reboot flag set due to disruption update (for example, in case of CRI change) we pass this step.
# this step runs normally after node reboot.
if bb-flag? disruption && bb-flag? reboot; then
  exit 0
fi

function get_node_users_secret() {
  local secret="node-users"
  local attempt=0
  local max_attempts=5
  local kubeconfig=""

  if [ -f /etc/kubernetes/kubelet.conf ]; then
    kubeconfig="/etc/kubernetes/kubelet.conf"
  elif [ -f /etc/kubernetes/bootstrap-kubelet.conf ]; then
    kubeconfig="/etc/kubernetes/bootstrap-kubelet.conf"
  else
    bb-log-error "Can't find /etc/kubernetes/kubelet.conf nor /etc/kubernetes/bootstrap-kubelet.conf"
    exit 1
  fi

  until bb-kubectl --kubeconfig $kubeconfig get secrets -n d8-cloud-instance-manager $secret -o json; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "$max_attempts" ]; then
      bb-log-error "failed to get secret $secret after $max_attempts attempts"
      exit 1
    fi
    echo "Waiting for get secret $secret (attempt $attempt of $max_attempts)..."
    sleep 5
  done
}

# $1 - userName, $2 - basePath, $3 - mainGroup, $4 - sshKey
function put_user_ssh_key() {
  local userName="$1"
  local basePath="$2"
  local mainGroup="$3"
  local sshKey="$4"
  local sshDir="$basePath/$userName/.ssh"
  mkdir -p $sshDir
  echo "$sshKey" > $sshDir/authorized_keys
  chown -R $userName:$mainGroup $sshDir
  chmod 700 $sshDir
  chmod 600 $sshDir/authorized_keys
}
sudoGroup="sudo"
mainGroup="100" # users
homeBasePath="/home/deckhouse"
defaultShell="/bin/bash"
comment="created by deckhouse"

secretJson="$(get_node_users_secret)"

if [ -z "$secretJson" ]; then
  exit 0
fi

mkdir -p $homeBasePath

nodeUsersJSON="$(jq -r '.data."node-users.json"' <<< "$secretJson" | base64 -d)"

for uid in $(jq -rc '.[].spec.uid' <<< "$nodeUsersJSON"); do
  userName="$(jq --arg uid $uid -rc '.[] | select(.spec.uid==($uid | tonumber)) | .name' <<< "$nodeUsersJSON")"
  passwordHash="$(jq --arg uid $uid -rc '.[] | select(.spec.uid==($uid | tonumber)) | .spec.passwordHash' <<< "$nodeUsersJSON")"
  sshPublicKey="$(jq --arg uid $uid -rc '.[] | select(.spec.uid==($uid | tonumber)) | .spec.sshPublicKey' <<< "$nodeUsersJSON")"
  extraGroups="$(jq --arg uid "$uid" --arg sudo_group "$sudoGroup" -rc '.[] | select(.spec.uid==($uid | tonumber)) | [.spec.extraGroups[]?] + (if .spec.isSudoer then [$sudo_group] else [] end) | join(",")' <<< "$nodeUsersJSON")"

  # check for uid > 1000
  if [ $uid -le 1000 ]; then
    bb-log-error "Uid for user $userName must be > 1000"
    exit 1
  fi

  # Check user existence
  if id $uid 1>/dev/null 2>/dev/null; then
    userInfo="$(getent passwd $uid)"
    # check comment
    if [[ "$(cut -d ":" -f5 <<< "$userInfo")" != "$comment" ]]; then
      bb-log-error "User with UID $uid was created before by someone else"
      exit 1
    fi
    # check username
    if [[ "$(cut -d ":" -f1 <<< "$userInfo")" != "$userName" ]]; then
      bb-log-error "Username of user with UID $uid was changed by someone else"
      exit 1
    fi
    # check mainGroup
    if [[ "$(cut -d ":" -f4 <<< "$userInfo")" != "$mainGroup" ]]; then
      bb-log-error "User GID of user with UID $uid was changed by someone else"
      exit 1
    fi
    # check homeDir
    if [[ "$(cut -d ":" -f6 <<< "$userInfo")" != "$homeBasePath/$userName" ]]; then
      bb-log-error "User home dir of user with UID $uid was changed by someone else"
      exit 1
    fi
    # All ok, modify user
    usermod -G "$extraGroups" -p "$passwordHash" "$userName"
    put_user_ssh_key "$userName" "$homeBasePath" "$mainGroup" "$sshPublicKey"
  else
    # Adding user
    useradd -b "$homeBasePath" -g "$mainGroup" -G "$extraGroups" -p "$passwordHash" -s "$defaultShell" -u "$uid" -c "$comment" -m "$userName"
    put_user_ssh_key "$userName" "$homeBasePath" "$mainGroup" "$sshPublicKey"
  fi
done

# Remove users which exist locally but does not exist in secret
localUsersUIDs="$(getent passwd | grep "$comment" | cut -d ":" -f3 || true)"
secretUsersUIDs="$(jq -r '.[].spec.uid' <<< "$nodeUsersJSON")"
for localUserID in $localUsersUIDs; do
  isUserIDFound="false"
  for secretUserID in $secretUsersUIDs; do
    if [ "$localUserID" -eq "$secretUserID" ]; then
      isUserIDFound="true"
      break
    fi
  done
  if [[ "$isUserIDFound" == "false" ]]; then
    if [ "$localUserID" -gt "1000" ]; then
      userdel -r "$(id -nu $localUserID)"
    else
      bb-log-error "Strange user with UID: $localUserID, cannot delete it"
      exit 1
    fi
  fi
done
