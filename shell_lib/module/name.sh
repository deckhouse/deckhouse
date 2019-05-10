#!/bin/bash -e

function module::name() {
  # /antiopa/modules/301-prometheus-metrics-adapter/hooks/superhook.sh -> prometheus-metrics-adapter
  echo $0 | sed -r 's/^\/antiopa\/modules\/\d+-([a-zA-Z0-9-]+)\/.+/\1/'
}
