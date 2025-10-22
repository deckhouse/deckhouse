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

bb-event-on 'bb-sync-file-changed' '_on_journald_service_config_changed'
_on_journald_service_config_changed() {
  systemctl restart systemd-journald.service
}

bb-sync-file /etc/systemd/journald.conf - << "EOF"
# Configure log rotation for all journal logs, which is where kubelet and
# container runtime  are configured to write their log entries.
# Journal config will:
# * stores individual Journal files for 24 hours before rotating to a new Journal file
# * keep only 14 old Journal files, and will discard older ones

[Journal]
MaxFileSec=24h
MaxRetentionSec=14day
EOF
