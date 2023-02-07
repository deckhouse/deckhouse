#!/usr/bin/env bash

ISTIO_VER=${1:-1.16.2}

git clone --depth 1  --branch "${ISTIO_VER}" git@github.com:istio/istio.git 2>/dev/null

cp istio/manifests/addons/dashboards/*.json .

# Заменить `irate` на `rate`
sed 's/irate(/rate(/g' -i *.json
# Заменить `Resolution` на `1/1`
sed 's/"intervalFactor":\s[0-9]/"intervalFactor": 1/' -i *.json
# Убрать `Min Step`
sed '/"interval":/d' -i *.json
# Заменить все графики на `Staircase` (поломает графики `Stack` + `Percent`, которые придется поправить руками на `Bars`)
sed 's/"steppedLine": false/"steppedLine": true/' -i *.json
# Заменить все datasource на null
sed 's/"datasource": "Prometheus"/"datasource": null/' -i *.json

WORKLOADS_UID=$(cat workload.json| jq .uid -r)
SERVICES_UID=$(cat service.json| jq .uid -r)

# Изменить url на корректные
sed 's|/dashboard/db/istio-workload-dashboard|/d/'${WORKLOADS_UID}'/istio-workload-dashboard|g' -i *.json
sed 's|/dashboard/db/istio-service-dashboard|/d/'${SERVICES_UID}'/istio-service-dashboard|g' -i *.json

# Найти все range'и и заменить на `$__interval_sx4`:
for dashboard in *.json; do
  for range in $(grep '\[[0-9]\+[a-z]\]' $dashboard | sed 's/.*\(\[[0-9][a-z]\]\).*/\1/g' | tr -d "[]" | sort | uniq); do
    echo $dashboard $range
    sed  -i -e 's/\['${range}'\]/[$__interval_sx4]/g'  $dashboard
  done
done
