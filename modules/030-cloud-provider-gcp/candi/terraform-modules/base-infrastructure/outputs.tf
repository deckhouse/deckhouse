# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

output "cloud_discovery_data" {
  value = {
    "apiVersion"        = "deckhouse.io/v1"
    "kind"              = "GCPCloudDiscoveryData"
    "networkName"       = google_compute_network.kube.name
    "subnetworkName"    = google_compute_subnetwork.kube.name
    "zones"             = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.google_compute_zones.available.names, var.providerClusterConfiguration.zones)) : data.google_compute_zones.available.names
    "disableExternalIP" = var.providerClusterConfiguration.layout == "WithoutNAT" ? false : true
    "instances" = {
      "image"       = var.providerClusterConfiguration.masterNodeGroup.instanceClass.image
      "diskSizeGb"  = 50
      "diskType"    = "pd-standard"
      "networkTags" = [local.prefix]
      "labels"      = lookup(var.providerClusterConfiguration, "labels", {})
    }
  }
}
