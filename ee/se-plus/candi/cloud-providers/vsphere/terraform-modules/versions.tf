# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

terraform {
  required_providers {
    vsphere = {
      source = "vmware/vsphere"
      version = "2.14.2"
    }
  }
  required_version = ">= 0.13"
}
