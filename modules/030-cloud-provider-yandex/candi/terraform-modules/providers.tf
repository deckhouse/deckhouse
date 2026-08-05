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

# This file is symlinked into every root module: each layout's
# base-infrastructure, master-node and static-node. All of them declare
# module "migration", so this is the one place shared by every root module and
# therefore the single place where the resolved credentials are unwrapped for use;
# local.credentials below is also consumed by the layouts.

locals {
  credentials_name = "d8-credentials"

  # Read the *resolved* configuration, never var.settings/var.secrets directly.
  # The migration module decides which source of truth wins, and in a
  # half-migrated cluster that is still the legacy YandexClusterConfiguration.
  # Configuring the provider from the new model there would let converge act on
  # a different cloud/folder, or with a different service account key, than the
  # configuration the rest of the layout plans against.
  _resolved_provider_parameters = try(module.migration.settings.spec.settings.provider.parameters, {})

  # The selection itself lives in the migration module, which resolves the source of truth;
  # nonsensitive() is needed to look values up by key here and to keep the WithNATInstance
  # layout able to publish the monitoring API key in cloud_discovery_data, exactly as it did
  # before the migration.
  credentials = nonsensitive(module.migration.credentials)
}

provider "yandex" {
  cloud_id                 = try(local._resolved_provider_parameters.cloudID, "")
  folder_id                = try(local._resolved_provider_parameters.folderID, "")
  service_account_key_file = lookup(local.credentials, local.credentials_name, "")
}
