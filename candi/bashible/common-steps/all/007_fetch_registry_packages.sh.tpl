# Copyright 2026 Flant JSC
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

# Step intentionally a no-op.
#
# Registry packages are prefetched in the background by step
# `001_prefetch_registry_packages.sh` (systemd-run unit `rpp-prefetch.service`).
# Each install step (031_install_containerd, 034_ctr_import_local_images,
# 035_install_kubelet, ...) calls `bb-rpp-wait-fetched <name> <digest>` to block
# only on its own package, so an install starts as soon as its package finishes
# downloading instead of waiting for the whole batch.
#
# Kept as a deliberate stub (rather than deleted) so anyone reading the bundle
# steps in numeric order has a pointer to the new architecture.
:
