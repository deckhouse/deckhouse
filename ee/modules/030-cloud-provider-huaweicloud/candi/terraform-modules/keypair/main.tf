# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "huaweicloud_kps_keypair" "ssh" {
  name       = var.prefix
  public_key = var.ssh_public_key
  scope      = "user"
}
