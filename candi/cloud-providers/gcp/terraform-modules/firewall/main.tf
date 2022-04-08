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

resource "google_compute_firewall" "ssh-and-icmp" {
  name    = join("-", [var.prefix, "ssh-and-ping"])
  network = var.network_self_link
  source_ranges = var.ssh_allow_list

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

  target_tags   = [var.prefix]
  source_tags   = [var.prefix]
  source_ranges = [var.pod_subnet_cidr]
}
