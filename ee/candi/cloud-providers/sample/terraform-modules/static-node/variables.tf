# Copyright 2024 Flant JSC
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

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "nodeIndex" {
  type    = number
  default = 0
}

variable "cloudConfig" {
  type = string
  default = ""
}

variable "nodeGroupName" {
  type = string
}

locals {
  resource_name_prefix = var.clusterConfiguration.cloud.prefix
  ng             = [for i in var.providerClusterConfiguration.nodeGroups : i if i.name == var.nodeGroupName][0]
  instance_class = local.ng["instanceClass"]
  node_group_name = local.ng.name
  node_name = join("-", [local.resource_name_prefix, local.node_group_name, var.nodeIndex])
  cpus = lookup(local.instance_class, "numCPUs", [])
  ram_mb = lookup(local.instance_class, "memory", [])
  ssh_pubkey = lookup(var.providerClusterConfiguration, "sshPublicKey", null)
  root_disk_size = lookup(local.instance_class, "rootDiskSizeGb", 50)
  image_name = lookup(local.instance_class, "imageName", null)

  cloud_init_script = yamlencode(merge({
    "hostname": local.node_name,
    "create_hostname_file": true,
    "ssh_deletekeys": true,
    "ssh_genkeytypes": ["rsa", "ecdsa", "ed25519"],
    "ssh_authorized_keys": [local.ssh_pubkey],
    "users": [
      {
        "name" : "user",
        "ssh_authorized_keys" : [local.ssh_pubkey]
        "groups": "users, wheel",
        "sudo": "ALL=(ALL) NOPASSWD:ALL"
      }
    ]
  }, length(var.cloudConfig) > 0 ? yamldecode(base64decode(var.cloudConfig)) : tomap({})))
}
