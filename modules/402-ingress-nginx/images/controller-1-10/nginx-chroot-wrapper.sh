#!/bin/bash

# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

cat /etc/resolv.conf > /chroot/etc/resolv.conf

if [ "$NGINX_PROFILING_ENABLED" == "true" ]; then
  args="$@"
  nginxWithCaps="/chroot/usr/local/nginx/sbin/nginx"
  nginxWOCaps="/chroot/etc/ingress-controller/nginx/nginx"

  if [ ! -f $nginxWOCaps ]; then
    # copy the nginx binary to drop capabilities (valgrind doesn't want to profile a privileged file)
    cp -f $nginxWithCaps $nginxWOCaps
  fi

  if [ ! -f /chroot/proc/cmdline ]; then
    # echo "Mounting proc fs"
    # unshare --mount-proc -f -p don't work, need use mount -t proc for parent pid
    mount -t proc /proc /chroot/proc
  fi

  logDirInChroot="/var/log/valgrind"
  timestamp=$(date +%Y%m%d_%H%M%S%N)
  logfile="${logDirInChroot}/memcheck.${timestamp}.log"

  # echo "Run profiling with Valgrind"
  # echo "The log will be written to a file $logfile"
  unshare -R /chroot /usr/local/valgrind \
    --trace-children=yes \
    --log-file="$logfile" \
    --tool=memcheck \
    --leak-check=full \
    --show-leak-kinds=all \
    /etc/ingress-controller/nginx/nginx "$@"
else
  unshare -S 64535 -R /chroot /usr/local/nginx/sbin/nginx "$@"
fi
