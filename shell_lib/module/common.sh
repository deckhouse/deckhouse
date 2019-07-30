function module::path() {
  # /antiopa/modules/301-prometheus-metrics-adapter/hooks/superhook.sh -> /antiopa/modules/301-prometheus-metrics-adapter
  echo $0 | sed -r 's/^(\/antiopa\/modules\/\d+-[a-zA-Z0-9-]+)\/.+/\1/'
}

