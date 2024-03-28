#!/bin/bash

set -e

script=$(cat <<EOF
tree /spelling/pages
EOF
)

cp -R tools/spelling docs/site
cd docs/site/
werf run docs-spell-checker --dev --env development --docker-options="--entrypoint=bash" -- -c "$script"
rm -rf ./spelling