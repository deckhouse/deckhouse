# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

resource "openstack_compute_keypair_v2" "ssh" {
  name = var.prefix
  public_key = var.ssh_public_key
}
