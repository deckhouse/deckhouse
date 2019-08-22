#!/bin/bash -e

function module::name() {
  # /antiopa/modules/301-prometheus-metrics-adapter/hooks/superhook.sh -> prometheusMetricsAdapter
  echo $0 | sed -r 's/^\/antiopa\/modules\/\d+-([a-zA-Z0-9-]+)\/.+/\1/' | awk -F - '{printf "%s", $1; for(i=2; i<=NF; i++) printf "%s", toupper(substr($i,1,1)) substr($i,2); print"";}'
}

function module::path() {
  # /antiopa/modules/301-prometheus-metrics-adapter/hooks/superhook.sh -> /antiopa/modules/301-prometheus-metrics-adapter
  echo $0 | sed -r 's/^(\/antiopa\/modules\/\d+-[a-zA-Z0-9-]+)\/.+/\1/'
}
