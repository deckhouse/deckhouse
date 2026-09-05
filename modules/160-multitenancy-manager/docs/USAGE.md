---
title: "The multitenancy-manager module: usage examples"
---
{% raw %}

## Default project templates

The following project templates are included in the Deckhouse Kubernetes Platform:

- `default` — a template that covers basic project use cases:
  - resource limitation
  - network isolation
  - automatic alerts and log collection
  - choice of security profile
  - project administrators setup

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/default.yaml).

- `secure` — includes all the capabilities of the `default` template and additional features:
  - setting up permissible UID/GID for the project
  - audit rules for project users' access to the Linux kernel
  - scanning of launched container images for CVE presence

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure.yaml).

- `secure-with-dedicated-nodes` — includes all the capabilities of the `secure` template and additional features:
  - defining the node selector for all the pods in the project: if a pod is created, the node selector pod will be **substituted** with the project's node selector automatically.
  - defining the default toleration for all the pods in the project: if a pod is created, the default toleration will be **added** to the pod automatically.

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure-with-dedicated-nodes.yaml).

To list all available parameters for a project template, execute the command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Creating a project

1. To create a project, create the [Project](cr.html#project) resource by specifying the name of the project template in [.spec.projectTemplateName](cr.html#project-v1alpha2-spec-projecttemplatename) field.
1. In the [.spec.parameters](cr.html#project-v1alpha2-spec-parameters) field of the Project resource, specify the parameter values suitable for the ProjectTemplate [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema).

   Example of creating a project using the [Project](cr.html#project) resource from the `default` [ProjectTemplate](cr.html#projecttemplate):

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example from the Deckhouse documentation.
     projectTemplateName: default
     parameters:
       resourceQuota:
         requests:
           cpu: 5
           memory: 5Gi
           storage: 1Gi
         limits:
           cpu: 5
           memory: 5Gi
       networkPolicy: Isolated
       podSecurityProfile: Restricted
       extendedMonitoringEnabled: true
       administrators:
       - subject: Group
         name: k8s-admins
   ```

1. To check the status of the project, execute the command:

   ```shell
   d8 k get projects my-project
   ```

   A successfully created project should be in the `Deployed` state. If the state equals `Error`, add the `-o yaml` argument to the command (e.g., `d8 k get projects my-project -o yaml`) to get more detailed information about the error.

### Creating a project automatically for a namespace

You can create a new project for a namespace. To do this, add the `projects.deckhouse.io/adopt` annotation to the namespace. For example:

1. Create a new namespace:

   ```shell
   d8 k create ns test
   ```

1. Add the annotation:

   ```shell
   d8 k annotate ns test projects.deckhouse.io/adopt=""
   ```

1. Make sure that the project was created:

   ```shell
   d8 k get projects
   ```

   A new project corresponding to the namespace will appear in the project list:

   ```shell
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   empty                                                                     1m
   ```

You can change the template of the created project to the existing one.

{% endraw %}

{% alert level="warning" %}
Note that changing the template may cause a resource conflict. If the template chart contains resources that are already present in the namespace, you will not be able to apply the template.
{% endalert %}

{% raw %}

## Creating your own project template

Default templates cover basic project use cases and serve as a good example of template capabilities.

To create your own template:

1. Take one of the default templates as a basis, for example, `default`.
1. Copy it to a separate file, for example, `my-project-template.yaml` using the command:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Edit the `my-project-template.yaml` file, make the necessary changes.

   {% alert level="info" %}
   You must update not only the template itself, but also the input parameters schema to match it.

   Project templates support all [Helm templating functions](https://helm.sh/docs/chart_template_guide/function_list/).
   {% endalert %}

1. Change the template name in the `.metadata.name` field.
1. Apply your new template with the command:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Check the availability of the new template with the command:

   ```shell
   d8 k get projecttemplates <NEW_TEMPLATE_NAME>
   ```

{% endraw %}

## Using labels to manage resources

When creating resources in `ProjectTemplate`, you can use special labels to control how the `multitenancy-manager` processes these resources.

### Skipping creation of the `heritage: multitenancy-manager` label

By default, all resources created from `ProjectTemplate` receive the label `heritage: multitenancy-manager`.  
This label prohibits changes to resources by users or any other controller except `multitenancy-manager`.  
If you need to allow resource modification (for example, for compatibility with other systems, or if implementing your own control over the created objects), add the label `projects.deckhouse.io/skip-heritage-label` to the resource.

Example:

{% raw %}

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: {{ .projectName }}
  labels:
    projects.deckhouse.io/skip-heritage-label: "true"
    app: my-app
data:
  key: value
```

{% endraw %}

In this case, the resource will receive the labels `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template`, but will not receive the label `heritage: multitenancy-manager`.

### Excluding resources from management by multitenancy-manager

If you need to exclude a resource from management by `multitenancy-manager` (for example, if the resource should be managed manually or by another controller), add the label `projects.deckhouse.io/unmanaged` to the resource.

Example:

{% raw %}

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: external-secret
  namespace: {{ .projectName }}
  labels:
    projects.deckhouse.io/unmanaged: "true"
type: Opaque
data:
  token: <base64-encoded-value>
```

{% endraw %}

Resources with the label `projects.deckhouse.io/unmanaged`:

- Will be created **only once** when the project is created;
- **Will not be updated** with subsequent template changes or updates;
- Will not be monitored in the project's status;
- Will receive the labels `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template` but **will not receive** the label `heritage: multitenancy-manager`.

{% alert level="warning" %}
Once a resource is marked as `unmanaged`, it will be created on initial installation but not updated when the `ProjectTemplate` is changed.  
After creation, the resource becomes fully independent and must be managed manually.
{% endalert %}

## Implementing validation of object changes with a custom label

The `multitenancy-manager` module uses `ValidatingAdmissionPolicy` to protect resources labeled `heritage: multitenancy-manager` from manual changes.  
You can implement similar validation for resources with any label.

### How validation works in multitenancy-manager

Validation occurs for objects labeled `heritage: multitenancy-manager`.  
The following components are used for this:

1. `ValidatingAdmissionPolicy`: Defines validation rules:
   - Operations: `UPDATE` and `DELETE`.
   - Check: only operations on behalf of the controller's service account are allowed.
   - Applies to all resources and API groups.
1. `ValidatingAdmissionPolicyBinding`: Defines which objects the validation applies to:
   - Uses `namespaceSelector` and `objectSelector` to select resources by the label `heritage: multitenancy-manager`.

### Creating your own validation

To implement validation for resources with a different label (for example, `heritage: my-custom-label`):

1. Create a file with the ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding resource manifests:

   ```yaml
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicy
   metadata:
     name: my-custom-label-validation
   spec:
     failurePolicy: Fail
     matchConstraints:
       resourceRules:
         - apiGroups:   ["*"]
           apiVersions: ["*"]
           operations:  ["UPDATE", "DELETE"]
           resources:   ["*"]
           scope: "*"
     validations:
       - expression: 'request.userInfo.username == "system:serviceaccount:my-namespace:my-service-account"' # Replace with your service account
         reason: Forbidden
         messageExpression: 'object.kind == ''Namespace'' ? ''This resource is managed by '' + object.metadata.name + '' system. Manual modification is forbidden.''
           : ''This resource is managed by '' + object.metadata.namespace + '' system. Manual modification is forbidden.'''
   ---
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicyBinding
   metadata:
     name: my-custom-label-validation
   spec:
     policyName: my-custom-label-validation
     validationActions: [Deny, Audit]
     matchResources:
       namespaceSelector:
         matchLabels:
           heritage: my-custom-label
       objectSelector:
         matchLabels:
           heritage: my-custom-label
   ```

1. Configure the validation parameters:

   - `policyName`: Unique policy name (must match in Policy and Binding).
   - `request.userInfo.username`: The name of the service account allowed to change resources (replace with your service account).
   - `heritage: my-custom-label`: The value of the `heritage` label for your resources (replace with your value). The use of the values `multitenancy-manager`, `deckhouse` is prohibited.
   - `failurePolicy: Fail`: Policy on validation failure.
     - `Fail`: Reject the request on validation failure.
     - `Ignore`: Ignore validation errors.
   - `validationActions`: Validation actions:
     - `Deny`: Deny unauthorized operations.
     - `Audit`: Record operations in the audit log.
1. Apply the policy:

   ```shell
   d8 k apply -f my-validation-policy.yaml
   ```

1. Ensure your resources have the corresponding `heritage` label:

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: my-resource
     labels:
       heritage: my-custom-label
   ```

## Managing access to cluster-wide resources

The module allows you to manage project access to cluster-wide resources such as StorageClass, ClusterIssuer, ClusterRole, and others.

For a description of the mechanism, the resources it uses, and the cluster-wide resources registered by the platform, refer to the [module description](./#managing-access-to-cluster-wide-resources).

The following sections provide common scenarios for configuring and using the mechanism.

### For cluster administrators

The following examples show how to configure project access to cluster-wide resources using ClusterResourceGrantPolicy.

#### Restricting StorageClass for a project

To allow projects to use only the `fast-ssd` and `standard` StorageClasses and use `fast-ssd` by default, create the following [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production-storage
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
    - resourceName: storageclasses
      default: fast-ssd
      allowed:
        - fast-ssd
        - standard
```

{% endraw %}

When a PersistentVolumeClaim is created without a value in `spec.storageClassName`, `fast-ssd` is automatically assigned to this field. If a StorageClass that is not in the allowed list is specified, the PersistentVolumeClaim is rejected.

StorageClass uses the [`Coerce`](cr.html#grantableclusterresourcereference-v1alpha1-spec-fieldpaths-defaulting) default assignment mode. If the built-in Kubernetes admission controller has already assigned a default class to `spec.storageClassName` that is unavailable to the project, the value is replaced with `fast-ssd` instead of rejecting the PersistentVolumeClaim.

To check which StorageClasses are available to the project, run the following command:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

#### Restricting ClusterIssuer for a project

To allow projects to use only the `letsencrypt-prod` and `vault-issuer` ClusterIssuers and use `letsencrypt-prod` by default, create the following [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production-issuers
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
    - resourceName: clusterissuers
      default: letsencrypt-prod
      allowed:
        - letsencrypt-prod
        - vault-issuer
```

{% endraw %}

The policy applies to a ClusterIssuer specified in either of the following ways:

- In the `.spec.issuerRef.name` field of a Certificate if `.spec.issuerRef.kind` is set to ClusterIssuer.
- In the `cert-manager.io/cluster-issuer` annotation of an Ingress.

When a Certificate with a ClusterIssuer is created without a value in `.spec.issuerRef.name`, `letsencrypt-prod` is automatically assigned to this field. If a ClusterIssuer that is not in the allowed list is specified, the Certificate is rejected.

No default value is assigned to the `cert-manager.io/cluster-issuer` annotation. If the annotation is specified, its value is checked against the same list of allowed ClusterIssuers.

{% alert level="info" %}
Access management for ClusterIssuer is available only when the [`cert-manager`](/modules/cert-manager/) module is enabled.
{% endalert %}

#### Granting access to additional ClusterRoles

By default, only ClusterRoles with the `rbac.deckhouse.io/delegatable` label can be used in RoleBinding.

To allow projects belonging to the `payments` team to use additional ClusterRoles, create the following [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: extra-roles
spec:
  projectSelector:
    matchLabels:
      team: payments
  resources:
    - resourceName: clusterroles
      allowed:
        - my-custom-role
      allowedSelector:
        matchLabels:
          shared: "true"
```

{% endraw %}

The policy additionally allows the following ClusterRoles:

- The ClusterRole named `my-custom-role`
- ClusterRoles matching the `shared: "true"` selector

ClusterRoles with the `rbac.deckhouse.io/delegatable` label remain available.

When a RoleBinding is created or modified, the ClusterRole specified in it is checked for availability to the project. No ClusterRole value is assigned automatically.

#### Restricting LoadBalancerClass for a Service

To allow projects to use only the `internal-lb` and `edge-lb` LoadBalancerClass values and use `internal-lb` by default, create the following [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: lb-classes
spec:
  projectSelector:
    matchLabels:
      environment: staging
  resources:
    - resourceName: loadbalancerclasses
      default: internal-lb
      allowed:
        - internal-lb
        - edge-lb
```

{% endraw %}

Unlike StorageClass, ClusterIssuer, and ClusterRole, LoadBalancerClass is not a separate Kubernetes resource. The policy defines the allowed values of the `.spec.loadBalancerClass` field for Services of the `LoadBalancer` type.

When a Service of the `LoadBalancer` type is created without a value in `.spec.loadBalancerClass`, `internal-lb` is automatically assigned to this field. If a value that is not in the allowed list is specified, the Service is rejected.

The policy does not apply to Services of other types.

#### Granting access to all resources of a specific type

To allow specific projects to use all resources of a selected type without listing them explicitly, set [`availabilityDefault: All`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-availabilitydefault).

The following policy allows all projects with the `environment: sandbox` label to use any StorageClass:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: open-storage-for-sandbox
spec:
  projectSelector:
    matchLabels:
      environment: sandbox
  resources:
    - resourceName: storageclasses
      availabilityDefault: All
```

{% endraw %}

Usually, explicitly specifying allowed resources using [`allowed`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowed) or [`allowedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowedselector) is sufficient for access management. Use `availabilityDefault: All` when the selected projects need access to all resources of the specified type.

#### Denying individual resources

To prevent projects from using individual resources while keeping the others available, use [`denied`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-denied) or [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-deniedselector).

The following policy prevents projects with the `environment: dev` label from using the `expensive-nvme` and `archived-hdd` StorageClasses:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: deny-expensive-storage
spec:
  projectSelector:
    matchLabels:
      environment: dev
  resources:
    - resourceName: storageclasses
      denied:
        - expensive-nvme
        - archived-hdd
```

{% endraw %}

Deny rules take precedence over allow rules. If a resource matches both `denied` or `deniedSelector` and `allowed` or `allowedSelector`, it is considered unavailable.

#### Managing access using label selectors

To manage access to resources without listing their names, use [`allowedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowedselector) and [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-deniedselector). Selectors allow or deny resources based on their labels.

The following policy allows projects with the `tier: shared` label to use StorageClasses with the `shared: "true"` label, except for StorageClasses with the `deprecated: "true"` label:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: shared-storage-only
spec:
  projectSelector:
    matchLabels:
      tier: shared
  resources:
    - resourceName: storageclasses
      allowedSelector:
        matchLabels:
          shared: "true"
      deniedSelector:
        matchLabels:
          deprecated: "true"
```

{% endraw %}

### For project users

Project users can view the cluster-wide resources available to them and check the values used by default.

#### Viewing available cluster-wide resources

An [AvailableClusterResource](cr.html#availableclusterresource) is automatically created in each project namespace for every registered resource. These resources show which cluster-wide resources are available to the project and which resource is used by default.

To view all cluster-wide resources available to a project, run the following command:

```shell
d8 k get available -n <PROJECT_NAME>
```

Example output:

```text
NAME                KIND          DEFAULT      AVAILABLE   AGE
storageclasses      StorageClass  fast-ssd     2           5m
clusterissuers      ClusterIssuer letsencrypt  2           5m
```

To view detailed information about resources of a specific type, such as StorageClass, run the following command:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

#### Rejection due to an unavailable cluster-wide resource

If the specified cluster-wide resource is unavailable to the project when an object is created or modified, the operation is rejected with the following message:

```text
resource <RESOURCE_NAME> is not available to project <PROJECT_NAME>
```

In this case, check the available resources using AvailableClusterResource. Use a resource from the list of available resources or ask the cluster administrator to grant access to the required resource.

#### Assigning default values automatically

For some cluster-wide resources, the administrator can configure a default value. If the corresponding value is not specified when an object is created, it is assigned automatically.

For example, if `fast-ssd` is configured as the default StorageClass, the `fast-ssd` value can be automatically assigned to `.spec.storageClassName` when a PersistentVolumeClaim is created without this field.

You can specify a value explicitly by selecting any resource available to the project from the corresponding AvailableClusterResource.

### For module developers

#### Configuring validation of a cluster-wide resource reference

If a module resource contains a field that references a cluster-wide resource already registered using [GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition), create a [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference). It defines which resources and fields use the cluster-wide resource and allows you to configure availability checks and automatic default value assignment.

For example, to configure validation of the StorageClass specified in the `.spec.storageClassName` field of a PostgresDatabase resource, add the following resource to the module's Helm chart:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: postgresdatabases-storageclasses
  labels:
    heritage: deckhouse
    module: postgres
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups: ["postgres.example.com"]
    apiVersions: ["*"]
    resources: ["postgresdatabases"]
  fieldPaths:
    - path: $.spec.storageClassName
      defaulting: Coerce
```

{% endraw %}

In this example, `storageclasses` is the name of an existing GrantableClusterResourceDefinition, while `Coerce` allows the default StorageClass available to the project to be assigned when a PostgresDatabase is created if the value is missing or unavailable to the project.

For descriptions of GrantableClusterResourceReference parameters, default assignment modes, `match` conditions, and configuration for resources with multiple API versions, refer to the [resource description](cr.html#grantableclusterresourcereference).

#### Registering a new cluster-wide resource

To add access management for a new cluster-wide resource:

1. Create a [GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition) to register the cluster-wide resource with the access management mechanism.
1. Create one or more [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference) resources to define the fields that reference it.

   For example, to register the MyClusterResource resource, add the following resource to the module's Helm chart:

   {% raw %}

   ```yaml
   apiVersion: multitenancy.deckhouse.io/v1alpha1
   kind: GrantableClusterResourceDefinition
   metadata:
     name: myclusterresources
     labels:
       heritage: deckhouse
       module: my-module
   spec:
     grantedResource:
       apiGroup: my.example.com
       kind: MyClusterResource
     enforcement: Managed
     defaultAvailability: All
     excluded:
       - matchExpressions:
           - key: my.example.com/internal
             operator: Exists
   ```

   {% endraw %}

   In this example, the registered resources are available to projects by default. Resources with the `my.example.com/internal` label are excluded from the available resources.

   For descriptions of GrantableClusterResourceDefinition parameters and available access management modes, refer to the [resource description](cr.html#grantableclusterresourcedefinition).

1. After registering the cluster-wide resource, configure references to it using GrantableClusterResourceReference as described in ["Configuring validation of a cluster-wide resource reference"](#configuring-validation-of-a-cluster-wide-resource-reference).

#### Using x-deckhouse-grantable-resource in DKP application settings

To manage access to cluster-wide resources in DKP application settings, use the `x-deckhouse-grantable-resource` OpenAPI extension. In this case, deckhouse-controller automatically checks the availability of the specified resource and assigns the default value when necessary. You do not need to create GrantableClusterResourceReference manually.

For a description of the extension and usage examples, refer to ["Application development"](/products/kubernetes-platform/documentation/v1/architecture/marketplace/application-development.html#defaulting-from-cluster-resource-grants-x-deckhouse-grantable-resource).

#### Checking resource registration status

You can check the status of GrantableClusterResourceDefinition and its associated GrantableClusterResourceReference resources in the `status` field:

- [`GrantableClusterResourceDefinition.status.references`](cr.html#grantableclusterresourcedefinition-v1alpha1-status-references): Contains a list of associated GrantableClusterResourceReference resources and information about the resources to which they apply.
- [`GrantableClusterResourceReference.status.bound`](cr.html#grantableclusterresourcereference-v1alpha1-status-bound): Indicates whether the corresponding GrantableClusterResourceDefinition was found.
- `GrantableClusterResourceReference.status.conditions[Bound]`: Contains the binding status: `Resolved` if the definition was found, or `UnknownResource` if it is missing. The `UnknownResource` status can indicate an incorrect GrantableClusterResourceDefinition name or a missing registration.
