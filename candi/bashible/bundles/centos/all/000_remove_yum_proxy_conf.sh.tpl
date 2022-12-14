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

# If yum-utils is not installed, we will try to install it. In closed environments yum-utils should be preinstalled in distro image
# We cannot use bb-* commands, due to absent yum-plugin-versionlock package,
# which will be installed later in 001_install_mandatory_packages.sh step.

# TODO remove after 1.42 release !!!

if ! rpm -q --quiet yum-utils; then
  yum install -y yum-utils
fi

if bb-is-centos-version? 7; then
  proxy="_none_"
fi

if yum --version | grep -q dnf; then
  proxy=""
fi

yum-config-manager --save --setopt=proxy=${proxy}
yum-config-manager --save --setopt=proxy_username=${proxy_username}
yum-config-manager --save --setopt=proxy_password=${proxy_password}
