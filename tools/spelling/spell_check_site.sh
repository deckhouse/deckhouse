#!/bin/bash

set -e

if [ -n "$1" ]; then
  arg_target_page=$1
fi

if [ -n "$2" ]; then
  arg_get_plain_text=$2
fi

script=$(cat <<EOF
cd /spelling && \
  /temp/internal/container_spell_check.sh $arg_target_page $arg_get_plain_text
EOF
)

cd docs/site/
werf run docs-spell-checker --dev --env development  --docker-options="--entrypoint=sh" -- -c "$script"
