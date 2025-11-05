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

bb-flag-unset apt-updated
bb-flag-unset yum-updated

rm -f /var/lib/bashible/bootstrap-token
rm -f /var/lib/bashible/ca.crt
rm -f /var/lib/bashible/cloud-provider-bootstrap-networks-*.sh
rm -f /var/lib/bashible/detect_bundle.sh

rm -f "$BB_SYNC_UNHANDLED_FILES_STORE"
rm -f "$BB_APT_UNHANDLED_PACKAGES_STORE"
rm -f "$BB_YUM_UNHANDLED_PACKAGES_STORE"

# safety for re-bootstrap, look into 050_reset_control_plane_on_configuration_change.sh.tpl
find /.kubeadm.checksum -mmin +120 -delete >/dev/null 2>&1 || true

rm -f /var/lib/bashible/first_run
