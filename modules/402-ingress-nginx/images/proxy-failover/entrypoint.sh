#!/usr/bin/env sh
set -eu

envsubst '${CONTROLLER_NAME}' < /etc/nginx/nginx.conf.tpl > /etc/nginx/nginx.conf

exec "$@"
