#!/bin/bash

if ! type jq || ! type curl; then
  apt update
  export DEBIAN_FRONTEND=noninteractive
  until apt install jq curl -y; do
    echo "Error installing packages"
    sleep 10
  done
fi
