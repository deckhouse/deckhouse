resource "google_compute_firewall" "ssh-and-icmp" {
  name    = join("-", [var.prefix, "ssh-and-ping"])
  network = var.network_self_link

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  target_tags = [var.prefix]
}

resource "google_compute_firewall" "intercommunication" {
  name    = join("-", [var.prefix, "intercommunication"])
  network = var.network_self_link

  allow {
    protocol = "all"
  }

  target_tags = [var.prefix]
  source_tags = [var.prefix]
}
