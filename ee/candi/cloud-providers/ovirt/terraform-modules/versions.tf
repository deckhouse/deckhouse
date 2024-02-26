# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

terraform {
  required_providers {
    ovirt = {
      source = "terraform-provider-ovirt/ovirt"
      version = "2.15.0"
    }
  }
  required_version = ">= 0.13"
}
