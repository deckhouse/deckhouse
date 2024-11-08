# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

terraform {
  required_providers {
    decort = {
      version = "4.6.1"
      source  = "terraform-provider-decort/decort"
    }
  }
  required_version = ">= 0.13"
}
