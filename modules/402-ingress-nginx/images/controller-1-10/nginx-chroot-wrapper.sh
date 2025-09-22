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
  if [ "$args" == "-c /etc/nginx/nginx.conf" ]; then
    logDirInChroot="/var/log/valgrind"
    timestamp=$(date +%Y%m%d_%H%M%S)
    logfile="${logDirInChroot}/memcheck.${timestamp}.log"

    # echo "Drop NGINX file capabilities as it prevent valgrind from running"
    nginxWithCaps="/chroot/usr/local/nginx/sbin/nginx"
    nginxWOCaps="/chroot/etc/ingress-controller/nginx/"
    # copy the nginx binary to drop capabilities (valgrind doesn't want to profile a privileged file)
    cp -f $nginxWithCaps $nginxWOCaps

    # echo "Mounting proc fs"
    # unshare --mount-proc -f -p don't work, need use mount -t proc for parent pid
    mount -t proc /proc /chroot/proc
    # Set hack for www-data user
    # echo 'www-data:x:64535:64535:www-data:/nonexistent:/usr/sbin/nologin' >> /chroot/etc/passwd
    # echo 'www-data:x:64535:' >> /chroot/etc/group

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
    unshare -R /chroot /etc/ingress-controller/nginx/nginx "$@"
  fi
else
  unshare -S 64535 -R /chroot /usr/local/nginx/sbin/nginx "$@"
fi
