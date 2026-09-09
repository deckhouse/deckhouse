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

# The provider marks secret_key as sensitive, so this output has to be sensitive too:
# without the annotation any root module that exports it — the WithNATInstance layout
# publishes it in cloud_discovery_data — fails to plan with "Output refers to sensitive
# values". one() instead of [0] because a conditional evaluates both arms, and the
# resource has no instances unless apiKey is "Auto".
output "apiKey" {
  sensitive = true
  value     = var.apiKey == "Auto" ? one(yandex_iam_service_account_api_key.monitoring_sa[*].secret_key) : var.apiKey
}
