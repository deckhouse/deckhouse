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

# This module has intentionally no providers/resources: it is pure data
# transformation, so it can be unit-tested with `tofu test`/`terraform test`
# without a real cluster (same pattern as ../migration).

locals {
  # Pre-existing behavior, byte-for-byte: used whenever ssh_ca_keys is empty,
  # regardless of whether user_data is empty (master-0) or not (master-1/2/3,
  # static-node). Nothing about this path changes with this module's
  # introduction - it is the exact same templatefile() call that used to live
  # directly in master/main.tf and static-node/main.tf.
  legacy_user_data = templatefile("${path.module}/templates/cloudinit.tftpl", {
    host_name      = var.hostname
    ssh_public_key = var.ssh_public_key
    user_data      = var.user_data
  })

  # Structural (not textual) merge, used only when ssh_ca_keys is non-empty.
  # The ternary must branch on two strings (not {} vs. object) - Terraform
  # requires the true/false branches of a conditional to unify to the same
  # type, and an empty object never unifies with the actual decoded object's
  # attribute set.
  base_cloud_config = yamldecode(var.user_data == "" ? "{}" : var.user_data)

  static_block = {
    hostname                  = var.hostname
    prefer_fqdn_over_hostname = false
    ssh_authorized_keys       = [var.ssh_public_key]
    users                     = ["default"]
  }

  ca_write_files = [
    {
      path    = "/etc/ssh/trusted-user-ca-keys.pem"
      content = join("\n", var.ssh_ca_keys)
    },
    {
      path    = "/etc/ssh/sshd_config.d/50-trusted-ca.conf"
      content = "TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pem"
    },
  ]

  ca_runcmd = [
    "grep -q '^Include /etc/ssh/sshd_config.d/\\*\\.conf' /etc/ssh/sshd_config || sed -i '1i Include /etc/ssh/sshd_config.d/*.conf' /etc/ssh/sshd_config",
    "sshd -t && systemctl reload ssh",
  ]

  # merge() overrides on matching top-level keys, later args win - so
  # write_files/runcmd below always take the explicit concatenated value,
  # never a bare overwrite of the bashible-supplied list.
  merged_cloud_config = merge(
    local.base_cloud_config,
    local.static_block,
    {
      write_files = concat(try(local.base_cloud_config.write_files, []), local.ca_write_files)
      runcmd      = concat(try(local.base_cloud_config.runcmd, []), local.ca_runcmd)
    },
  )

  final_user_data = length(var.ssh_ca_keys) > 0 ? (
    "#cloud-config\n${yamlencode(local.merged_cloud_config)}"
    ) : (
    local.legacy_user_data
  )
}
