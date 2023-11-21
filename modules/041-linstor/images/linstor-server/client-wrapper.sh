#!/usr/bin/bash

# Copyright 2023 Flant JSC
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

# only allowed subcommands of linstor client
valid_subcommands_list=("storage-pool" "sp" "node" "n" "resource" "r" "volume" "v" "resource-definition" "rd")
valid_subcommands_ver=("controller" "c")
valid_subcommands_lv=("resource" "r")
allowed=false

# check for allowed linstor ... l and linstor ... list commands
if [[ $(echo "${valid_subcommands_list[@]}" | fgrep -w $1) ]]; then
  if [[ $2 == "l" || $2 == "list" ]]; then
    allowed=true
  fi
fi


# check for allowed linstor ... v and linstor ... version commands
if [[ $(echo "${valid_subcommands_ver[@]}" | fgrep -w $1) ]]; then
  if [[ $2 == "v" || $2 == "version" ]]; then
    allowed=true
  fi
fi

# check for allowed linstor ... lv commands
if [[ $(echo "${valid_subcommands_lv[@]}" | fgrep -w $1) ]]; then
  if [[ $2 == "lv" ]]; then
    allowed=true
  fi
fi

if [[ $allowed == true ]]; then
  /usr/bin/originallinstor "$@"
else
  echo "You're not allowed to change state of linstor cluster manually. Please contact tech support"
  exit 1
fi
