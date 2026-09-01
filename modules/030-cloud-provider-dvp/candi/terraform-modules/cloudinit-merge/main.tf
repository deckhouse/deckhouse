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
  # Pre-existing behavior, byte-for-byte: used whenever ssh_ca_keys and
  # additional_users are both empty, regardless of whether user_data is empty
  # (master-0) or not (master-1/2/3, static-node). Nothing about this path
  # changes with this module's introduction - it is the exact same
  # templatefile() call that used to live directly in master/main.tf and
  # static-node/main.tf.
  legacy_user_data = templatefile("${path.module}/templates/cloudinit.tftpl", {
    host_name      = var.hostname
    ssh_public_key = var.ssh_public_key
    user_data      = var.user_data
  })

  # Structural (not textual) merge, used whenever ssh_ca_keys or
  # additional_users is non-empty. The ternary must branch on two strings
  # (not {} vs. object) - Terraform requires the true/false branches of a
  # conditional to unify to the same type, and an empty object never unifies
  # with the actual decoded object's attribute set.
  base_cloud_config = yamldecode(var.user_data == "" ? "{}" : var.user_data)

  # Additional named users, created alongside (not instead of) the image's
  # default user. No keys/passwd of their own: SSH access relies entirely on
  # ssh_ca_keys (TrustedUserCAKeys is host-wide) or an out-of-band mechanism.
  #
  # Intentionally NOT using groups=["sudo"]: the "sudo" group only exists on
  # Debian/Ubuntu - RHEL/SUSE/AltLinux (all supported OS families, see
  # candi/version_map.yml) use "wheel" instead, and cloud-init would fail to
  # add the user to a non-existent group. The "sudo" key below (rendered as
  # a per-user /etc/sudoers.d entry, see ngc-additional-users.yaml for the
  # day-2 equivalent) already grants access directly to the named user, so
  # group membership is redundant on top of it.
  additional_user_blocks = [
    for name in var.additional_users : {
      name  = name
      sudo  = "ALL=(ALL) NOPASSWD:ALL"
      shell = "/bin/bash"
    }
  ]

  static_block = {
    hostname                  = var.hostname
    prefer_fqdn_over_hostname = false
    ssh_authorized_keys       = [var.ssh_public_key]
    users                     = concat(["default"], local.additional_user_blocks)
  }

  # Only emitted when ssh_ca_keys is actually set - additional_users alone
  # (no CA) must not write an empty/pointless trusted-user-ca-keys.pem.
  ca_write_files = length(var.ssh_ca_keys) > 0 ? [
    {
      path    = "/etc/ssh/trusted-user-ca-keys.pem"
      content = join("\n", var.ssh_ca_keys)
    },
    {
      path    = "/etc/ssh/sshd_config.d/50-trusted-ca.conf"
      content = "TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pem"
    },
  ] : []

  ca_runcmd = length(var.ssh_ca_keys) > 0 ? [
    "grep -q '^Include /etc/ssh/sshd_config.d/\\*\\.conf' /etc/ssh/sshd_config || sed -i '1i Include /etc/ssh/sshd_config.d/*.conf' /etc/ssh/sshd_config",
    "sshd -t && systemctl reload ssh",
  ] : []

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

  final_user_data = (length(var.ssh_ca_keys) > 0 || length(var.additional_users) > 0) ? (
    "#cloud-config\n${yamlencode(local.merged_cloud_config)}"
    ) : (
    local.legacy_user_data
  )
}
