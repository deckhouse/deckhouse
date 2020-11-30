data "google_compute_zones" "available" {}

data "google_compute_address" "reserved" {
  for_each = toset(local.cloud_nat_addresses)
  name     = each.value
}

resource "google_compute_network" "kube" {
  name                    = local.prefix
  auto_create_subnetworks = false
  remove_routes_on_deletion = true
}

resource "google_compute_subnetwork" "kube" {
  name          = local.prefix
  network       = google_compute_network.kube.self_link
  ip_cidr_range = local.subnetwork_cidr
}

module "firewall" {
  source            = "../../../terraform-modules/firewall"
  prefix            = local.prefix
  network_self_link = google_compute_network.kube.self_link
  pod_subnet_cidr   = local.pod_subnet_cidr
}

locals {
  peered_vpcs = toset(local.peered_vpcs_names)
}

# network peering
data "google_compute_network" "other" {
  for_each = local.peered_vpcs
  name     = each.value
}

resource "google_compute_network_peering" "kube-with-other" {
  count        = length(local.peered_vpcs)
  name         = join("-with-", [local.prefix, local.peered_vpcs[count.index].name])
  network      = google_compute_network.kube.self_link
  peer_network = local.peered_vpcs[count.index].self_link
}

resource "google_compute_network_peering" "other-with-kube" {
  count        = length(local.peered_vpcs)
  name         = join("-with-", [local.peered_vpcs[count.index].name, local.prefix])
  network      = local.peered_vpcs[count.index].self_link
  peer_network = google_compute_network.kube.self_link
}
