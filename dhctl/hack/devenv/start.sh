#!/bin/sh

# Copyright 2022 Flant JSC
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

set -Eeuo pipefail
shopt -s failglob

INSTALLER_IMAGE_URL=dev-registry.deckhouse.io/sys/deckhouse-oss/dev/install:main
DEV_CONTAINER_NAME=dhctl-dev

echo -e "\n#1 Run Docker Container\n==="
id=$(docker ps -aqf "name=$DEV_CONTAINER_NAME")
if [[ "x$id" == "x" ]]; then
  id=$(docker run \
     --pull=always \
     --name "${DEV_CONTAINER_NAME}" \
     --detach \
     --rm \
     --mount type=tmpfs,destination=/tmp:exec \
     -v $HOME/.ssh/:/root/.ssh/ \
     -v $(pwd)/bin:/test-bin \
     -v $(pwd)/.state/:/.state/ \
     -v $(pwd)/../../dhctl:/dhctl \
     -v $(pwd)/../../candi:/deckhouse/candi \
     -v $(pwd)/../ee/candi/cloud-providers/openstack:/deckhouse/candi/cloud-providers/openstack \
     -v $(pwd)/../ee/candi/cloud-providers/vsphere:/deckhouse/candi/cloud-providers/vsphere \
     ${INSTALLER_IMAGE_URL} \
     tail -f /dev/null)
  echo "Run new container with ID: ${id}"
else
  echo "Container found: ${id}"
fi

echo -e "\n#2 Exec into Docker Container\n==="
docker exec -it ${id} bash
