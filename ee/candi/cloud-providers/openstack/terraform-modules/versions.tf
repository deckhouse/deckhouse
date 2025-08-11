# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

terraform {
  required_providers {
    openstack = {
      source = "terraform-provider-openstack/openstack"
      version = "1.32.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "3.2.4"
    }
  }
  required_version = ">= 0.13"
}
