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

set -Eeo pipefail

function check_jq() {
    if ! jq --version &>/dev/null; then
      >&2 echo "ERROR: jq is not installed. Please install it from https://stedolan.github.io/jq/download"
      return 1
    fi
}

function check_crane() {
    if ! crane version &>/dev/null; then
      >&2 echo "ERROR: crane is not installed. Please install it from https://github.com/google/go-containerregistry/tree/main/cmd/crane"
      return 1
    fi
}

function install_crane() {
    # Mac OS
    if brew --version >/dev/null; then
      brew install crane
    # Linux
    else
      curl curl -sSfL https://github.com/google/go-containerregistry/releases/download/v0.10.0/go-containerregistry_Linux_x86_64.tar.gz | tar -xzf - crane && mv crane /usr/local/bin && chmod +x /usr/local/bin/crane
    fi
}

function check_yq() {
    if ! yq --version &>/dev/null; then
      >&2 echo "ERROR: yq is not installed. Please install it from https://github.com/mikefarah/yq/releases"
      return 1
    fi

    if ! yq --version | grep -q ".*4\.[0-9]*.*"; then
      >&2 echo "ERROR: yq version should be equal 4"
      return 1
    fi
}

function install_yq() {
    # Mac OS
    if brew --version >/dev/null; then
      brew install yq
    # Linux
    else
      curl curl -sSfL https://github.com/mikefarah/yq/releases/download/v4.25.3/yq_linux_amd64 -o /usr/local/bin/yq && chmod +x /usr/local/bin/yq
    fi
}

function check_docker() {
    if ! docker version &>/dev/null; then
      >&2 echo "ERROR: docker is not installed"
      return 1
    fi
}
