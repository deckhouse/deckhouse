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
# bashible: parallel-group=light-prep

# Neutralise unattended-upgrades without waiting on /var/lib/dpkg/lock: config off + mask + async stop. No-op on non-apt or non-systemd systems.
units="unattended-upgrades.service apt-daily.timer apt-daily-upgrade.timer apt-daily.service apt-daily-upgrade.service"

if [ -d /etc/apt/apt.conf.d ]; then
  bb-sync-file /etc/apt/apt.conf.d/20auto-upgrades - << "EOF"
APT::Periodic::Update-Package-Lists "0";
APT::Periodic::Unattended-Upgrade "0";
EOF
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl mask --quiet $units >/dev/null 2>&1 || true
  systemctl stop --no-block $units >/dev/null 2>&1 || true
fi
