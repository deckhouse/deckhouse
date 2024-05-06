#!/bin/bash

set -e

werf run docs-spell-checker --dev --docker-options="--entrypoint=sh" -- /app/internal/container_spell_check.sh $@
