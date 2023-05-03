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

# docker run --pull=always -it -v "$PWD/:/config:ro" -v "$PWD/dhctl-tmp:/tmp/dhctl" -v "$HOME/.ssh/:/home/user/.ssh/" -e "USER_ID=$(id -u ${USER})" -e "GROUP_ID=$(id -g ${USER})" -e "USER=${USER}" registry.deckhouse.io/deckhouse/ee/install:stable /deckhouse/candi/tools/user.sh

addgroup -g ${GROUP_ID} group_${GROUP_ID}
adduser -D ${USER} -h /home/user -u ${USER_ID} -G group_${GROUP_ID} -s /bin/bash
mkdir -p /home/user
chown ${USER_ID}:${GROUP_ID} /home/user
chown ${USER_ID}:${GROUP_ID} /tmp/dhctl
ln -s /etc/bashrc /home/user/.bashrc
su ${USER}
