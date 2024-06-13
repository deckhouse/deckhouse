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

for swapunit in $(systemctl list-units --no-legend --plain --no-pager --type swap | cut -f1 -d" "); do
  systemctl stop "$swapunit"
  systemctl mask "$swapunit"
done

# systemd-gpt-auto-generator automatically detects swap partition in GPT and activates it     
if [ -f /lib/systemd/system-generators/systemd-gpt-auto-generator ]; then 
  mkdir /etc/systemd/system-generators
  ln -s /dev/null /etc/systemd/system-generators/systemd-gpt-auto-generator
fi

swapoff -a

if grep -q "swap" /etc/fstab; then
  sed -i '/[[:space:]]swap[[:space:]]/d' /etc/fstab
fi
