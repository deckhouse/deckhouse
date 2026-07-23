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

# Fixture matching the real payload produced by
# modules/040-node-manager/templates/node-group/_cloud_init_cloud_config.tpl
# (define "node_group_cloud_init_cloud_config") - this is what dhctl/bashible
# actually puts into var.cloud_config for master-1/2/3 and every static-node.
# Not synthetic: same top-level keys, same write_files entries (path +
# permissions + multi-line content), same runcmd.
#
# NOTE: a file-level `variables` block only accepts keys that are declared
# input variables of the module under test - it cannot hold arbitrary
# test-only fixtures. So the fixture below is duplicated verbatim into the
# two `run` blocks that need it (scenario 3 and 4), instead of trying to
# smuggle it through a fake global variable.

# Scenario 1: master-0 bootstrap, feature NOT used (today's default for 100%
# of existing clusters). Must stay byte-identical to the pre-existing
# templatefile() rendering - no write_files/runcmd/CA content at all.
run "master0_no_ca_unchanged" {
  command = plan

  variables {
    hostname       = "master-0"
    ssh_public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIZakrNbKZ7i/uDQqxy7/FtPr4+H+pT7VC7ZxdVp0QXA"
    user_data      = ""
    ssh_ca_keys    = []
  }

  assert {
    condition     = !strcontains(output.user_data, "trusted-user-ca-keys")
    error_message = "empty ssh_ca_keys must not add any CA content to the rendering"
  }

  assert {
    condition     = !strcontains(output.user_data, "write_files")
    error_message = "empty ssh_ca_keys + empty user_data must not introduce write_files at all (legacy path)"
  }

  assert {
    condition     = yamldecode(output.user_data).hostname == "master-0"
    error_message = "legacy path must still render hostname correctly"
  }

  assert {
    condition     = yamldecode(output.user_data).ssh_authorized_keys[0] == "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIZakrNbKZ7i/uDQqxy7/FtPr4+H+pT7VC7ZxdVp0QXA"
    error_message = "legacy path must still render ssh_authorized_keys correctly"
  }
}

# Scenario 2: master-0 bootstrap, feature used. user_data is guaranteed empty
# for master-0 (dhctl passes NodeCloudConfig: ""), so base_cloud_config is {}.
run "master0_with_ca" {
  command = plan

  variables {
    hostname       = "master-0"
    ssh_public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIZakrNbKZ7i/uDQqxy7/FtPr4+H+pT7VC7ZxdVp0QXA"
    user_data      = ""
    ssh_ca_keys    = ["ssh-rsa-ca-AAAA-fake-vault-ca-key"]
  }

  assert {
    condition     = startswith(output.user_data, "#cloud-config\n")
    error_message = "merged path must still start with a valid #cloud-config header"
  }

  assert {
    condition     = output.merged_cloud_config.hostname == "master-0"
    error_message = "merged cloud-config must still carry hostname"
  }

  assert {
    condition     = length(output.merged_cloud_config.write_files) == 2
    error_message = "with empty user_data, only the 2 CA write_files entries are expected"
  }

  assert {
    condition     = [for wf in output.merged_cloud_config.write_files : wf.path][0] == "/etc/ssh/trusted-user-ca-keys.pem"
    error_message = "expected trusted-user-ca-keys.pem to be written"
  }

  assert {
    condition     = strcontains([for wf in output.merged_cloud_config.write_files : wf.content if wf.path == "/etc/ssh/trusted-user-ca-keys.pem"][0], "ssh-rsa-ca-AAAA-fake-vault-ca-key")
    error_message = "expected the configured CA key bytes to be present in trusted-user-ca-keys.pem content"
  }

  assert {
    condition     = contains(output.merged_cloud_config.runcmd, "sshd -t && systemctl reload ssh")
    error_message = "expected sshd reload runcmd to be present so CA trust is active before dhctl's first SSH attempt"
  }
}

