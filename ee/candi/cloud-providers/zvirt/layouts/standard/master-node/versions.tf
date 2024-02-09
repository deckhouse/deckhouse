# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

terraform {
  required_providers {
    vsphere = {
      source = "oVirt/ovirt"
      version = "2.1.5"
    }
  }
  required_version = ">= 0.13"
}
