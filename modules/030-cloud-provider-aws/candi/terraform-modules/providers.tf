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

provider "aws" {
  access_key = var.providerClusterConfiguration.provider.providerAccessKeyId
  secret_key = var.providerClusterConfiguration.provider.providerSecretAccessKey
  region     = var.providerClusterConfiguration.provider.region
  # The provider validates the region against a list compiled into its binary, so regions
  # launched after the provider release (e.g. ap-southeast-6) fail with "invalid AWS Region".
  # Endpoints for such regions are still resolved by the AWS SDK.
  skip_region_validation = true
}
