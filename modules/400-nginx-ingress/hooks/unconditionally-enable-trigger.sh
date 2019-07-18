#!/bin/bash

### Миграция 2019-07-18: https://github.com/deckhouse/deckhouse/merge_requests/927
###
### Этот хук можно удалить после начала второй фазы миграции nginx-ingress rewrite-target: https://github.com/deckhouse/deckhouse/issues/641

source /antiopa/shell_lib.sh

function __config__() {
  echo '
{
  "beforeHelm": 5
}'
}

function __main__() {
  values::set --config nginxIngress.rewriteTargetMigration "true"
}

hook::run "$@"
