output "master_ip_address_for_ssh" {
  value = lookup(yandex_compute_instance.master.network_interface.0, "nat_ip_address", "") != "" ? yandex_compute_instance.master.network_interface.0.nat_ip_address : yandex_compute_instance.master.network_interface.0.ip_address
}

output "node_internal_ip_address" {
  value = yandex_compute_instance.master.network_interface.0.ip_address
}

output "kubernetes_data_device_path" {
  value = "/dev/disk/by-id/virtio-kubernetes-data"
}
