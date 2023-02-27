#!/bin/bash

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

set -Eeuo pipefail

readiness_file_path="/tmp/coredns-readiness"
ready_state="ready"
not_ready_state="not-ready"
latest_state=""

function add_rule() {
  if [[ ${CNI_CILIUM} == "yes" ]]; then
    if ! iptables -t nat -C PREROUTING -p tcp -m tcp -i lxc+ --dport 5353 -j DNAT --to-destination ${KUBE_DNS_SVC_IP}:53 >/dev/null 2>&1 ; then
      iptables -t nat -A PREROUTING -p tcp -m tcp -i lxc+ --dport 5353 -j DNAT --to-destination ${KUBE_DNS_SVC_IP}:53
    fi
    if ! iptables -t nat -C PREROUTING -p udp -m udp -i lxc+ --dport 5353 -j DNAT --to-destination ${KUBE_DNS_SVC_IP}:53 >/dev/null 2>&1 ; then
      iptables -t nat -A PREROUTING -p udp -m udp -i lxc+ --dport 5353 -j DNAT --to-destination ${KUBE_DNS_SVC_IP}:53
    fi
    return 0
  fi

  if ! iptables -w 60 -W 100000 -t raw -C PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
    iptables -w 60 -W 100000 -t raw -A PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK
  fi
}

function delete_rule() {
  if [[ ${CNI_CILIUM} == "yes" ]]; then
    if iptables -t nat -C PREROUTING -p tcp -m tcp -i lxc+ --dport 5353 -j DNAT --to-destination ${KUBE_DNS_SVC_IP}:53 >/dev/null 2>&1 ; then
      iptables -t nat -D PREROUTING -p tcp -m tcp -i lxc+ --dport 5353 -j DNAT --to-destination ${KUBE_DNS_SVC_IP}:53
    fi
    if iptables -t nat -C PREROUTING -p udp -m udp -i lxc+ --dport 5353 -j DNAT --to-destination ${KUBE_DNS_SVC_IP}:53 >/dev/null 2>&1 ; then
      iptables -t nat -D PREROUTING -p udp -m udp -i lxc+ --dport 5353 -j DNAT --to-destination ${KUBE_DNS_SVC_IP}:53
    fi
    return 0
  fi

  if iptables -w 60 -W 100000 -t raw -C PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK >/dev/null 2>&1 ; then
    iptables -w 60 -W 100000 -t raw -D PREROUTING -d "${KUBE_DNS_SVC_IP}/32" -m socket --nowildcard -j NOTRACK
  fi
}

function check_readiness() {
  readiness_file_state="$(< "$readiness_file_path")"
  case $readiness_file_state in
    "$ready_state")
      if [[ "$latest_state" != "$ready_state" ]]; then
        add_rule
        latest_state="$ready_state"
      fi
      ;;
    "$not_ready_state")
      if [[ "$latest_state" != "$not_ready_state" ]]; then
        delete_rule
        latest_state="$not_ready_state"
      fi
      ;;
    *)
      echo "Unknown state in file \"$readiness_file_path\": \"$readiness_file_state\""
      if [[ ${CNI_CILIUM} == "yes" ]]; then
        delete_rule
      fi
      exit 1
  esac
}

trap delete_rule INT TERM ERR

until [ -f "$readiness_file_path" ]; do
  echo "File \"$readiness_file_path\" does not exist yet. It should be created by a readinessProbe in the \"coredns\" container."
  delete_rule
  sleep 1
done

until [[ $(< "$readiness_file_path") == "$ready_state" ]]; do
  echo "\"coredns\" container is not ready yet"
  delete_rule
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
