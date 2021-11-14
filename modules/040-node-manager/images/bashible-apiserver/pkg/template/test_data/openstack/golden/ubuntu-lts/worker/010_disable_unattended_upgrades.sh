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

# Disable auto reboot and remove unused deps
if [ -f "/etc/apt/apt.conf.d/50unattended-upgrades" ] ; then
  sed -i 's/\/\/Unattended-Upgrade::Automatic-Reboot "false"/Unattended-Upgrade::Automatic-Reboot "false"/g' /etc/apt/apt.conf.d/50unattended-upgrades
  sed -i 's/\/\/Unattended-Upgrade::InstallOnShutdown "true"/Unattended-Upgrade::InstallOnShutdown "false"/g' /etc/apt/apt.conf.d/50unattended-upgrades
  sed -i 's/\/\/Unattended-Upgrade::Remove-Unused-Dependencies "false"/Unattended-Upgrade::Remove-Unused-Dependencies "false"/g' /etc/apt/apt.conf.d/50unattended-upgrades
fi

# Disable periodic unattended-upgrades
sed -i 's/APT::Periodic::Unattended-Upgrade "1"/APT::Periodic::Unattended-Upgrade "0"/g' /etc/apt/apt.conf.d/*
