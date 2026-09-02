#!/bin/bash

# Copyright 2026 Flant JSC
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

# AWS official Ubuntu images do not include linux-modules-extra by default,
# which means the erofs kernel module is unavailable. Install it so that
# the containerd v2 support check (000_check_containerd_v2_support.sh) can
# find and load erofs via modprobe.

# erofs may be built into the kernel — nothing to install.
if grep -qwe erofs /proc/filesystems || modprobe -qn erofs 2>/dev/null; then
  exit 0
fi

# linux-modules-extra-$(uname -r) is Ubuntu-specific; skip on other distros.
if ! bb-is-distro-like? "ubuntu"; then
  exit 0
fi

package="linux-modules-extra-$(uname -r)"

# linux-modules-extra is not published for every kernel build for aws images
# D8NodeContainerdV2NotSupported alert explains how to fix it.
if ! (bb-apt-install "$package"); then
  bb-log-warning "Failed to install '${package}': the erofs kernel module stays unavailable and containerd v2 cannot be used on this node. Switch the node group to an AMI whose kernel has a matching linux-modules-extra package."
fi
