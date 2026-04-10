/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module ee/se-plus/modules/030-cloud-provider-vsphere
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module ee/se-plus/modules/030-cloud-provider-zvirt
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module ee/modules/030-cloud-provider-openstack
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module ee/modules/030-cloud-provider-huaweicloud
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module ee/modules/030-cloud-provider-dynamix
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module ee/modules/030-cloud-provider-vcd
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module modules/030-cloud-provider-aws
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module modules/030-cloud-provider-azure
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module modules/030-cloud-provider-gcp
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module modules/030-cloud-provider-dvp
//go:generate /bin/bash sync_versions/sync_terraform_versions_with_oss.sh --repo-root .. --module modules/030-cloud-provider-yandex

//go:generate /bin/bash sync_versions/sync_oss_versions.sh --repo-root .. --source-module ee/se-plus/modules/030-cloud-provider-vsphere --source-id csi-vsphere --target-module modules/000-common --target-id csi-vsphere
//go:generate /bin/bash sync_versions/sync_oss_versions.sh --repo-root .. --source-module modules/030-cloud-provider-aws --source-id ccm-aws --target-module modules/007-registrypackages --target-id ecr-credential-provider-aws
