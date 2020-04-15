output "master_ip_address" {
  value = module.standard_master.master_ip_address
}

output "master_instance_class" {
  value = {
    "apiVersion": "deckhouse.io/v1alpha1"
    "kind": "OpenStackInstanceClass"
    "metadata": {
      "name": "master"
    }
    "spec": {
      "flavorName": local.flavor_name
      "imageName": local.image_name
      "rootDiskSize": local.root_disk_size
      "mainInterwork": data.openstack_networking_network_v2.internal.name
//      "floatingIPsFromNetworks": [local.external_network_name]
    }
  }
}

output "deckhouse_config" {
  value = {}
}
