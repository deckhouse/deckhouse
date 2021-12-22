# Copyright 2021 Flant JSC
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

bb-event-on 'bb-sync-file-changed' '_on_apparmor_profile_changed'
_on_apparmor_profile_changed() {
  systemctl reload apparmor.service
}

bb-sync-file /etc/apparmor.d/node-exporter - << "EOF"
#include <tunables/global>

profile node-exporter flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>

  capability,

  network,

  deny mount,

  umount,

  ptrace read peer=cri-containerd.apparmor.d,
  ptrace read peer=unconfined,

  deny /sys/[^f]*/** wlkx,
  deny /sys/f[^s]*/** wlkx,
  deny /sys/firmware/efi/efivars/** rwlkx,
  deny /sys/fs/[^c]*/** wlkx,
  deny /sys/fs/c[^g]*/** wlkx,
  deny /sys/fs/cg[^r]*/** wlkx,
  deny /sys/kernel/security/** rwlkx,
  deny @{PROC}/* w, # deny write for all files directly in /proc (not in a subdir)
  deny @{PROC}/kcore rwlkx,
  deny @{PROC}/kmem rwlkx,
  deny @{PROC}/mem rwlkx,
  deny @{PROC}/sys/[^k]** w, # deny /proc/sys except /proc/sys/k* (effectively /proc/sys/kernel)
  deny @{PROC}/sys/kernel/{?,??,[^s][^h][^m]**} w, # deny everything except shm* in /proc/sys/kernel/
  deny @{PROC}/sysrq-trigger rwlkx,
  deny @{PROC}/{[^1-9],[^1-9][^0-9],[^1-9s][^0-9y][^0-9s],[^1-9][^0-9][^0-9][^0-9]*}/** w,

  file,

}
EOF
