#!/usr/bin/env bash

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

ISTIO_VER=${1:-1.19.7}

git clone --depth 1  --branch "${ISTIO_VER}" git@github.com:istio/istio.git 2>/dev/null

cp istio/manifests/addons/dashboards/*.json .

for i in $( ls *.json ); do mv $i ${i/-dashboard/}; done
for i in $( ls *.json ); do mv $i ${i/istio-/}; done

# Replace `irate` to `rate`
sed -i '' -e 's/irate(/rate(/g' *.json
# Replace `Resolution` to `1/1`
sed -i '' -e 's/"intervalFactor": [0-9]/"intervalFactor": 1/' *.json
# Remove `Min Step`
sed -i '' -e '/"interval":/d' *.json
# Replace `Staircase` graphs
sed -i '' -e 's/"steppedLine": false/"steppedLine": true/' *.json
# Replace all datasource to `null`
sed -i '' -e 's/"datasource": "Prometheus"/"datasource": null/' *.json

WORKLOADS_UID=$(cat workload.json| jq .uid -r)
SERVICES_UID=$(cat service.json| jq .uid -r)

# Fix dashboard urls
sed -i '' -e 's|/dashboard/db/istio-workload-dashboard|/d/'${WORKLOADS_UID}'/istio-workload-dashboard|g' *.json
sed -i '' -e 's|/dashboard/db/istio-service-dashboard|/d/'${SERVICES_UID}'/istio-service-dashboard|g' *.json

# Find all ranges and replace them to `$__interval_sx4`:
for dashboard in *.json; do
  for range in $(grep '\[[0-9]\+[a-z]\]' $dashboard | sed 's/.*\(\[[0-9][a-z]\]\).*/\1/g' | tr -d "[]" | sort | uniq); do
    echo $dashboard $range
    sed -i '' -e 's/\['${range}'\]/[$__interval_sx4]/g' $dashboard
  done
done
