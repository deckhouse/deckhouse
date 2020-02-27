#!/bin/bash

if [[ ! -f "/etc/rsyslog.d/10-kubelet.conf" ]]; then
  echo ':programname,isequal, "kubelet" ~
  :programname,isequal, "dockerd" ~' | tee  /etc/rsyslog.d/10-kubelet.conf &&
  systemctl restart rsyslog
fi
