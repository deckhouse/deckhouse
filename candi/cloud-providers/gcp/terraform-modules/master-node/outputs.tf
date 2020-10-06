output "id" {
  value = lookup(google_compute_instance.master, "instance_id")
}

output "node_internal_ip_address" {
  value = google_compute_instance.master.network_interface.0.network_ip
}

output "master_ip_address_for_ssh" {
  value = local.disable_external_ip == true ? google_compute_instance.master.network_interface.0.network_ip : google_compute_instance.master.network_interface.0.access_config.0.nat_ip
}

output "kubernetes_data_device_path" {
  value = google_compute_instance.master.attached_disk.0.device_name
}
