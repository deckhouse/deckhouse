#!/bin/bash

# Copyright 2024 Flant JSC
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

# Usage: $0 <BASE_IMAGE_NAME(optional)>
# if run without arguments, creates dev images all base images
set -Eeo pipefail

VERSION_FILE="../../candi/image_versions.yml"
READ_REGISTRY="$(grep "^REGISTRY_PATH" ${VERSION_FILE} | sed "s/\"//g" | awk '{print $2}')"
WRITE_REGISTRY="registry-write.deckhouse.io/base_images/"

to_add=""

list=$(grep -E "^BASE_" ${VERSION_FILE} | grep -vE "_DEV:")

if [ -n "$1" ]; then
  list=$(grep "$1" <<< "$list")
fi

while IFS=: read -r var_name image_with_hash
do
  image_name="$(sed "s/\"//g" <<< "${image_with_hash}" | sed "s/ //g")"
  new_image_name="dev-$(sed "s/\"//g" <<< "${image_with_hash}" | sed "s/ //g" | cut -d "@" -f1)"

  if [[ ! -f Dockerfiles/Dockerfile.${var_name} ]]; then
      echo "There's no need to add corresponding dev image for ${var_name}"
      continue
  fi
  cp -p Dockerfiles/Dockerfile.${var_name} Dockerfile
  docker build --platform linux/amd64 --build-arg DISTRO=${READ_REGISTRY}${image_name} -t ${WRITE_REGISTRY}${new_image_name} .
  docker push ${WRITE_REGISTRY}${new_image_name}
  image_id="$(docker image inspect --format '{{index .RepoDigests 0}}' ${WRITE_REGISTRY}${new_image_name} | cut -d "@" -f2)"
  to_add="${to_add}${var_name}_DEV: \"${new_image_name}@${image_id}\"\n"
  docker rmi ${WRITE_REGISTRY}${new_image_name}
  rm -f Dockerfile
done <<< "${list}"

if [[ -n ${to_add} ]]; then
  echo "Please add following lines to ${VERSION_FILE}"
  echo
  echo -e ${to_add}
fi
