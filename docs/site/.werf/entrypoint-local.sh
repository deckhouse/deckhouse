#!/bin/sh
set -e

export CHANNELS_YAML_PATH="/app/channels.yaml"
export CHANNELS_CONF_PATH="/app/channels-data/channels.conf"

sh /scripts/channels-convert.sh

echo
echo "Starting nginx..."

exec nginx -g "daemon off;"
