#!/bin/bash

# Copyright 2025 Flant JSC
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

generate_hash() {
    local password=$1
    docker run --rm -e PASSWORD="$password" python:3-alpine \
    sh -c "pip install bcrypt > /dev/null && python -c 'import bcrypt, os; print(bcrypt.hashpw(os.environ[\"PASSWORD\"].encode(), bcrypt.gensalt(rounds=10)).decode())'"
}

MIRROR_HASH=$(generate_hash "$1")
CLUSTER_HASH=$(generate_hash "$2")

cat <<EOF
server:
  addr: ":5001"
token:
  certificate: "/ssl/server.pem"
  key: "/ssl/server.key"
  issuer: "auth-service"
  expiration: 900
users:
  mirror:
    password: |
        ${MIRROR_HASH}
  cluster:
    password: |
        ${CLUSTER_HASH}
acl:
  - match: {account: "mirror"}
    actions: ["*"]
  - match: {account: "cluster"}
    actions: ["pull"]
EOF
