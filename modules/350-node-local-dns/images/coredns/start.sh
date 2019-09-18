#!/bin/bash

START_FILE=/shared/starting
CONFIG_FILE=/etc/coredns/Corefile

touch ${START_FILE}

while true; do
  if [[ -f "${START_FILE}" ]] ; then
    echo "Waiting for remove ${START_FILE} file"
    sleep 1
  else
    if [[ -f "${CONFIG_FILE}" ]]; then
      exec /coredns -conf ${CONFIG_FILE}
    else
      echo "Waiting for coredns config file"
      sleep 1
    fi
  fi
done
