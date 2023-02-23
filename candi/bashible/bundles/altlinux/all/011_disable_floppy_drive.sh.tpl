# Copyright 2023 Flant JSC
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

# There is issue that blkid hangs on nodes with kernel 5.x.x version because of floppy drive presence.
# We don't need floppy drive on kubernetes nodes so we disable it for good.
if [[ -f /etc/modprobe.d/blacklist-floppy.conf ]]; then
  return 0
fi

echo "blacklist floppy" > /etc/modprobe.d/blacklist-floppy.conf
if lsmod | grep floppy -q ; then
    make-initrd
    bb-flag-set reboot
fi
