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

# Run from this directory with the OpenTofu version pinned in
# candi/terraform_versions.yml:
#
#   tofu init -backend=false && tofu test
#
# The pinned provider version is no longer downloadable from the registry, so keep a copy
# in a plugin cache and point OpenTofu at it to run offline:
#
#   export TF_PLUGIN_CACHE_DIR="$HOME/.terraform.d/plugin-cache"
#
# The Yandex Cloud provider is mocked, so nothing is created and no credentials are
# needed; the provider still has to be installable for its schema.
#
# This module has two modes, selected by the magic value "Auto" in var.apiKey:
# either it provisions a monitoring service account with a folder-scoped
# monitoring.viewer binding and an API key, or it passes an operator-supplied key
# through untouched. Both modes are covered here.

mock_provider "yandex" {}

variables {
  folderID = "folder-test"
  prefix   = "cluster-test"
}

run "auto_provisions_service_account_and_key" {
  command = plan

  variables {
    apiKey = "Auto"
  }

  assert {
    condition     = length(yandex_iam_service_account.monitoring_sa) == 1
    error_message = "expected the monitoring service account to be created when apiKey is Auto"
  }

  assert {
    condition     = yandex_iam_service_account.monitoring_sa[0].name == "cluster-test-monitoring"
    error_message = "expected the service account name to be prefixed with the cluster prefix"
  }

  assert {
    condition     = yandex_iam_service_account.monitoring_sa[0].folder_id == "folder-test"
    error_message = "expected the service account in the configured folder"
  }

  # The exporter only needs to read metrics, so the binding must stay at monitoring.viewer.
  assert {
    condition     = yandex_resourcemanager_folder_iam_binding.monitoring_sa[0].role == "monitoring.viewer"
    error_message = "expected the folder binding to grant monitoring.viewer only"
  }

  assert {
    condition     = length(yandex_iam_service_account_api_key.monitoring_sa) == 1
    error_message = "expected an API key to be created for the monitoring service account"
  }
}

# An operator-supplied key must not provision anything: the cluster then has no
# Deckhouse-managed service account to clean up, and the key is used verbatim.
run "explicit_key_creates_nothing" {
  command = plan

  variables {
    apiKey = "operator-supplied-key"
  }

  assert {
    condition     = length(yandex_iam_service_account.monitoring_sa) == 0
    error_message = "expected no service account when the API key is supplied explicitly"
  }

  assert {
    condition     = length(yandex_resourcemanager_folder_iam_binding.monitoring_sa) == 0
    error_message = "expected no folder binding when the API key is supplied explicitly"
  }

  assert {
    condition     = length(yandex_iam_service_account_api_key.monitoring_sa) == 0
    error_message = "expected no API key resource when the API key is supplied explicitly"
  }

  assert {
    condition     = output.apiKey == "operator-supplied-key"
    error_message = "expected the supplied API key to be passed through unchanged"
  }
}
