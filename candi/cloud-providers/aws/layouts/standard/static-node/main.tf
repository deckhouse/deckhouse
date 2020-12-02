module "static-node" {
  source = "../../../terraform-modules/static-node"
  prefix = local.prefix
  cluster_uuid = var.clusterUUID
  node_index = var.nodeIndex
  node_group = local.node_group
  associate_public_ip_address = local.associate_public_ip_to_nodes
  root_volume_size = local.root_volume_size
  root_volume_type = local.root_volume_type
  additional_security_groups = local.additional_security_groups
  cloud_config = var.cloudConfig
  zones = local.zones
  tags = local.tags
}
