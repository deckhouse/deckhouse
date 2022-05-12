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

help_arg="$1"

function print_help_and_exit() {
  cat <<HELP
Generate secret for running deckhouse images copier.
Pass all params via next env variables:
   IMAGE_WITH_TAG - destination image path with tag where copy deckhouse
                    WARNING it is not registry! It is full path to images repo
   REPO_USER      - destination registry auth username
   REPO_PASSWORD  - destination registry auth password

   INSECURE_REGISTRY - if is not empty, indicate that repository is not secure via TLS
   REGISTRY_CA - registry TLS root-certificate

  Example:
    REPO_USER="aaaaa" REPO_PASSWORD='Pass"Word' IMAGE_WITH_TAG="q.com/deckhouse/ee:1.28.1" $0

  Example repo without TLS:
    REPO_USER="aaaaa" REPO_PASSWORD='Pass"Word' IMAGE_WITH_TAG="q.com/deckhouse/ee:1.28.1" INSECURE_REGISTRY=true $0
HELP

   exit "$1"
}

if [ "$help_arg" == "-h" ] || [ "$help_arg" == "--help" ]; then
  print_help_and_exit 0
fi

usr="$(printenv REPO_USER)"
usr="${usr/\"/\\\"}"
pass="$(printenv REPO_PASSWORD)"
pass="${pass/\"/\\\"}"
image=$(printenv IMAGE_WITH_TAG)
image="${image/\"/\\\"}"
insecure_registry="$(printenv INSECURE_REGISTRY)"
registry_ca="$(printenv REGISTRY_CA)"

if [ -z "$usr" ]; then
  echo -e "Username is empty. Pass it via REPO_USER env var\n"
  print_help_and_exit 1
fi

if [ -z "$pass" ]; then
  echo -e "Password is empty. Pass it via REPO_PASSWORD env var\n"
  print_help_and_exit 1
fi

if [ -z "$image" ]; then
  echo -e "Image is empty. Pass it via IMAGE_WITH_TAG env var\n"
  print_help_and_exit 1
fi

schema="https"
insecure_conf="false"
if [ -n "$insecure_registry" ]; then
  schema="http"
  insecure_conf="true"
fi
schema_enc="$(echo -n "$schema" | base64 -w 0)"

image_copier_conf="{\"username\":\"$usr\",\"password\":\"$pass\",\"insecure\":$insecure_conf,\"image\":\"$image\"}"
image_copier_conf_enc="$(echo "$image_copier_conf" | base64 -w 0)"

registry=$(echo "$image" | cut -d'/' -f 1)
new_auth=$(echo -n "$usr:$pass" | base64 -w 0)
registry_auth="{\"auths\":{\"$registry\":{\"auth\":\"$new_auth\"}}}"
registry_auth_enc="$(echo -n "$registry_auth" | base64 -w 0)"

repo="${image%:*}"
repo_path="${repo#"$registry"}"
repo_path_enc="$(echo -n "$repo_path" | base64 -w 0)"
registry_enc="$(echo -n "$registry" | base64 -w 0)"

ca_patch=""
if [ -n "$registry_ca" ]; then
  ca_enc="$(echo -n "$registry_ca" | base64 -w 0)"
  ca_patch=",\"ca\":\"$ca_enc\""
fi

patch_auth='{"data":{".dockerconfigjson":"'$registry_auth_enc'","address":"'$registry_enc'","path":"'$repo_path_enc'","scheme":"'$schema_enc"\"$ca_patch}}"

patch_secret_cmd="kubectl -n d8-system patch secret deckhouse-registry --patch '$patch_auth'"
patch_deployment_cmd="kubectl -n d8-system patch deploy/deckhouse -p '{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"deckhouse\",\"image\":\"$image\"}]}}}}'"

echo -e "
# backup current secret
kubectl -n d8-system get secret deckhouse-registry -o yaml > backup-deckhouse-registry-secret.yaml

# Start copy images:
kubectl create -f - <<EndOfSecret
apiVersion: v1
kind: Secret
metadata:
  name: images-copier-config
  namespace: d8-system
data:
  dest-repo.json: $image_copier_conf_enc
EndOfSecret

# Wait for sync images. After finish sync images, run next commands:

# change deckhouse image
$patch_deployment_cmd

# patch d8-system/deckhouse-registry
$patch_secret_cmd

# check deckhouse pod
kubectl -n d8-system get po -l app=deckhouse

# if it is in ImagePullBackoff, restart it
kubectl -n d8-system rollout restart deployment deckhouse

# Wait for the Deckhouse Pod to become Ready.
# Wait for bashible to apply the new settings on the master node.
# The bashible log on the master node (journalctl -u bashible) should contain the message Configuration is in sync, nothing to do.
"

