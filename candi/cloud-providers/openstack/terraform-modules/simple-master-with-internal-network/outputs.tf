output "id" {
  value = var.root_disk_size == "" ? openstack_compute_instance_v2.master[0].id : openstack_compute_instance_v2.master_with_root_disk[0].id
}

output "node_ip" {
  value = var.root_disk_size == "" ? openstack_compute_instance_v2.master[0].network[0].fixed_ip_v4: openstack_compute_instance_v2.master_with_root_disk[0].network[0].fixed_ip_v4
}

output "master_ip_address" {
  value = var.root_disk_size == "" ? openstack_compute_instance_v2.master[0].network[0].fixed_ip_v4: openstack_compute_instance_v2.master_with_root_disk[0].network[0].fixed_ip_v4
}
