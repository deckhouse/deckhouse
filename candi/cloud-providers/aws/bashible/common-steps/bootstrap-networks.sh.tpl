#!/bin/bash
{{- /*
# Copyright 2021 Flant CJSC
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
  curl -L -o /usr/local/bin/ec2_describe_tags https://github.com/flant/go-ec2-describe-tags/releases/download/v0.0.1-flant.1/ec2_describe_tags
  chmod +x /usr/local/bin/ec2_describe_tags
  instance_name=$(/usr/local/bin/ec2_describe_tags -query_meta | grep -Po '(?<=Name=).+')
  hostnamectl set-hostname "$instance_name"
  rm /usr/local/bin/ec2_describe_tags
  mkdir -p /var/lib/bashible
  touch /var/lib/bashible/hosname-set-as-in-aws
fi
