# TEST
output "deckhouse_config" {
  value = {
    userAuthnEnabled: "true"
  }
}

output "master_ip_address" {
  value = "1.2.3.4"
}

output "master_instance_class" {
  value = {
    some: "value"
    another: "another_value"
  }
}
