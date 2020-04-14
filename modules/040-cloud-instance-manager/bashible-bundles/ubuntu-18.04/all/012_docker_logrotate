#!/bin/bash

if [[ -f "/etc/logrotate.d/docker-containers" ]]; then
  rm -f /etc/logrotate.d/docker-containers
fi


if [[ -f "/etc/systemd/system/docker-logrotate.service" ]]; then
  systemctl stop docker-logrotate.service
  systemctl disable docker-logrotate.service
  rm -f /etc/systemd/system/docker-logrotate.service
  systemctl daemon-reload
  systemctl reset-failed
fi

if [[ -f "/etc/systemd/system/docker-logrotate.timer" ]]; then
  systemctl stop docker-logrotate.timer
  systemctl disable docker-logrotate.timer
  rm -f /etc/systemd/system/docker-logrotate.timer
fi
