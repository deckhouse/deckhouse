output "route_table_id" {
  value = yandex_vpc_route_table.kube.id
}

output "zone_to_subnet_id_map" {
    value = {
      (yandex_vpc_subnet.kube_a.zone): yandex_vpc_subnet.kube_a.id
      (yandex_vpc_subnet.kube_b.zone): yandex_vpc_subnet.kube_b.id
      (yandex_vpc_subnet.kube_c.zone): yandex_vpc_subnet.kube_c.id
    }
}
