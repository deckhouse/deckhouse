#!/bin/bash -e

: "${GF_PATHS_CONFIG:=/etc/grafana/grafana.ini}"
: "${GF_PATHS_DATA:=/var/lib/grafana}"
: "${GF_PATHS_PLUGINS:=/var/lib/grafana/plugins}"
: "${GF_PATHS_PROVISIONING:=/etc/grafana/provisioning}"

IFS=","
for plugin in ${GF_CUSTOM_PLUGINS}; do
  grafana-cli --pluginsDir "${GF_PATHS_PLUGINS}" plugins install ${plugin};
done

exec /usr/sbin/grafana-server                           \
  --homepath=/usr/share/grafana                         \
  --config="$GF_PATHS_CONFIG"                           \
  cfg:default.log.mode="console"                        \
  cfg:default.paths.data="$GF_PATHS_DATA"               \
  cfg:default.paths.plugins="$GF_PATHS_PLUGINS"         \
  cfg:default.paths.provisioning=$GF_PATHS_PROVISIONING \
  "$@"
