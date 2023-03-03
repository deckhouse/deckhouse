#!/bin/bash

set -Eeuo pipefail
shopt -s failglob

unameOut="$(uname -s)"
case "${unameOut}" in
    Darwin*)    shopt -s inherit_errexit 2>/dev/null || true;; #ignore on MacOS
    *)          shopt -s inherit_errexit;;
esac

backtrace() {
  local ret=$?
  local i=0
  local frames=${#BASH_SOURCE[@]}

  echo >&2 "Traceback (most recent call last):"

  for ((frame=frames-2; frame >= 0; frame--)); do
    local lineno=${BASH_LINENO[frame]}

    printf >&2 '  File "%s", line %d, in %s\n' \
        "${BASH_SOURCE[frame+1]}" "$lineno" "${FUNCNAME[frame+1]}"

    sed >&2 -n "${lineno}s/^[   ]*/    /p" "${BASH_SOURCE[frame+1]}"
  done

  printf >&2 "Exiting with status %d\n" "$ret"
}

trap backtrace ERR

for f in $(find /deckhouse/shell-operator/frameworks/shell/ -type f -iname "*.sh"; find /deckhouse/shell_lib/ -type f -iname "*.sh"); do
  source $f
done
