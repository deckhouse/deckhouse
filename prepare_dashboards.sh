#!/bin/bash

listOfDomains="
flant.com
example.com
"

listOfDashboards=$(find modules/300-prometheus/grafana-dashboards -name "*json")

for dashboard in $listOfDashboards; do
  for domain in $listOfDomains; do
    sed -i -E  "s/([^\"]+$domain)/example.com/g" $dashboard
  done
done
