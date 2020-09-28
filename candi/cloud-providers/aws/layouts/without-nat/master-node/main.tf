module "master-node" {
  source = "../../../terraform-modules/master-node"
  prefix = local.prefix
  cluster_uuid = var.clusterUUID
  node_index = var.nodeIndex
  node_group = var.providerClusterConfiguration.masterNodeGroup
  associate_public_ip_address = true
  root_volume_size = local.root_volume_size
  root_volume_type = local.root_volume_type
  additional_security_groups = local.additional_security_groups
  cloud_config = var.cloudConfig
  zones = local.zones
}
