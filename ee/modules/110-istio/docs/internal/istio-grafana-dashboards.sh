#!/usr/bin/env bash

# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

ISTIO_VER=${1:-1.16.2}

git clone --depth 1  --branch "${ISTIO_VER}" git@github.com:istio/istio.git 2>/dev/null

cp istio/manifests/addons/dashboards/*.json .

# Replace `irate` to `rate`
sed 's/irate(/rate(/g' -i *.json
# Replace `Resolution` to `1/1`
sed 's/"intervalFactor":\s[0-9]/"intervalFactor": 1/' -i *.json
# Remove `Min Step`
sed '/"interval":/d' -i *.json
# Replace `Staircase` graphs
sed 's/"steppedLine": false/"steppedLine": true/' -i *.json
# Replace all datasource to `null`
sed 's/"datasource": "Prometheus"/"datasource": null/' -i *.json

WORKLOADS_UID=$(cat workload.json| jq .uid -r)
SERVICES_UID=$(cat service.json| jq .uid -r)

# Fix dashboard urls
sed 's|/dashboard/db/istio-workload-dashboard|/d/'${WORKLOADS_UID}'/istio-workload-dashboard|g' -i *.json
sed 's|/dashboard/db/istio-service-dashboard|/d/'${SERVICES_UID}'/istio-service-dashboard|g' -i *.json

# Find all ranges and replace them to `$__interval_sx4`:
for dashboard in *.json; do
  for range in $(grep '\[[0-9]\+[a-z]\]' $dashboard | sed 's/.*\(\[[0-9][a-z]\]\).*/\1/g' | tr -d "[]" | sort | uniq); do
    echo $dashboard $range
    sed  -i -e 's/\['${range}'\]/[$__interval_sx4]/g'  $dashboard
  done
done
