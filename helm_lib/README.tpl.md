## helm_lib_is_ha_to_value
 returns value <yes> if cluster is highly available, else — returns <no> 

### Usage
`{{ include "helm_lib_is_ha_to_value" (list . <yes> <no>) }} `


## helm_lib_ha_enabled
 returns empty value, which is treated by go template as false 

### Usage
`{{- if (include "helm_lib_ha_enabled" .) }} `


## helm_lib_module_ephemeral_storage_logs_with_extra
 50Mi for container logs `log-opts.max-file * log-opts.max-size` would be added to passed value 
 returns ephemeral-storage size for logs with extra space 

### Usage
`{{ include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 }} `


## helm_lib_module_ephemeral_storage_only_logs
 50Mi for container logs `log-opts.max-file * log-opts.max-size` would be requested 
 returns ephemeral-storage size for only logs 

### Usage
`{{ include "helm_lib_module_ephemeral_storage_only_logs" . }} `

## helm_lib_module_uri_scheme
 return module uri scheme "http" or "https" 

### Usage
`{{ include "helm_lib_module_uri_scheme" . }} `

## helm_lib_module_image
 returns image name 

### Usage
`{{ include "helm_lib_module_image" (list . "<container-name>") }} `


## helm_lib_module_image_no_fail
 returns image name if found 

### Usage
`{{ include "helm_lib_module_image_no_fail" (list . "<container-name>") }} `


## helm_lib_module_common_image
 returns image name from common module 

### Usage
`{{ include "helm_lib_module_common_image" (list . "<container-name>") }} `


## helm_lib_module_common_image_no_fail
 returns image name from common module if found 

### Usage
`{{ include "helm_lib_module_common_image_no_fail" (list . "<container-name>") }} `

## helm_lib_module_ingress_class
 returns ingress class from module settings or if not exists from global config 

### Usage
`{{ include "helm_lib_module_ingress_class" . }} `

## helm_lib_module_init_container_chown_nobody_volume
 ### Migration 11.12.2020: Remove this helper with all its usages after this commit reached RockSolid 
 returns initContainer which chowns recursively all files and directories in passed volume 

### Usage
`{{ include "helm_lib_module_init_container_chown_nobody_volume" (list . "volume-name") }} `


## helm_lib_module_init_container_check_linux_kernel
 returns initContainer which checks the kernel version on the node for compliance to semver constraint 

### Usage
`{{ include "helm_lib_module_init_container_check_linux_kernel" (list . ">= 4.9.17") }} `

## helm_lib_module_labels
 returns deckhouse labels 

### Usage
`{{ include "helm_lib_module_labels" (list . (dict "app" "test" "component" "testing")) }} `


## helm_lib_module_public_domain
 returns rendered publicDomainTemplate to service fqdn 

### Usage
`{{ include "helm_lib_module_public_domain" (list . "<name-portion>") }} `

## helm_lib_module_pod_security_context_run_as_user_custom
 returns PodSecurityContext parameters for Pod with custom user and group 

### Usage
`{{ include "helm_lib_module_pod_security_context_run_as_user_custom" (list . 1000 1000) }} `


## helm_lib_module_pod_security_context_run_as_user_nobody
 returns PodSecurityContext parameters for Pod with user and group nobody 

### Usage
`{{ include "helm_lib_module_pod_security_context_run_as_user_nobody" . }} `


## helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs
 returns PodSecurityContext parameters for Pod with user and group nobody with write access to mounted volumes 

### Usage
`{{ include "helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs" . }} `


## helm_lib_module_pod_security_context_run_as_user_root
 returns PodSecurityContext parameters for Pod with user and group 0 

### Usage
`{{ include "helm_lib_module_pod_security_context_run_as_user_root" . }} `


## helm_lib_module_container_security_context_not_allow_privilege_escalation
 returns SecurityContext parameters for Container with allowPrivilegeEscalation false 

### Usage
`{{ include "helm_lib_module_container_security_context_not_allow_privilege_escalation" . }} `


## helm_lib_module_container_security_context_read_only_root_filesystem
 returns SecurityContext parameters for Container with read only root filesystem 

### Usage
`{{ include "helm_lib_module_container_security_context_read_only_root_filesystem" . }} `


## helm_lib_module_container_security_context_privileged
 returns SecurityContext parameters for Container running privileged 

### Usage
`{{ include "helm_lib_module_container_security_context_privileged" . }} `


## helm_lib_module_container_security_context_privileged_read_only_root_filesystem
 returns SecurityContext parameters for Container running privileged with read only root filesystem 

### Usage
`{{ include "helm_lib_module_container_security_context_privileged_read_only_root_filesystem" . }} `


## helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all
 returns SecurityContext for Container with read only root filesystem and all capabilities dropped  

### Usage
`{{ include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" . }} `


## helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add
 returns SecurityContext parameters for Container with read only root filesystem, all dropped and some added capabilities 

### Usage
`{{ include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add"  (list . (list "KILL" "SYS_PTRACE")) }} `


## helm_lib_module_container_security_context_capabilities_drop_all_and_add
 returns SecurityContext parameters for Container with all dropped and some added capabilities 

### Usage
`{{ include "helm_lib_module_container_security_context_capabilities_drop_all_and_add"  (list . (list "KILL" "SYS_PTRACE")) }} `

## helm_lib_module_storage_class_annotations
 return module StorageClass annotations 

### Usage
`{{ include "helm_lib_module_storage_class_annotations" (list $ $index $storageClass.name) }} `

## helm_lib_grafana_dashboard_definitions_recursion
 returns all the dashboard-definintions from <root dir>/ 
 current dir is optional — used for recursion but you can use it for partially generating dashboards 

### Usage
`{{ include "helm_lib_grafana_dashboard_definitions_recursion" (list . <root dir> [current dir]) }} `


## helm_lib_grafana_dashboard_definitions
 returns dashboard-definintions from monitoring/grafana-dashboards/ 

### Usage
`{{ include "helm_lib_grafana_dashboard_definitions" . }} `


## helm_lib_single_dashboard
 renders a single dashboard 

### Usage
`{{ include "helm_lib_single_dashboard" (list . "dashboard-name" "folder" $dashboard) }} `

## helm_lib_prometheus_rules_recursion
 returns all the prometheus rules from <root dir>/ 
 current dir is optional — used for recursion but you can use it for partially generating rules 

### Usage
`{{ include "helm_lib_prometheus_rules_recursion" (list . <namespace> <root dir> [current dir]) }} `


## helm_lib_prometheus_rules
 returns all the prometheus rules from monitoring/prometheus-rules/ 

### Usage
`{{ include "helm_lib_prometheus_rules" (list . <namespace>) }} `


## helm_lib_prometheus_target_scrape_timeout_seconds
 returns adjust timeout value to scrape interval / 

### Usage
`{{ include "helm_lib_prometheus_target_scrape_timeout_seconds" (list . <timeout>) }} `



## helm_lib_priority_class
 returns priority class if priority-class module enabled, otherwise returns nothing 

### Usage
``

## helm_lib_resources_management_pod_resources
 returns rendered resources section based on configuration if it is 

### Usage
`{{ include "helm_lib_resources_management_pod_resources" (list <resources configuration> [ephemeral storage requests]) }} `


## helm_lib_resources_management_original_pod_resources
 returns rendered resources section based on configuration if it is present 

### Usage
`{{ include "helm_lib_resources_management_original_pod_resources" <resources configuration> }} `


## helm_lib_resources_management_vpa_spec
 returns rendered vpa spec based on configuration and target reference 

### Usage
`{{ include "helm_lib_resources_management_vpa_spec" (list <target apiversion> <target kind> <target name> <target container> <resources configuration> ) }} `


## helm_lib_resources_management_cpu_units_to_millicores
 helper for converting cpu units to millicores 

### Usage
`{{ include "helm_lib_resources_management_cpu_units_to_millicores" <cpu units> }} `


## helm_lib_resources_management_memory_units_to_bytes
 helper for converting memory units to bytes 

### Usage
`{{ include "helm_lib_resources_management_memory_units_to_bytes" <memory units> }} `


## helm_lib_vpa_kube_rbac_proxy_resources
 helper for VPA resources for kube_rbac_proxy 

### Usage
`{{ include "helm_lib_vpa_kube_rbac_proxy_resources" . }} `


## helm_lib_container_kube_rbac_proxy_resources
 helper for container resources for kube_rbac_proxy 

### Usage
`{{ include "helm_lib_container_kube_rbac_proxy_resources" . }} `

## helm_lib_pod_anti_affinity_for_ha
 returns pod affinity spec 

### Usage
`{{ include "helm_lib_pod_anti_affinity_for_ha" (list . (dict "app" "test")) }} `


## helm_lib_deployment_on_master_strategy_and_replicas_for_ha
 returns deployment strategy and replicas for ha components running on master nodes 

### Usage
`{{ include "helm_lib_deployment_on_master_strategy_and_replicas_for_ha" }} `


## helm_lib_deployment_strategy_and_replicas_for_ha
 returns deployment strategy and replicas for ha components running not on master nodes 

### Usage
`{{ include "helm_lib_deployment_strategy_and_replicas_for_ha" }} `
