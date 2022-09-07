#!/usr/bin/env bash
set -euo pipefail

available_modules="$(ls /available_hooks)"
for module in $ENABLED_MODULES; do
 module_dir=$(grep -E "^[0-9]+-$module" <<< "$available_modules" || true)
 if [[ -n "$module_dir" ]]; then
   cp -r /available_hooks/"$module_dir" /hooks
 fi
done

exec /sbin/tini -- /shell-operator "$@"
