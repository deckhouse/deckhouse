# Copyright 2024 Flant JSC
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

if [ ! -d /etc/apparmor.d ]; then
  exit 0
fi

bb-sync-file /etc/apparmor.d/deckhouse_controller - << "EOF"
#include <tunables/global>
profile deckhouse-controller flags=(attach_disconnected, mediate_deleted) {
  #include <abstractions/base>
  network,
  capability,
  file,
  umount,
  mount,
        
  # Host (privileged) processes may send signals to container processes.
  signal (receive) peer=unconfined,
  # Manager may send signals to container processes.
  signal (receive) peer=unconfined
  ,
  # Container processes may send signals amongst themselves.
  signal (send,receive) peer=test-dump-default-profile,
        
  deny @{PROC}/* w,   # deny write for all files directly in /proc (not in a subdir)
  # deny write to files not in /proc/<number>/** or /proc/sys/**
  deny @{PROC}/{[^1-9],[^1-9][^0-9],[^1-9s][^0-9y][^0-9s],[^1-9][^0-9][^0-9][^0-9]*}/** w,
  deny @{PROC}/sys/[^k]** w,  # deny /proc/sys except /proc/sys/k* (effectively /proc/sys/kernel)
  deny @{PROC}/sys/kernel/{?,??,[^s][^h][^m]**} w,  # deny everything except shm* in /proc/sys/kernel/
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/mem rwklx,
  deny @{PROC}/kmem rwklx,
  deny @{PROC}/kcore rwklx,
        
  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/** rwklx,
  deny /sys/kernel/security/** rwklx,
        
  ptrace (trace,read) peer=deckhouse-controller,
        
}
EOF

aa-enforce /etc/apparmor.d/deckhouse_controller
