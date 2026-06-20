/*
Copyright 2021 Flant JSC

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

//go:generate /bin/bash -c "cd .. && go run ./tools/audit_policy/main.go -output-go ./modules/040-control-plane-manager/hooks/audit_policy_basic_targets_generated.go -output-doc-en ./docs/site/pages/virtualization-platform/documentation/admin/platform-management/security/KUBERNETES-API-AUDIT.md -output-doc-ru ./docs/site/pages/virtualization-platform/documentation/admin/platform-management/security/KUBERNETES-API-AUDIT_RU.md -output-doc-en-2 ./docs/documentation/pages/admin/configuration/security/KUBERNETES-API-AUDIT.md -output-doc-ru-2 ./docs/documentation/pages/admin/configuration/security/KUBERNETES-API-AUDIT_RU.md -output-rules-en ./modules/040-control-plane-manager/docs/AUDIT-RULES.md -output-rules-ru ./modules/040-control-plane-manager/docs/AUDIT-RULES_RU.md"
