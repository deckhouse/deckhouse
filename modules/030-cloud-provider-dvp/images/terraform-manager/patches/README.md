# 001-go-mod.patch
cve fixes

# 002-add-config-data-base64.patch
Add argument config_data_base64 to the provider configuration
Add RestMapper provider to provider metadata

# 003-add-resource-ready-resource.patch
Add kubernetes_resource_ready_v1 resource for checking another resource ready.
kubernetes provider has `wait` block, but we have bad situation.
Provider creates resource (resource now present in cluster) but if resource not 
ready with wait block, provider returns error and terraform does not save resource
in state. Now, we have situation when we cannot revert or 
in some cases recreate resource automatically and client should use manual actions
for reverts and restarts, especially in commander.
Also, this patch contains huge testing for new resource. For testing, we can use
`./run_resource_ready_tests.sh` because: 
- unfortunately  parallel tests cannot work with panic in testing framework internal. 
  It is uncomfortable with running tests in IDE's
- script contains some initialization for run tests with `kind` cluster.
