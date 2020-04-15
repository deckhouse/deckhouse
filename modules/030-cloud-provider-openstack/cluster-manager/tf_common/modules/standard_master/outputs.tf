output "id" {
  value = var.root_disk_size == "" ? openstack_compute_instance_v2.master[0].id : openstack_compute_instance_v2.master_with_root_disk[0].id
}

output "master_ip_address" {
  value = openstack_compute_floatingip_v2.master.address
}
