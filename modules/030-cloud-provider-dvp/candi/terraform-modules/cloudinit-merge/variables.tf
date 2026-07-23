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

variable "hostname" {
  type = string
}

variable "ssh_public_key" {
  type = string
}

variable "user_data" {
  description = "Bashible cloud-config payload (already base64-decoded), empty string for master-0 bootstrap."
  type        = string
  default     = ""
}

variable "ssh_ca_keys" {
  description = "SSH CA public keys to trust via TrustedUserCAKeys. Empty by default: rendering must stay bit-identical to the pre-existing behavior."
  type        = list(string)
  default     = []
}
