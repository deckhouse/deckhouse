# Helm library for Deckhouse modules

## Table of contents

| Table of contents |
|---|
| **Enable Ds Eviction** |
| [helm_lib_prevent_ds_eviction_annotation](#helm_lib_prevent_ds_eviction_annotation) |
| **Envs For Proxy** |
| [helm_lib_envs_for_proxy](#helm_lib_envs_for_proxy) |
| **High Availability** |
| [helm_lib_is_ha_to_value](#helm_lib_is_ha_to_value) |
| [helm_lib_ha_enabled](#helm_lib_ha_enabled) |
| **Kube Rbac Proxy** |
| [helm_lib_kube_rbac_proxy_ca_certificate](#helm_lib_kube_rbac_proxy_ca_certificate) |
| **Module Documentation Uri** |
| [helm_lib_module_documentation_uri](#helm_lib_module_documentation_uri) |
| **Module Ephemeral Storage** |
| [helm_lib_module_ephemeral_storage_logs_with_extra](#helm_lib_module_ephemeral_storage_logs_with_extra) |
| [helm_lib_module_ephemeral_storage_only_logs](#helm_lib_module_ephemeral_storage_only_logs) |
| **Module Generate Common Name** |
| [helm_lib_module_generate_common_name](#helm_lib_module_generate_common_name) |
| **Module Https** |
| [helm_lib_module_uri_scheme](#helm_lib_module_uri_scheme) |
| [helm_lib_module_https_mode](#helm_lib_module_https_mode) |
| [helm_lib_module_https_cert_manager_cluster_issuer_name](#helm_lib_module_https_cert_manager_cluster_issuer_name) |
| [helm_lib_module_https_ingress_tls_enabled](#helm_lib_module_https_ingress_tls_enabled) |
| [helm_lib_module_https_copy_custom_certificate](#helm_lib_module_https_copy_custom_certificate) |
| [helm_lib_module_https_secret_name](#helm_lib_module_https_secret_name) |
| **Module Image** |
| [helm_lib_module_image](#helm_lib_module_image) |
| [helm_lib_module_image_no_fail](#helm_lib_module_image_no_fail) |
| [helm_lib_module_common_image](#helm_lib_module_common_image) |
| [helm_lib_module_common_image_no_fail](#helm_lib_module_common_image_no_fail) |
| **Module Ingress Class** |
| [helm_lib_module_ingress_class](#helm_lib_module_ingress_class) |
| **Module Init Container** |
| [helm_lib_module_init_container_chown_nobody_volume](#helm_lib_module_init_container_chown_nobody_volume) |
| [helm_lib_module_init_container_chown_deckhouse_volume](#helm_lib_module_init_container_chown_deckhouse_volume) |
| [helm_lib_module_init_container_check_linux_kernel](#helm_lib_module_init_container_check_linux_kernel) |
| **Module Labels** |
| [helm_lib_module_labels](#helm_lib_module_labels) |
| **Module Public Domain** |
| [helm_lib_module_public_domain](#helm_lib_module_public_domain) |
| **Module Security Context** |
| [helm_lib_module_pod_security_context_run_as_user_custom](#helm_lib_module_pod_security_context_run_as_user_custom) |
| [helm_lib_module_pod_security_context_run_as_user_nobody](#helm_lib_module_pod_security_context_run_as_user_nobody) |
| [helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs](#helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs) |
| [helm_lib_module_pod_security_context_run_as_user_deckhouse](#helm_lib_module_pod_security_context_run_as_user_deckhouse) |
| [helm_lib_module_pod_security_context_run_as_user_deckhouse_with_writable_fs](#helm_lib_module_pod_security_context_run_as_user_deckhouse_with_writable_fs) |
| [helm_lib_module_container_security_context_run_as_user_deckhouse_pss_restricted](#helm_lib_module_container_security_context_run_as_user_deckhouse_pss_restricted) |
| [helm_lib_module_pod_security_context_run_as_user_root](#helm_lib_module_pod_security_context_run_as_user_root) |
| [helm_lib_module_pod_security_context_runtime_default](#helm_lib_module_pod_security_context_runtime_default) |
| [helm_lib_module_container_security_context_not_allow_privilege_escalation](#helm_lib_module_container_security_context_not_allow_privilege_escalation) |
| [helm_lib_module_container_security_context_read_only_root_filesystem_with_selinux](#helm_lib_module_container_security_context_read_only_root_filesystem_with_selinux) |
| [helm_lib_module_container_security_context_read_only_root_filesystem](#helm_lib_module_container_security_context_read_only_root_filesystem) |
| [helm_lib_module_container_security_context_privileged](#helm_lib_module_container_security_context_privileged) |
| [helm_lib_module_container_security_context_escalated_sys_admin_privileged](#helm_lib_module_container_security_context_escalated_sys_admin_privileged) |
| [helm_lib_module_container_security_context_privileged_read_only_root_filesystem](#helm_lib_module_container_security_context_privileged_read_only_root_filesystem) |
| [helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all](#helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all) |
| [helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add](#helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add) |
| [helm_lib_module_container_security_context_capabilities_drop_all_and_add](#helm_lib_module_container_security_context_capabilities_drop_all_and_add) |
| [helm_lib_module_container_security_context_capabilities_drop_all_and_run_as_user_custom](#helm_lib_module_container_security_context_capabilities_drop_all_and_run_as_user_custom) |
| **Module Storage Class** |
| [helm_lib_module_storage_class_annotations](#helm_lib_module_storage_class_annotations) |
| **Monitoring Grafana Dashboards** |
| [helm_lib_grafana_dashboard_definitions_recursion](#helm_lib_grafana_dashboard_definitions_recursion) |
| [helm_lib_grafana_dashboard_definitions](#helm_lib_grafana_dashboard_definitions) |
| [helm_lib_single_dashboard](#helm_lib_single_dashboard) |
| **Monitoring Prometheus Rules** |
| [helm_lib_prometheus_rules_recursion](#helm_lib_prometheus_rules_recursion) |
| [helm_lib_prometheus_rules](#helm_lib_prometheus_rules) |
| [helm_lib_prometheus_target_scrape_timeout_seconds](#helm_lib_prometheus_target_scrape_timeout_seconds) |
| **Node Affinity** |
| [helm_lib_internal_check_node_selector_strategy](#helm_lib_internal_check_node_selector_strategy) |
| [helm_lib_node_selector](#helm_lib_node_selector) |
| [helm_lib_tolerations](#helm_lib_tolerations) |
| [_helm_lib_cloud_or_hybrid_cluster](#_helm_lib_cloud_or_hybrid_cluster) |
| [helm_lib_internal_check_tolerations_strategy](#helm_lib_internal_check_tolerations_strategy) |
| [_helm_lib_any_node_tolerations](#_helm_lib_any_node_tolerations) |
| [_helm_lib_wildcard_tolerations](#_helm_lib_wildcard_tolerations) |
| [_helm_lib_monitoring_tolerations](#_helm_lib_monitoring_tolerations) |
| [_helm_lib_frontend_tolerations](#_helm_lib_frontend_tolerations) |
| [_helm_lib_system_tolerations](#_helm_lib_system_tolerations) |
| [_helm_lib_additional_tolerations_uninitialized](#_helm_lib_additional_tolerations_uninitialized) |
| [_helm_lib_additional_tolerations_node_problems](#_helm_lib_additional_tolerations_node_problems) |
| [_helm_lib_additional_tolerations_storage_problems](#_helm_lib_additional_tolerations_storage_problems) |
| [_helm_lib_additional_tolerations_no_csi](#_helm_lib_additional_tolerations_no_csi) |
| [_helm_lib_additional_tolerations_cloud_provider_uninitialized](#_helm_lib_additional_tolerations_cloud_provider_uninitialized) |
| **Pod Disruption Budget** |
| [helm_lib_pdb_daemonset](#helm_lib_pdb_daemonset) |
| **Priority Class** |
| [helm_lib_priority_class](#helm_lib_priority_class) |
| **Resources Management** |
| [helm_lib_resources_management_pod_resources](#helm_lib_resources_management_pod_resources) |
| [helm_lib_resources_management_original_pod_resources](#helm_lib_resources_management_original_pod_resources) |
| [helm_lib_resources_management_vpa_spec](#helm_lib_resources_management_vpa_spec) |
| [helm_lib_resources_management_cpu_units_to_millicores](#helm_lib_resources_management_cpu_units_to_millicores) |
| [helm_lib_resources_management_memory_units_to_bytes](#helm_lib_resources_management_memory_units_to_bytes) |
| [helm_lib_vpa_kube_rbac_proxy_resources](#helm_lib_vpa_kube_rbac_proxy_resources) |
| [helm_lib_container_kube_rbac_proxy_resources](#helm_lib_container_kube_rbac_proxy_resources) |
| **Spec For High Availability** |
| [helm_lib_pod_anti_affinity_for_ha](#helm_lib_pod_anti_affinity_for_ha) |
| [helm_lib_deployment_on_master_strategy_and_replicas_for_ha](#helm_lib_deployment_on_master_strategy_and_replicas_for_ha) |
| [helm_lib_deployment_on_master_custom_strategy_and_replicas_for_ha](#helm_lib_deployment_on_master_custom_strategy_and_replicas_for_ha) |
| [helm_lib_deployment_strategy_and_replicas_for_ha](#helm_lib_deployment_strategy_and_replicas_for_ha) |

## Enable Ds Eviction

### helm_lib_prevent_ds_eviction_annotation

 Adds `cluster-autoscaler.kubernetes.io/enable-ds-eviction` annotation to manage DaemonSet eviction by the Cluster Autoscaler. 
 This is important to prevent the eviction of DaemonSet pods during cluster scaling.  

#### Usage

`{{ include "helm_lib_prevent_ds_eviction_annotation" . }} `


## Envs For Proxy

### helm_lib_envs_for_proxy

 Add HTTP_PROXY, HTTPS_PROXY and NO_PROXY environment variables for container 
 depends on [proxy settings](https://deckhouse.io/documentation/v1/deckhouse-configure-global.html#parameters-modules-proxy) 

#### Usage

`{{ include "helm_lib_envs_for_proxy" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 

## High Availability

### helm_lib_is_ha_to_value

 returns value "yes" if cluster is highly available, else — returns "no" 

#### Usage

`{{ include "helm_lib_is_ha_to_value" (list . yes no) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Yes value 
-  No value 


### helm_lib_ha_enabled

 returns empty value, which is treated by go template as false 

#### Usage

`{{- if (include "helm_lib_ha_enabled" .) }} `

#### Arguments

-  Template context with .Values, .Chart, etc 

## Kube Rbac Proxy

### helm_lib_kube_rbac_proxy_ca_certificate

 Renders configmap with kube-rbac-proxy CA certificate which uses to verify the kube-rbac-proxy clients. 

#### Usage

`{{ include "helm_lib_kube_rbac_proxy_ca_certificate" (list . "namespace") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Namespace where CA configmap will be created  

## Module Documentation Uri

### helm_lib_module_documentation_uri

 returns rendered documentation uri using publicDomainTemplate or deckhouse.io domains

#### Usage

`{{ include "helm_lib_module_documentation_uri" (list . "<path_to_document>") }} `


## Module Ephemeral Storage

### helm_lib_module_ephemeral_storage_logs_with_extra

 50Mi for container logs `log-opts.max-file * log-opts.max-size` would be added to passed value 
 returns ephemeral-storage size for logs with extra space 

#### Usage

`{{ include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 }} `

#### Arguments

-  Extra space in mebibytes 


### helm_lib_module_ephemeral_storage_only_logs

 50Mi for container logs `log-opts.max-file * log-opts.max-size` would be requested 
 returns ephemeral-storage size for only logs 

#### Usage

`{{ include "helm_lib_module_ephemeral_storage_only_logs" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 

## Module Generate Common Name

### helm_lib_module_generate_common_name

 returns the commonName parameter for use in the Certificate custom resource(cert-manager) 

#### Usage

`{{ include "helm_lib_module_generate_common_name" (list . "<name-portion>") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Name portion 

## Module Https

### helm_lib_module_uri_scheme

 return module uri scheme "http" or "https" 

#### Usage

`{{ include "helm_lib_module_uri_scheme" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_https_mode

 returns https mode for module 

#### Usage

`{{ if (include "helm_lib_module_https_mode" .) }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_https_cert_manager_cluster_issuer_name

 returns cluster issuer name  

#### Usage

`{{ include "helm_lib_module_https_cert_manager_cluster_issuer_name" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_https_ingress_tls_enabled

 returns not empty string if tls should enable for ingress  

#### Usage

`{{ if (include "helm_lib_module_https_ingress_tls_enabled" .) }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_https_copy_custom_certificate

 Renders secret with [custom certificate](https://deckhouse.io/documentation/v1/deckhouse-configure-global.html#parameters-modules-https-customcertificate) 
 in passed namespace with passed prefix 

#### Usage

`{{ include "helm_lib_module_https_copy_custom_certificate" (list . "namespace" "secret_name_prefix") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Namespace 
-  Secret name prefix 


### helm_lib_module_https_secret_name

 returns custom certificate name 

#### Usage

`{{ include "helm_lib_module_https_secret_name (list . "secret_name_prefix") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Secret name prefix 

## Module Image

### helm_lib_module_image

 returns image name 

#### Usage

`{{ include "helm_lib_module_image" (list . "<container-name>") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Container name 


### helm_lib_module_image_no_fail

 returns image name if found 

#### Usage

`{{ include "helm_lib_module_image_no_fail" (list . "<container-name>") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Container name 


### helm_lib_module_common_image

 returns image name from common module 

#### Usage

`{{ include "helm_lib_module_common_image" (list . "<container-name>") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Container name 


### helm_lib_module_common_image_no_fail

 returns image name from common module if found 

#### Usage

`{{ include "helm_lib_module_common_image_no_fail" (list . "<container-name>") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Container name 

## Module Ingress Class

### helm_lib_module_ingress_class

 returns ingress class from module settings or if not exists from global config 

#### Usage

`{{ include "helm_lib_module_ingress_class" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 

## Module Init Container

### helm_lib_module_init_container_chown_nobody_volume

 ### Migration 11.12.2020: Remove this helper with all its usages after this commit reached RockSolid 
 returns initContainer which chowns recursively all files and directories in passed volume 

#### Usage

`{{ include "helm_lib_module_init_container_chown_nobody_volume" (list . "volume-name") }} `



### helm_lib_module_init_container_chown_deckhouse_volume

 returns initContainer which chowns recursively all files and directories in passed volume 

#### Usage

`{{ include "helm_lib_module_init_container_chown_deckhouse_volume" (list . "volume-name") }} `



### helm_lib_module_init_container_check_linux_kernel

 returns initContainer which checks the kernel version on the node for compliance to semver constraint 

#### Usage

`{{ include "helm_lib_module_init_container_check_linux_kernel" (list . ">= 4.9.17") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Semver constraint 

## Module Labels

### helm_lib_module_labels

 returns deckhouse labels 

#### Usage

`{{ include "helm_lib_module_labels" (list . (dict "app" "test" "component" "testing")) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Additional labels dict 

## Module Public Domain

### helm_lib_module_public_domain

 returns rendered publicDomainTemplate to service fqdn 

#### Usage

`{{ include "helm_lib_module_public_domain" (list . "<name-portion>") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Name portion 

## Module Security Context

### helm_lib_module_pod_security_context_run_as_user_custom

 returns PodSecurityContext parameters for Pod with custom user and group 

#### Usage

`{{ include "helm_lib_module_pod_security_context_run_as_user_custom" (list . 1000 1000) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  User id 
-  Group id 


### helm_lib_module_pod_security_context_run_as_user_nobody

 returns PodSecurityContext parameters for Pod with user and group "nobody" 

#### Usage

`{{ include "helm_lib_module_pod_security_context_run_as_user_nobody" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs

 returns PodSecurityContext parameters for Pod with user and group "nobody" with write access to mounted volumes 

#### Usage

`{{ include "helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_pod_security_context_run_as_user_deckhouse

 returns PodSecurityContext parameters for Pod with user and group "deckhouse" 

#### Usage

`{{ include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_pod_security_context_run_as_user_deckhouse_with_writable_fs

 returns PodSecurityContext parameters for Pod with user and group "deckhouse" with write access to mounted volumes 

#### Usage

`{{ include "helm_lib_module_pod_security_context_run_as_user_deckhouse_with_writable_fs" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_container_security_context_run_as_user_deckhouse_pss_restricted

 returns SecurityContext parameters for Container with user and group "deckhouse" plus minimal required settings to comply with the Restricted mode of the Pod Security Standards 

#### Usage

`{{ include "helm_lib_module_container_security_context_run_as_user_deckhouse_pss_restricted" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_pod_security_context_run_as_user_root

 returns PodSecurityContext parameters for Pod with user and group 0 

#### Usage

`{{ include "helm_lib_module_pod_security_context_run_as_user_root" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_pod_security_context_runtime_default

 returns PodSecurityContext parameters for Pod with seccomp profile RuntimeDefault 

#### Usage

`{{ include "helm_lib_module_pod_security_context_runtime_default" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_container_security_context_not_allow_privilege_escalation

 returns SecurityContext parameters for Container with allowPrivilegeEscalation false 

#### Usage

`{{ include "helm_lib_module_container_security_context_not_allow_privilege_escalation" . }} `



### helm_lib_module_container_security_context_read_only_root_filesystem_with_selinux

 returns SecurityContext parameters for Container with read only root filesystem and options for SELinux compatibility

#### Usage

`{{ include "helm_lib_module_container_security_context_read_only_root_filesystem_with_selinux" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_container_security_context_read_only_root_filesystem

 returns SecurityContext parameters for Container with read only root filesystem 

#### Usage

`{{ include "helm_lib_module_container_security_context_read_only_root_filesystem" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_container_security_context_privileged

 returns SecurityContext parameters for Container running privileged 

#### Usage

`{{ include "helm_lib_module_container_security_context_privileged" . }} `



### helm_lib_module_container_security_context_escalated_sys_admin_privileged

 returns SecurityContext parameters for Container running privileged with escalation and sys_admin 

#### Usage

`{{ include "helm_lib_module_container_security_context_escalated_sys_admin_privileged" . }} `



### helm_lib_module_container_security_context_privileged_read_only_root_filesystem

 returns SecurityContext parameters for Container running privileged with read only root filesystem 

#### Usage

`{{ include "helm_lib_module_container_security_context_privileged_read_only_root_filesystem" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all

 returns SecurityContext for Container with read only root filesystem and all capabilities dropped  

#### Usage

`{{ include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add

 returns SecurityContext parameters for Container with read only root filesystem, all dropped and some added capabilities 

#### Usage

`{{ include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add"  (list . (list "KILL" "SYS_PTRACE")) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  List of capabilities 


### helm_lib_module_container_security_context_capabilities_drop_all_and_add

 returns SecurityContext parameters for Container with all dropped and some added capabilities 

#### Usage

`{{ include "helm_lib_module_container_security_context_capabilities_drop_all_and_add"  (list . (list "KILL" "SYS_PTRACE")) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  List of capabilities 


### helm_lib_module_container_security_context_capabilities_drop_all_and_run_as_user_custom

 returns SecurityContext parameters for Container with read only root filesystem, all dropped, and custom user ID 

#### Usage

`{{ include "helm_lib_module_container_security_context_capabilities_drop_all_and_run_as_user_custom" (list . 1000 1000) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  User id 
-  Group id 

## Module Storage Class

### helm_lib_module_storage_class_annotations

 return module StorageClass annotations 

#### Usage

`{{ include "helm_lib_module_storage_class_annotations" (list $ $index $storageClass.name) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Storage class index 
-  Storage class name 

## Monitoring Grafana Dashboards

### helm_lib_grafana_dashboard_definitions_recursion

 returns all the dashboard-definintions from <root dir>/ 
 current dir is optional — used for recursion but you can use it for partially generating dashboards 

#### Usage

`{{ include "helm_lib_grafana_dashboard_definitions_recursion" (list . <root dir> [current dir]) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Dashboards root dir 
-  Dashboards current dir 


### helm_lib_grafana_dashboard_definitions

 returns dashboard-definintions from monitoring/grafana-dashboards/ 

#### Usage

`{{ include "helm_lib_grafana_dashboard_definitions" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_single_dashboard

 renders a single dashboard 

#### Usage

`{{ include "helm_lib_single_dashboard" (list . "dashboard-name" "folder" $dashboard) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Dashboard name 
-  Folder 
-  Dashboard definition 

## Monitoring Prometheus Rules

### helm_lib_prometheus_rules_recursion

 returns all the prometheus rules from <root dir>/ 
 current dir is optional — used for recursion but you can use it for partially generating rules 

#### Usage

`{{ include "helm_lib_prometheus_rules_recursion" (list . <namespace> <root dir> [current dir]) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Namespace for creating rules 
-  Rules root dir 
-  Current dir (optional) 


### helm_lib_prometheus_rules

 returns all the prometheus rules from monitoring/prometheus-rules/ 

#### Usage

`{{ include "helm_lib_prometheus_rules" (list . <namespace>) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Namespace for creating rules 


### helm_lib_prometheus_target_scrape_timeout_seconds

 returns adjust timeout value to scrape interval / 

#### Usage

`{{ include "helm_lib_prometheus_target_scrape_timeout_seconds" (list . <timeout>) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Target timeout in seconds 

## Node Affinity

### helm_lib_internal_check_node_selector_strategy

 Verify node selector strategy. 



### helm_lib_node_selector

 Returns node selector for workloads depend on strategy. 

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  strategy, one of "frontend" "monitoring" "system" "master" "any-node" "wildcard" 


### helm_lib_tolerations

 Returns tolerations for workloads depend on strategy. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "any-node" "with-uninitialized" "without-storage-problems") }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  base strategy, one of "frontend" "monitoring" "system" any-node" "wildcard" 
-  list of additional strategies. To add strategy list it with prefix "with-", to remove strategy list it with prefix "without-". 


### _helm_lib_cloud_or_hybrid_cluster

 Check cluster type. 
 Returns not empty string if this is cloud or hybrid cluster 



### helm_lib_internal_check_tolerations_strategy

 Verify base strategy. 
 Fails if strategy not in allowed list 



### _helm_lib_any_node_tolerations

 Base strategy for any uncordoned node in cluster. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "any-node") }} `



### _helm_lib_wildcard_tolerations

 Base strategy that tolerates all. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "wildcard") }} `



### _helm_lib_monitoring_tolerations

 Base strategy that tolerates nodes with "dedicated.deckhouse.io: monitoring" and "dedicated.deckhouse.io: system" taints. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "monitoring") }} `



### _helm_lib_frontend_tolerations

 Base strategy that tolerates nodes with "dedicated.deckhouse.io: frontend" taints. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "frontend") }} `



### _helm_lib_system_tolerations

 Base strategy that tolerates nodes with "dedicated.deckhouse.io: system" taints. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "system") }} `



### _helm_lib_additional_tolerations_uninitialized

 Additional strategy "uninitialized" - used for CNI's and kube-proxy to allow cni components scheduled on node after CCM initialization. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "any-node" "with-uninitialized") }} `



### _helm_lib_additional_tolerations_node_problems

 Additional strategy "node-problems" - used for shedule critical components on non-ready nodes or nodes under pressure. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "any-node" "with-node-problems") }} `



### _helm_lib_additional_tolerations_storage_problems

 Additional strategy "storage-problems" - used for shedule critical components on nodes with drbd problems. This additional strategy enabled by default in any base strategy except "wildcard". 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "any-node" "without-storage-problems") }} `



### _helm_lib_additional_tolerations_no_csi

 Additional strategy "no-csi" - used for any node with no CSI: any node, which was initialized by deckhouse, but have no csi-node driver registered on it. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "any-node" "with-no-csi") }} `



### _helm_lib_additional_tolerations_cloud_provider_uninitialized

 Additional strategy "cloud-provider-uninitialized" - used for any node which is not initialized by CCM. 

#### Usage

`{{ include "helm_lib_tolerations" (tuple . "any-node" "with-cloud-provider-uninitialized") }} `


## Pod Disruption Budget

### helm_lib_pdb_daemonset

 Returns PDB max unavailable 

#### Usage

`{{ include "helm_lib_pdb_daemonset" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 

## Priority Class

### helm_lib_priority_class

 returns priority class if priority-class module enabled, otherwise returns nothing 

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Priority class name 

## Resources Management

### helm_lib_resources_management_pod_resources

 returns rendered resources section based on configuration if it is 

#### Usage

`{{ include "helm_lib_resources_management_pod_resources" (list <resources configuration> [ephemeral storage requests]) }} `

#### Arguments

list:
-  VPA resource configuration [example](https://deckhouse.io/documentation/v1/modules/110-istio/configuration.html#parameters-controlplane-resourcesmanagement) 
-  Ephemeral storage requests 


### helm_lib_resources_management_original_pod_resources

 returns rendered resources section based on configuration if it is present 

#### Usage

`{{ include "helm_lib_resources_management_original_pod_resources" <resources configuration> }} `

#### Arguments

-  VPA resource configuration [example](https://deckhouse.io/documentation/v1/modules/110-istio/configuration.html#parameters-controlplane-resourcesmanagement) 


### helm_lib_resources_management_vpa_spec

 returns rendered vpa spec based on configuration and target reference 

#### Usage

`{{ include "helm_lib_resources_management_vpa_spec" (list <target apiversion> <target kind> <target name> <target container> <resources configuration> ) }} `

#### Arguments

list:
-  Target API version 
-  Target Kind 
-  Target Name 
-  Target container name 
-  VPA resource configuration [example](https://deckhouse.io/documentation/v1/modules/110-istio/configuration.html#parameters-controlplane-resourcesmanagement) 


### helm_lib_resources_management_cpu_units_to_millicores

 helper for converting cpu units to millicores 

#### Usage

`{{ include "helm_lib_resources_management_cpu_units_to_millicores" <cpu units> }} `



### helm_lib_resources_management_memory_units_to_bytes

 helper for converting memory units to bytes 

#### Usage

`{{ include "helm_lib_resources_management_memory_units_to_bytes" <memory units> }} `



### helm_lib_vpa_kube_rbac_proxy_resources

 helper for VPA resources for kube_rbac_proxy 

#### Usage

`{{ include "helm_lib_vpa_kube_rbac_proxy_resources" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_container_kube_rbac_proxy_resources

 helper for container resources for kube_rbac_proxy 

#### Usage

`{{ include "helm_lib_container_kube_rbac_proxy_resources" . }} `

#### Arguments

-  Template context with .Values, .Chart, etc 

## Spec For High Availability

### helm_lib_pod_anti_affinity_for_ha

 returns pod affinity spec 

#### Usage

`{{ include "helm_lib_pod_anti_affinity_for_ha" (list . (dict "app" "test")) }} `

#### Arguments

list:
-  Template context with .Values, .Chart, etc 
-  Match labels for podAntiAffinity label selector 


### helm_lib_deployment_on_master_strategy_and_replicas_for_ha

 returns deployment strategy and replicas for ha components running on master nodes 

#### Usage

`{{ include "helm_lib_deployment_on_master_strategy_and_replicas_for_ha" }} `

#### Arguments

-  Template context with .Values, .Chart, etc 


### helm_lib_deployment_on_master_custom_strategy_and_replicas_for_ha

 returns deployment with custom strategy and replicas for ha components running on master nodes 

#### Usage

`{{ include "helm_lib_deployment_on_master_custom_strategy_and_replicas_for_ha" (list . (dict "strategy" "strategy_type")) }} `



### helm_lib_deployment_strategy_and_replicas_for_ha

 returns deployment strategy and replicas for ha components running not on master nodes 

#### Usage

`{{ include "helm_lib_deployment_strategy_and_replicas_for_ha" }} `

#### Arguments

-  Template context with .Values, .Chart, etc 
