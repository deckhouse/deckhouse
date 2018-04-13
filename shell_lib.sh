#!/bin/bash

set -Eeuo pipefail

for f in $(find /antiopa/shell_lib/ -type f -iname "*.sh"); do
  source $f
done
