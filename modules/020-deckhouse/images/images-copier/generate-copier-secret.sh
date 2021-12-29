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

  Example:
    REPO_USER="aaaaa" REPO_PASSWORD='Pass"Word' IMAGE_WITH_TAG="q.com/deck" $0
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

if [[ "$image" =~ '/dev:' ]]; then
  echo -e "Image is incorrect. Address should not ending with /dev\n"
  print_help_and_exit 1
fi

conf="{\"username\":\"$usr\",\"password\":\"$pass\",\"insecure\":false,\"image\":\"$image\"}"
encoded="$(echo "$conf" | base64 -w 0)"

new_registry=$(echo "$image" | cut -d'/' -f 1)
new_auth=$(echo -n "$usr:$pass" | base64 -w 0)
new_registry="{\"auths\":{\"$new_registry\":{\"auth\":\"$new_auth\"}}}"

patch_auth="{\"data\":{\".dockerconfigjson\":\"$(echo -n "$new_registry" | base64 -w 0)\"}}"

echo -e " Use 'kubectl create -f' with next secret for run image copier in cluster
---
apiVersion: v1
kind: Secret
metadata:
  name: images-copier-config
  namespace: d8-system
data:
  dest-repo.json: $encoded
---
"

