# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

locals {
  zone_to_subnet_id_map_a = merge({}, (length(data.yandex_vpc_subnet.kube_a) > 0 ? {(data.yandex_vpc_subnet.kube_a[0].zone): data.yandex_vpc_subnet.kube_a[0].id } : {} ))
  zone_to_subnet_id_map_b = merge(local.zone_to_subnet_id_map_a, (length(data.yandex_vpc_subnet.kube_b) > 0 ?{(data.yandex_vpc_subnet.kube_b[0].zone): data.yandex_vpc_subnet.kube_b[0].id } : {} ))
  zone_to_subnet_id_map_d_final = merge(local.zone_to_subnet_id_map_b, (length(data.yandex_vpc_subnet.kube_d) > 0 ? {(data.yandex_vpc_subnet.kube_d[0].zone): data.yandex_vpc_subnet.kube_d[0].id } : {} ))
}

output "route_table_id" {
  value = yandex_vpc_route_table.kube.id
}

output "zone_to_subnet_id_map" {
    value = local.should_create_subnets ? {
      (yandex_vpc_subnet.kube_a[0].zone): yandex_vpc_subnet.kube_a[0].id
      (yandex_vpc_subnet.kube_b[0].zone): yandex_vpc_subnet.kube_b[0].id
      (yandex_vpc_subnet.kube_d[0].zone): yandex_vpc_subnet.kube_d[0].id
    } : local.zone_to_subnet_id_map_d_final
}

output "nat_instance_name" {
  value = local.is_with_nat_instance ? yandex_compute_instance.nat_instance[0].name : ""
}

output "nat_instance_zone" {
  value = local.is_with_nat_instance ? yandex_compute_instance.nat_instance[0].zone : ""
}
