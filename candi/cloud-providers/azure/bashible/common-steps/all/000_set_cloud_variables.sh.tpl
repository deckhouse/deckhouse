#!/bin/bash

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

shutdown_grace_period="0s"
shutdown_grace_period_critical_pods="0s"

if d8-curl -sS -H Metadata:true --noproxy "*" "http://169.254.169.254/metadata/instance?api-version=2021-12-13" | grep -q  '"priority":"Regular"'; then
  shutdown_grace_period="5m"
  shutdown_grace_period_critical_pods="5s"
fi

cat << EOF > /var/lib/bashible/cloud-provider-variables
shutdown_grace_period="$shutdown_grace_period"
shutdown_grace_period_critical_pods="$shutdown_grace_period_critical_pods"
EOF
