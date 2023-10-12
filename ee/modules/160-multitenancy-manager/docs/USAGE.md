---
title: "The multitenancy-manager module: usage"
---
{% raw %}

## Creating an isolated environment

Follow these steps to create an isolated environment in a kubernetes cluster:

1. Create two [static users](../150-user-authn/usage.html#an-example-of-creating-a-static-user) who need to be given access to the isolated environment:

   Create a `users.yaml` file with the following contents (CR `User`):

   ```yaml
   # users.yaml
   ---
   apiVersion: deckhouse.io/v1
   kind: User
   metadata:
     name: user
   spec:
     email: user@cluster
     # passwordUser
     password: $2a$10$yROPLTTMTI.AkkAskKGiUuQW3asoGosGgppj1NYXUboHx/onpGE7q
   ---
   apiVersion: deckhouse.io/v1
   kind: User
   metadata:
     name: admin
   spec:
     email: admin@cluster
     # passwordAdmin
     password: $2a$10$UpCxQCpMqJoVm53BvUyPluprS/mUtJ/yUoSuM8i3Z0TlbiBxGiB1q
   ```

   Run the following command to create users:

   ```shell
   kubectl create -f users.yaml
   ```

   Check that the users have been created successfully by running the following command:

   ```shell
   kubectl get users.deckhouse.io
   ```

   Below is an example of its output:

   ```shell
   NAME    EMAIL           GROUPS   EXPIRE_AT
   admin   admin@cluster
   user    user@cluster
   ```

1. Create an environment template using the [ProjectType](cr.html#projecttype) custom resource:

   - in the [.spec.subjects](cr.html#projecttype-v1alpha1-spec-subjects) field, describe [roles](../140-user-authz/cr.html#authorizationrule-v1alpha1-spec-accesslevel) to be given to users/groups/`ServiceAccount`s;
   - in the [.spec.resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate) field, describe the resource templates that you want to create when setting up isolated environments;
   - in the [.spec.openAPI](cr.html#projecttype-v1alpha1-spec-openapi) field, define the OpenAPI specification for `values` used in the template ([.spec.resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate));
   - in the [.spec.namespaceMetadata](cr.html#projecttype-v1alpha1-spec-namespacemetadata) field, describe labels and annotations that need to be set for the namespace when setting up the environment.

   In the example below, the [.spec.subjects](cr.html#projecttype-v1alpha1-spec-subjects) field of the template contains [roles](../150-user-authn/cr.html#user) to be assigned to the users created above in the new environments. The [.spec.resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate) field contains three resources: `NetworkPolicy` (limits network accessibility of Pods outside the created namespace, except for the `kube-dns`), `LimitRange` and `ResourceQuota`. The resource template uses the parameters described in the [.spec.openAPI](cr.html#projecttype-v1alpha1-spec-openapi) field (`requests.cpu`, `requests.memory`, `requests.storage`, `limits.cpu`, `limit.memory`).

   Create a `project-type.yaml` file with the following contents (CR `ProjectType`):

   ```yaml
   # project-type.yaml
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: ProjectType
   metadata:
     name: test-project-type
   spec:
     subjects:
       - kind: User
         name: admin@cluster
         role: Admin
       - kind: User
         name: user@cluster
         role: User
     namespaceMetadata:
       annotations:
         extended-monitoring.deckhouse.io/enabled: ""
       labels:
         created-from-project-type: test-project-type
     openAPI:
       requests:
         type: object
         properties:
           cpu:
             oneOf:
               - type: number
                 format: int
               - type: string
             pattern: "^[0-9]+m?$"
           memory:
             oneOf:
               - type: number
                 format: int
               - type: string
             pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
           storage:
             type: string
             pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
       limits:
         type: object
         properties:
           cpu:
             oneOf:
               - type: number
                 format: int
               - type: string
             pattern: "^[0-9]+m?$"
           memory:
             oneOf:
               - type: number
                 format: int
               - type: string
             pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
     resourcesTemplate: |
       ---
       # Max requests and limits for resource and storage consumption for all pods in a namespace.
       # Refer to https://kubernetes.io/docs/concepts/policy/resource-quotas/
       apiVersion: v1
       kind: ResourceQuota
       metadata:
         name: all-pods
       spec:
         hard:
           {{ with .params.requests.cpu }}requests.cpu: {{ . }}{{ end }}
           {{ with .params.requests.memory }}requests.memory: {{ . }}{{ end }}
           {{ with .params.requests.storage }}requests.storage: {{ . }}{{ end }}
           {{ with .params.limits.cpu }}limits.cpu: {{ . }}{{ end }}
           {{ with .params.limits.memory }}limits.memory: {{ . }}{{ end }}
       ---
       # Max requests and limits for resource consumption per pod in a namespace.
       # All containers in a namespace must have requests and limits specified.
       # Refer to https://kubernetes.io/docs/concepts/policy/limit-range/
       apiVersion: v1
       kind: LimitRange
       metadata:
         name: all-containers
       spec:
         limits:
           - max:
               {{ with .params.limits.cpu }}cpu: {{ . }}{{ end }}
               {{ with .params.limits.memory }}memory: {{ . }}{{ end }}
             maxLimitRequestRatio:
               cpu: 1
               memory: 1
             type: Container
       ---
       # Deny all network traffic by default except namespaced traffic and dns.
       # Refer to https://kubernetes.io/docs/concepts/services-networking/network-policies/
       kind: NetworkPolicy
       apiVersion: networking.k8s.io/v1
       metadata:
         name: deny-all-except-current-namespace
       spec:
         podSelector:
           matchLabels: {}
         policyTypes:
           - Ingress
           - Egress
         ingress:
           - from:
               - namespaceSelector:
                   matchLabels:
                     kubernetes.io/metadata.name: "{{ .projectName }}"
         egress:
           - to:
               - namespaceSelector:
                   matchLabels:
                     kubernetes.io/metadata.name: "{{ .projectName }}"
           - to:
               - namespaceSelector:
                   matchLabels:
                     kubernetes.io/metadata.name: kube-system
             ports:
               - protocol: UDP
                 port: 53
   ```

   Run the following command to create an environment template:

   ```shell
   kubectl create -f project-type.yaml
   ```

   Check that the environment template has been created successfully by running the following command:

   ```shell
   kubectl get projecttypes.deckhouse.io
   ```

   The following is an example of the command output:

   ```text
   NAME                READY
   test-project-type   true
   ```

1. Create an environment using the [Project](cr.html#project) CR, specifying the name of the environment template you created earlier in the [.spec.projectTypeName](cr.html#project-v1alpha1-spec-projecttypename) field. Fill in the [.spec.template](cr.html#project-v1alpha1-spec-template) field with values suitable for [.spec.openAPI ProjectType](cr.html#projecttype-v1alpha1-spec-openapi).

   Create a `project.yaml` file with the following contents (CR `Project`):

   ```yaml
   # project.yaml
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: Project
   metadata:
     name: test-project
   spec:
     description: Test case from Deckhouse documentation
     projectTypeName: test-project-type
     template:
       requests:
         cpu: 5
         memory: 5Gi
         storage: 1Gi
       limits:
         cpu: 5
         memory: 5Gi
   ```

   Run the following command to create an environment:

   ```shell
   kubectl create -f project.yaml
   ```

   Check that the environment has been created successfully by running the following command:

   ```shell
   kubectl get projects.deckhouse.io
   ```

   Below is an example of the command output:

   ```shell
   NAME           READY   DESCRIPTION
   test-project   true    Test case from Deckhouse documentation
   ```

1. Check the resources created within the isolated environment.

   Examples of commands and their output:

   ```shell
   $ kubectl get -n test-project namespaces test-project
   NAME           STATUS   AGE
   test-project   Active   5m

   $ kubectl get authorizationrules.deckhouse.io -n test-project
   NAME                                    AGE
   test-project-admin-user-admin-cluster   5m
   test-project-user-user-user-cluster     5m

   $ kubectl get -n test-project resourcequotas
   NAME                    AGE   REQUEST                                                              LIMIT
   test-project-all-pods   5m   requests.cpu: 0/5, requests.memory: 0/5Gi, requests.storage: 0/1Gi   limits.cpu: 0/5, limits.memory: 0/5Gi

   $ kubectl get -n test-project limitranges
   NAME                          CREATED AT
   test-project-all-containers   2023-06-01T14:37:42Z

   $ kubectl get -n test-project networkpolicies.networking.k8s.io
   NAME                                             POD-SELECTOR   AGE
   test-project-deny-all-except-current-namespace   <none>         5m
   ```

1. [Generate a kubeconfig](../150-user-authn/faq.html#how-can-i-generate-a-kubeconfig-and-access-kubernetes-api) to enable created users to access the API server.

1. Check whether the created users can access the API server using the generated kubeconfig.

   Examples of commands and their output:

   ```shell
   $ kubectl get limitranges -n test-project --kubeconfig admin-kubeconfig.yaml
   NAME                          CREATED AT
   test-project-all-containers   2023-06-01T14:37:42Z

   $ kubectl get limitranges -n test-project --kubeconfig user-kubeconfig.yaml
   NAME                          CREATED AT
   test-project-all-containers   2023-06-01T14:37:42Z
   ```

{% endraw %}
