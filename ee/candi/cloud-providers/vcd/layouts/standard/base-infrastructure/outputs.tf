# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "cloud_discovery_data" {
  value = {
    "apiVersion"       = "deckhouse.io/v1"
    "kind"             = "VCDCloudProviderDiscoveryData"
    # vcloud director does not contain meaning zones
    # but out machinery use them. we use default as one zone
    "zones"            = ["default"]
  }
}
