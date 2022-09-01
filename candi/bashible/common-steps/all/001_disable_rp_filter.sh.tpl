# Copyright 2022 Flant JSC
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

bb-event-on 'bb-sync-file-changed' '_on_sysctl_config_changed'
_on_sysctl_config_changed() {
  systemctl restart systemd-sysctl
}

bb-sync-file /etc/sysctl.d/99-override_rp_filter.conf - << "EOF"
net.ipv4.conf.*.rp_filter = 0
EOF
