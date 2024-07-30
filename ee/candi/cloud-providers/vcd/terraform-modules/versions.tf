# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

terraform {
  required_providers {
    vcd = {
      source = "vmware/vcd"
      version = "3.10.0"
    }
  }
  required_version = ">= 0.13"
}
