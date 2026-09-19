# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  provider_parameters = try(module.migration.settings.spec.settings.provider.parameters, {})
  credentials         = module.migration.credentials
}

provider "ovirt" {
  url           = try(local.provider_parameters.server, "")
  username      = try(local.credentials["d8-credentials"].identity, "")
  password      = try(local.credentials["d8-credentials"].secret, "")
  tls_insecure  = try(local.provider_parameters.insecure, false)
  tls_ca_bundle = base64decode(try(local.provider_parameters.caBundle, ""))
}
