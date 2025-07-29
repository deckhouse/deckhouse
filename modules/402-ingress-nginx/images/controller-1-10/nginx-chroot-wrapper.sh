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

timestamp=$(date +%Y%m%d_%H%M%S)
logfile="/var/log/valgrind/memcheck.${timestamp}.log"

if [[ "$PROFILING" == "true" ]]; then
  echo "Profiling enabled"

  echo "Drop NGINX capabilities"
  nginxchroot="/chroot/usr/local/nginx/sbin/nginx"

  if [ -z "$(getcap $nginxchroot)" ]; then
    echo "No capabilities set, skipping removal"
  else
    setcap -r $nginxchroot
  fi

  echo "Mounting proc fs"
  # unshare --mount-proc -f -p don't work, need use mount -t proc for parent pid
  mount -t proc /proc /chroot/proc

  echo "Run profiling with Valgrind"
  echo "The log will be written to a file $logfile"
  unshare -R /chroot /usr/local/valgrind \
    --trace-children=yes \
    --log-file="$logfile" \
    --tool=memcheck \
    --leak-check=full \
    --show-leak-kinds=all \
    /usr/local/nginx/sbin/nginx "$@"

else
  echo "Regular mode"
  unshare  -S 101 -R /chroot /usr/local/nginx/sbin/nginx "$@"
fi
