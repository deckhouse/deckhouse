#!/bin/sh

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

if [ -z "${NTP_SERVERS}" ]; then
  echo "NTP_SERVERS env must be set"
  exit 1
fi

if ss -nlup | grep -q "127.0.0.1:123"; then
  echo "NTP port on node is used"
  exit 1
fi

touch /var/run/chrony/chrony.drift
chown chrony:chrony -R /var/run/chrony
chmod 700 /var/run/chrony

cat << EOF > /var/run/chrony/chrony.conf
user chrony
cmdallow 127/8
allow 127/8
bindaddress 127.0.0.1
driftfile /var/run/chrony/chrony.drift
makestep 1.0 -1
rtcsync
EOF
for NTP_SERVER in ${NTP_SERVERS}; do
  echo "pool ${NTP_SERVER} iburst" >> /var/run/chrony/chrony.conf
done

# remove stale pidfile
rm -f /run/chrony/chronyd.pid
# Run Chrony Daemon
chronyd -d -s -f /var/run/chrony/chrony.conf