# Scenario 3: master-1/2/3 or static-node, feature NOT used. This is TODAY's
# real behavior (confirmed collision-prone if we ever blindly appended
# write_files/runcmd) - must stay completely untouched: gate is on
# length(ssh_ca_keys), not on user_data emptiness.
run "multimaster_no_ca_unchanged" {
  command = plan

  variables {
    hostname       = "master-1"
    ssh_public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIZakrNbKZ7i/uDQqxy7/FtPr4+H+pT7VC7ZxdVp0QXA"
    user_data      = <<-EOT
      #cloud-config
      package_update: false
      package_upgrade: false
      manage_etc_hosts: localhost
      write_files:
      - path: '/var/lib/bashible/bootstrap.sh'
        permissions: '0700'
        content: |
          #!/bin/bash
          set -Eeuo pipefail
          mkdir -p /var/lib/bashible
          echo "bootstrapping node" >> /var/log/bashible.log
          exit 0
      - path: '/var/lib/bashible/ca.crt'
        permissions: '0644'
        content: |
          -----BEGIN CERTIFICATE-----
          MIIDUjCCAjqgAwIBAgIQFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKEFA==
          -----END CERTIFICATE-----
      - path: /var/lib/bashible/bootstrap-token
        content: abcdef.0123456789abcdef
        permissions: '0600'
      - path: /var/lib/bashible/first_run
      runcmd:
      - /var/lib/bashible/bootstrap.sh
    EOT
    ssh_ca_keys    = []
  }

  assert {
    condition     = !strcontains(output.user_data, "trusted-user-ca-keys")
    error_message = "empty ssh_ca_keys must not add any CA content even when user_data (bashible payload) is non-empty"
  }

  assert {
    condition     = length(yamldecode(output.user_data).write_files) == 4
    error_message = "bashible's own 4 write_files entries must be untouched when the feature is not used"
  }
}

# Scenario 4: THE core regression test. master-1/2/3 or static-node, feature
# used - this is exactly the case that a naive text-concatenation
# implementation would have broken (bashible's write_files/runcmd silently
# overwritten). Proves the structural merge actually fixes it.
run "multimaster_with_ca_merges_without_collision" {
  command = plan

  variables {
    hostname       = "master-1"
    ssh_public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIZakrNbKZ7i/uDQqxy7/FtPr4+H+pT7VC7ZxdVp0QXA"
    user_data      = <<-EOT
      #cloud-config
      package_update: false
      package_upgrade: false
      manage_etc_hosts: localhost
      write_files:
      - path: '/var/lib/bashible/bootstrap.sh'
        permissions: '0700'
        content: |
          #!/bin/bash
          set -Eeuo pipefail
          mkdir -p /var/lib/bashible
          echo "bootstrapping node" >> /var/log/bashible.log
          exit 0
      - path: '/var/lib/bashible/ca.crt'
        permissions: '0644'
        content: |
          -----BEGIN CERTIFICATE-----
          MIIDUjCCAjqgAwIBAgIQFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKEFA==
          -----END CERTIFICATE-----
      - path: /var/lib/bashible/bootstrap-token
        content: abcdef.0123456789abcdef
        permissions: '0600'
      - path: /var/lib/bashible/first_run
      runcmd:
      - /var/lib/bashible/bootstrap.sh
    EOT
    ssh_ca_keys    = ["ssh-rsa-ca-AAAA-fake-vault-ca-key-1", "ssh-rsa-ca-AAAA-fake-vault-ca-key-2"]
  }

  assert {
    condition     = length(output.merged_cloud_config.write_files) == 6
    error_message = "expected 4 bashible write_files + 2 CA write_files = 6, none overwritten"
  }

  assert {
    condition     = length(output.merged_cloud_config.runcmd) == 3
    error_message = "expected 1 bashible runcmd + 2 CA runcmd = 3, none overwritten"
  }

  assert {
    condition     = contains([for wf in output.merged_cloud_config.write_files : wf.path], "/var/lib/bashible/bootstrap.sh")
    error_message = "bashible's bootstrap.sh write_files entry must survive the merge untouched"
  }

  assert {
    condition     = contains([for wf in output.merged_cloud_config.write_files : wf.path], "/etc/ssh/trusted-user-ca-keys.pem")
    error_message = "our CA write_files entry must be present alongside bashible's, not instead of it"
  }

  assert {
    condition     = contains(output.merged_cloud_config.runcmd, "/var/lib/bashible/bootstrap.sh")
    error_message = "bashible's own runcmd entry (node join) must survive the merge - this is the actual collision bashible depends on"
  }

  assert {
    condition     = strcontains([for wf in output.merged_cloud_config.write_files : wf.content if wf.path == "/var/lib/bashible/bootstrap.sh"][0], "set -Eeuo pipefail")
    error_message = "round-trip (yamldecode -> yamlencode) must not corrupt bashible's multi-line shell script content"
  }

  assert {
    condition     = [for wf in output.merged_cloud_config.write_files : wf.content if wf.path == "/var/lib/bashible/bootstrap-token"][0] == "abcdef.0123456789abcdef"
    error_message = "round-trip must not corrupt the bootstrap token value"
  }

  assert {
    condition     = join("\n", var.ssh_ca_keys) == [for wf in output.merged_cloud_config.write_files : wf.content if wf.path == "/etc/ssh/trusted-user-ca-keys.pem"][0]
    error_message = "expected both configured CA keys to be present in trusted-user-ca-keys.pem, newline separated"
  }
}
