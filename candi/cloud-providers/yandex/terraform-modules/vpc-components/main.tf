resource "yandex_vpc_route_table" "kube" {
  name           = var.prefix
  network_id     = var.network_id

  lifecycle {
    ignore_changes = [
      static_route,
    ]
  }

  labels = var.labels
}

resource "yandex_vpc_subnet" "kube_a" {
  name           = "${var.prefix}-a"
  network_id     = var.network_id
  v4_cidr_blocks = [cidrsubnet(var.node_network_cidr, ceil(log(3, 2)), 0)]
  route_table_id = yandex_vpc_route_table.kube.id
  zone           = "ru-central1-a"

  dynamic "dhcp_options" {
    for_each = (var.dhcp_domain_name != null) || (var.dhcp_domain_name_servers != null) ? [1] : []
    content {
      domain_name = var.dhcp_domain_name
      domain_name_servers = var.dhcp_domain_name_servers
    }
  }

  lifecycle {
    ignore_changes = [
      v4_cidr_blocks,
    ]
  }

  labels = var.labels
}

resource "yandex_vpc_subnet" "kube_b" {
  name           = "${var.prefix}-b"
  network_id     = var.network_id
  v4_cidr_blocks = [cidrsubnet(var.node_network_cidr, ceil(log(3, 2)), 1)]
  route_table_id = yandex_vpc_route_table.kube.id
  zone           = "ru-central1-b"

  dynamic "dhcp_options" {
    for_each = (var.dhcp_domain_name != null) || (var.dhcp_domain_name_servers != null) ? [1] : []
    content {
      domain_name = var.dhcp_domain_name
      domain_name_servers = var.dhcp_domain_name_servers
    }
  }

  lifecycle {
    ignore_changes = [
      v4_cidr_blocks,
    ]
  }

  labels = var.labels
}

resource "yandex_vpc_subnet" "kube_c" {
  name           = "${var.prefix}-c"
  network_id     = var.network_id
  v4_cidr_blocks = [cidrsubnet(var.node_network_cidr, ceil(log(3, 2)), 2)]
  route_table_id = yandex_vpc_route_table.kube.id
  zone           = "ru-central1-c"

  dynamic "dhcp_options" {
    for_each = (var.dhcp_domain_name != null) || (var.dhcp_domain_name_servers != null) ? [1] : []
    content {
      domain_name = var.dhcp_domain_name
      domain_name_servers = var.dhcp_domain_name_servers
    }
  }

  lifecycle {
    ignore_changes = [
      v4_cidr_blocks,
    ]
  }

  labels = var.labels
}
