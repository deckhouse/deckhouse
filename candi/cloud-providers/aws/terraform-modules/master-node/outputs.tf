output "master_public_ip" {
  value = var.associate_public_ip_address ? join("", aws_eip.eip.*.public_ip) : ""
}

output "master_private_ip" {
  value = aws_instance.master.private_ip
}

output "kubernetes_data_device_path" {
  value = aws_volume_attachment.kubernetes_data.device_name
}
