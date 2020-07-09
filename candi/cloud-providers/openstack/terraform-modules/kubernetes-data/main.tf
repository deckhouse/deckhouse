resource "openstack_blockstorage_volume_v2" "kubernetes_data" {
  name = join("-", [var.prefix, "kubernetes-data", var.node_index])
  description = "volume for etcd and kubernetes certs"
  size = 10
  volume_type = var.volume_type
}

resource "openstack_compute_volume_attach_v2" "kubernetes_data" {
  instance_id = var.master_id
  volume_id = openstack_blockstorage_volume_v2.kubernetes_data.id
}
