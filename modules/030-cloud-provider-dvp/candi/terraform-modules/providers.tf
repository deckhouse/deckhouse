# Copyright 2025 Flant JSC
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
  _has_pcc                = var.providerClusterConfiguration != null
  _has_credential_secret  = var.secrets != null && length(var.secrets) > 0
  _has_node_groups        = var.nodeGroups != null && length(var.nodeGroups) > 0
  _has_instance_classes   = var.instanceClasses != null && length(var.instanceClasses) > 0
  _new_resources_complete = local._has_credential_secret && local._has_node_groups && local._has_instance_classes
  _use_pcc                = local._has_pcc && !local._new_resources_complete

  _pcc_kubeconfig = try(var.providerClusterConfiguration.provider.kubeconfigDataBase64, "")
  _secret_kubeconfig = try(
    [
      for name, s in var.secrets : s.stringData.secret
      if try(s.stringData.secret, null) != null && try(s.type, "") == "cloud-provider.deckhouse.io/credentials"
    ][0],
    ""
  )
  _kubeconfig_base64 = local._use_pcc ? local._pcc_kubeconfig : local._secret_kubeconfig
}

provider "kubernetes" {
  config_data_base64 = local._kubeconfig_base64
  # insecure           = true
}
