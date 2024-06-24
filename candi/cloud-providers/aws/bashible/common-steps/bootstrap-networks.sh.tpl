#!/bin/bash
{{- /*
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
*/}}

if [ ! -f /var/lib/bashible/hosname-set-as-in-aws ]; then
  attempt=0
	fail_describe_tags=0
  until [[ $(/opt/deckhouse/bin/ec2_describe_tags -query_meta) ]]; do 
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "2" ]; then
      fail_describe_tags=1
      break
    fi
    >&2 echo "ec2_describe_tags return empty"
    sleep 2
  done

  if [[ $fail_describe_tags -eq 1 ]]; then
    >&2 echo "Failed to define hostname instance. Number of attempts exceeded."
    exit 1
  fi
  instance_name=$(/opt/deckhouse/bin/ec2_describe_tags -query_meta | grep -Po '(?<=Name=).+')
  hostnamectl set-hostname "$instance_name"
  mkdir -p /var/lib/bashible
  touch /var/lib/bashible/hosname-set-as-in-aws
fi
