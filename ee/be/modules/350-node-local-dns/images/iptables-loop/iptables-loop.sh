#!/bin/bash

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

set -Eeuo pipefail

readiness_file_path="/tmp/coredns-readiness"
ready_state="ready"
not_ready_state="not-ready"
latest_state=""

function add_rule() {
  if ! iptables -w 60 -W 100000 -t raw -C PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
    iptables -w 60 -W 100000 -t raw -A PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK
    echo "The state of the rule in iptables has changed to \"$latest_state\""
  fi
}

function delete_rule() {
  if iptables -w 60 -W 100000 -t raw -C PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
    iptables -w 60 -W 100000 -t raw -D PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK
    echo "The state of the rule in iptables has changed to \"$latest_state\""
  fi
}

function check_readiness() {
  if [[ $(< "$readiness_file_path") == "$ready_state" ]]; then
    if [[ "$latest_state" != "$ready_state" ]]; then
      add_rule
      latest_state="$ready_state"
    fi
  elif [[ $(< "$readiness_file_path") == "$not_ready_state" ]]; then
    if [[ "$latest_state" != "$not_ready_state" ]]; then
      delete_rule
      latest_state="$not_ready_state"
    fi
  else
    echo "Unknown state in file \"$readiness_file_path\": \"$(< "$readiness_file_path")\""
    exit 1
  fi
}

trap delete_rule INT TERM ERR

until [ -f "$readiness_file_path" ]; do
  echo "File \"$readiness_file_path\" does not exist yet. It should be created by a readinessProbe in the \"coredns\" container."
  sleep 1
done

until [[ $(< "$readiness_file_path") == "$ready_state" ]]; do
  echo "\"coredns\" container is not ready yet"
  sleep 1
done

pipe=$(mktemp -u)
mkfifo "$pipe"
exec 3<>"$pipe"
rm "$pipe"

echo "first run" >&3

inotifywait -q -m -e modify "$readiness_file_path" >&3 |
while read -r <&3; do
  check_readiness
done
