#!/bin/bash

if ! type jq 2>/dev/null || ! type curl 2>/dev/null || ! type nc 2>/dev/null; then
  apt update
  export DEBIAN_FRONTEND=noninteractive
  until apt install jq nc curl -y; do
    echo "Error installing packages"
    apt update
    sleep 10
  done
fi

mkdir -p /var/lib/bashible/
