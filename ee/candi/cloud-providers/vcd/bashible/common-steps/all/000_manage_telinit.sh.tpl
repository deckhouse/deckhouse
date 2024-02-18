#!/bin/bash

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

# To fix cloud-init bug https://github.com/vmware/open-vm-tools/issues/684.
# Vmware-guest-tools uses telinit to reboot node and we removes telinit on the bootstrap phase
# to prevent unwanted reboots during bootstrap process. Later we return telinit back.
if [ "$FIRST_BASHIBLE_RUN" == "yes" ]; then
  mv -f /sbin/telinit /sbin/telinit.removed
  exit 0
fi

if [ -f /sbin/telinit.removed ]; then
  mv -f /sbin/telinit.removed /sbin/telinit
fi
