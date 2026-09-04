---
title: "The multitenancy-manager module: FAQ"
---

## Managing access to cluster-wide resources

### What should I do if a PersistentVolumeClaim is rejected with an "is not available to project" error?

The StorageClass specified in `spec.storageClassName` is unavailable to the project. The webhook rejects such a request with a message similar to the following:

```text
[multitenancy] PersistentVolumeClaim "<OBJECT_NAME>" references "<RESOURCE_NAME>" which is not available to project "<PROJECT_NAME>". Ask the cluster administrator to grant it.
```

To view the available StorageClasses, run the following command:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

If the required StorageClass is not in the list, use an available one or ask the cluster administrator to add it to the [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy).

You can check the availability of other cluster-wide resources, such as ClusterIssuer, ClusterRole, and LoadBalancerClass, in the same way using the corresponding [AvailableClusterResource](cr.html#availableclusterresource).

### How can I view access policies applied to a project?

A [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy) applies to a project if its `spec.projectSelector` matches the labels of the project namespaces.

To view all policies and their selectors, run the following command:

```shell
d8 k get clusterresourcegrantpolicies -o yaml
```

You can view the resulting list of cluster-wide resources available to a specific project using [AvailableClusterResource](cr.html#availableclusterresource):

```shell
d8 k get available -n <PROJECT_NAME>
```

### What happens to existing objects after access is restricted?

Existing objects continue to use their previously configured values. When an object is updated, only new values are checked, so changing an access policy does not disrupt existing objects.

If an existing object needs to use a different cluster-wide resource, explicitly change the corresponding field to a value available to the project.

If an existing object uses a cluster-wide resource that is no longer available to the project, the [`ClusterResourceGrantPolicyViolation`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation) alert is triggered.

### What does the ClusterResourceGrantPolicyViolation alert mean?

The alert indicates that one or more existing objects in a project use a cluster-wide resource that is no longer available to the project under the current policies.

Such objects continue to operate. To resolve the discrepancy, grant the project access to the cluster-wide resource in use or modify the object to use an available resource.

Detailed information about violations is available on the Grafana dashboard under "Security" → "Cluster Resource Grant Violations". The `d8_cluster_objects_grant_violated` metric is used for monitoring.

### How can I allow all StorageClasses except specific ones?

To deny specific StorageClasses while keeping the others available, use [`denied`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-denied) or [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-deniedselector). Deny rules take precedence over allow rules.

For a configuration example, refer to ["Denying individual resources"](usage.html#denying-individual-resources).

### What happens if there is no ClusterResourceGrantPolicy?

If no ClusterResourceGrantPolicy is configured for a resource, its availability is determined by its registration. The resource is available to all projects if [`defaultAvailability: All`](cr.html#grantableclusterresourcedefinition-v1alpha1-spec-defaultavailability) (the default value) is set in GrantableClusterResourceDefinition and the resource does not match the [`excluded`](cr.html#grantableclusterresourcedefinition-v1alpha1-spec-excluded) filters.

For example, the `clusterroles` definition provided by DKP excludes all ClusterRoles without the `rbac.deckhouse.io/delegatable` label. Therefore, such roles are unavailable in RoleBinding even when no policies are configured.

To restrict access, create a [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy) and specify the projects and the cluster-wide resources available to them.

### I created a ClusterResourceGrantPolicy, but nothing changed — why?

Check the following:

1. **Project selector match**. Make sure the policy's [`spec.projectSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-projectselector) matches the labels of the project namespace:

   ```shell
   d8 k get ns <PROJECT_NAME> --show-labels
   ```

1. **Resources available to the project**. Check whether AvailableClusterResource reflects the expected policy settings:

   ```shell
   d8 k get available <RESOURCE_NAME> -n <PROJECT_NAME> -o yaml
   ```

1. **Cluster-wide resource registration**. Make sure a corresponding [GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition) exists for the `resourceName` value.

1. **Reference registration**. If a specific field is expected to be checked, make sure it is registered using [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference) and that the reference is successfully associated with the corresponding GrantableClusterResourceDefinition.

For information about registration status, refer to ["Checking resource registration status"](usage.html#checking-resource-registration-status).

### How does cluster-wide resource access management interact with RBAC?

These mechanisms operate independently. RBAC determines *who can perform* operations on an object, while cluster-wide resource access management determines *which cluster-wide resources* that object can use.

For example, to create a PersistentVolumeClaim, a user must have the appropriate RBAC permissions, and the StorageClass specified in `.spec.storageClassName` must be available to the project.

### What happens if the multitenancy-manager module is disabled?

After the module is disabled, cluster-wide resource availability checks and automatic default value assignment are no longer performed. Existing objects remain unchanged.

Fields that use the `x-deckhouse-grantable-resource` extension in DKP application settings are no longer checked either.

The `d8:use:dict` role in the `user-authz` module continues to work.

### What happens when access management is enabled on an existing cluster?

Existing objects continue to use their previously configured values. Access checks apply to new objects and, when existing objects are updated, only to new values.

Therefore, existing objects do not need to be modified before access policies are created.

### What happens to objects created before access was restricted?

Such objects continue to use their previously configured values. New objects and new values in the fields of existing objects are checked against the current access policies.

If an existing object uses a cluster-wide resource that is no longer available to the project, the [`ClusterResourceGrantPolicyViolation`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation) alert is triggered.
