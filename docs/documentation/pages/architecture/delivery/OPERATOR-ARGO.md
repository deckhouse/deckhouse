---
title: operator-argo module
permalink: en/architecture/delivery/operator-argo.html
search: operator-argo, GitOps, Argo CD, application deployment
description: Architecture of the operator-argo module in Deckhouse Kubernetes Platform.
---

The [`operator-argo`](/modules/operator-argo/) module deploys [Argo CD Operator](https://github.com/argoproj-labs/argocd-operator) in a Deckhouse Kubernetes Platform (DKP) cluster. The module enables to install Argo CD in a DKP cluster using the ArgoCD resource.

The module works with the following custom resources:

- [Application](/modules/operator-argo/cr.html#application): Describes application deployment and its management.
- [ApplicationSet](/modules/operator-argo/cr.html#applicationset): Provides templating and mass application creation according to defined rules.
- [AppProject](/modules/operator-argo/cr.html#appproject): Defines a set of applications and application access policies.
- [ArgoCD](/modules/operator-argo/cr.html#argocd): Main resource for deploying and configuring an Argo CD instance.
- [ArgoCDExport](/modules/operator-argo/cr.html#argocdexport): Exports Argo CD configuration and state for backup or migration.
- [ImageUpdater](/modules/operator-argo/cr.html#imageupdater): Automatically updates application container images.
- [NamespaceManagement](/modules/operator-argo/cr.html#namespacemanagement): Defines namespace management rules for an Argo CD instance.
- [NotificationsConfiguration](/modules/operator-argo/cr.html#notificationsconfiguration): Defines notification settings for events in Argo CD and applications.

For more details on module settings and usage examples, refer to [the module documentation](/modules/operator-argo/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
* Only the main containers of each component are shown in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`operator-argo`](/modules/operator-argo/) module and its interactions with other DKP components are shown in the following diagrams.

Main module operator:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Operator-argo module operator architecture](../../images/architecture/delivery/c4-l2-operator-argo-operator.svg)

Argo CD instance deployment scenario with Redis in a non-HA configuration:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Operator-argo module architecture with Redis non-HA](../../images/architecture/delivery/c4-l2-operator-argo.svg)

Argo CD instance deployment scenario with Redis in an HA configuration (the diagram shows only differences from the primary deployment scenario):

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Operator-argo module architecture with Redis HA](../../images/architecture/delivery/c4-l2-operator-argo-ha.svg)

Argo CD instance deployment scenario in a [principal cluster](https://argocd-agent.readthedocs.io/stable/concepts/components-terminology/) for a multicluster setup (the diagram shows only differences from the primary deployment scenario):

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Operator-argo module architecture with Principal](../../images/architecture/delivery/c4-l2-operator-argo-principal.svg)

## Module components

The module consists of the following components:

1. **argocd-operator-controller-manager** (Deployment): Implementation of [Argo CD Operator](https://github.com/argoproj-labs/argocd-operator) that allows deploying Argo CD instances in a DKP cluster. The component works with the following custom resources:
   - [ArgoCD](/modules/operator-argo/cr.html#argocd): Main resource for deploying and configuring an Argo CD instance.
   - [ArgoCDExport](/modules/operator-argo/cr.html#argocdexport): Exports Argo CD configuration and state for backup or migration. The operator reads the ArgoCDExport custom resource and creates a Job/CronJob with the same name as the ArgoCDExport resource. The created Job/CronJob performs backup of the Argo CD instance configuration.
   - [NamespaceManagement](/modules/operator-argo/cr.html#namespacemanagement): Defines namespace management rules for an Argo CD instance. The operator watches the NamespaceManagement custom resource and updates the `argocd-cmd-params-cm` ConfigMap accordingly.
   - [NotificationsConfiguration](/modules/operator-argo/cr.html#notificationsconfiguration): Defines notification settings for events in Argo CD and applications. The operator reads NotificationsConfiguration custom resources and updates configuration in the `argocd-notifications-cm` ConfigMap based on them.

   Argocd-operator-controller-manager creates Deployment, Secret, ConfigMap, StatefulSet, and other resources for each ArgoCD custom resource, adding that resource name as a prefix to the created resources.

   It consists of the following containers:

   - **manager**: Main container.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to `manager` metrics.

{% alert level="info" %}
The following components describe resources created by argocd-operator-controller-manager based on configuration defined in the ArgoCD custom resource. The `<ArgoCD name>` prefix is used in descriptions and is replaced by the controller with the ArgoCD resource name.
{% endalert %}

1. **&lt;ArgoCD name&gt;-server** (Deployment): Argocd-server. Main component for interacting with an Argo CD instance. Argocd-server provides REST/gRPC API and a web UI for Argo CD management. The component allows managing Application, ApplicationSet, and AppProject custom resources through the provided interfaces (web UI, API, CLI).

   The operator creates this component if the [`.spec.server.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-enabled) parameter of the ArgoCD custom resource is set to `true` (default value is `true`).

   It consists of the following containers:

   - **argocd-server-init**: Optional set of init containers defined by the user in the [`.spec.server.initContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-initcontainers) parameter of the ArgoCD custom resource.
   - **rollout-extension**: Optional init container that loads the UI extension for the [Rollout custom resource](https://argoproj.github.io/argo-rollouts/features/specification/). The module does not provide a controller for this custom resource. Such controller must be installed and configured separately. `argocd-operator-controller-manager` adds rollout-extension if [`.spec.server.enableRolloutsUI`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-enablerolloutsui) is set to `true`.
   - **argocd-server**: Main container.
   - **argocd-server-sidecar**: Optional set of sidecar containers defined by the user in the [`.spec.server.sidecarContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-sidecarcontainers) parameter of the ArgoCD custom resource.

1. **&lt;ArgoCD name&gt;-repo-server** (Deployment): Argocd-repo-server. Component responsible for template rendering, application manifest generation, and working with external repositories used by Argo CD. Argocd-repo-server synchronizes application manifests from configured repositories and passes them to the corresponding components for further deployment.

   The operator creates this component if the [`.spec.repo.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-repo-enabled) parameter of the ArgoCD custom resource is set to `true` (default value is `true`).

   It consists of the following containers:

   - **copyutil**: Init container that copies executables for use by the main container.
   - **argocd-repo-server-init**: Optional set of init containers configured through the [`.spec.repo.initContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-repo-initcontainers) parameter of the ArgoCD custom resource to prepare the environment.
   - **argocd-repo-server**: Main container that generates and processes manifests and works with remote application Git repositories.
   - **argocd-repo-server-sidecar**: Optional set of sidecar containers defined by the user in the [`.spec.repo.sidecarContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-repo-sidecarcontainers) parameter of the ArgoCD custom resource and used to extend repo-server functionality.

1. **&lt;ArgoCD name&gt;-application-controller** (StatefulSet): Argocd-application-controller. Component responsible for synchronization and state management of applications defined in Argo CD. Argocd-application-controller provides idempotent application of Kubernetes manifests, manages deployment, rollback, and self-healing workflows, and monitors resource state in the cluster.

   The operator creates this component if the [`.spec.controller.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-controller-enabled) parameter of the ArgoCD custom resource is set to `true` (default value is `true`).

   It consists of the following containers:

   - **application-controller-init**: Optional set of init containers configured through the [`.spec.controller.initContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-controller-initcontainers) parameter of the ArgoCD custom resource to prepare the environment.
   - **argocd-application-controller**: Main container implementing synchronization logic for Application custom resources and resources created from them.
   - **application-controller-sidecar**: Optional set of sidecar containers defined by the user in the [`.spec.controller.sidecarContainers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-controller-sidecarcontainers) parameter of the ArgoCD custom resource and used to extend controller capabilities.

1. **&lt;ArgoCD name&gt;-applicationset-controller** (Deployment): Argocd-applicationset-controller. Optional component consisting of a single **applicationset-controller** container and responsible for managing the [ApplicationSet](/modules/operator-argo/cr.html#applicationset) custom resource in Argo CD. It allows automatic creation, update, and deletion of Application resources based on configured templates and generators (for example, Git, List, Matrix, and Cluster generators). This simplifies mass management of similar applications that must be deployed across different environments or clusters.

   The operator creates this component if the [`.spec.applicationSet.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-applicationset-enabled) parameter of the ArgoCD custom resource is set to `true` (default value is `true`).

   For more details about the component, refer to [the applicationset-controller documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/).

1. **&lt;ArgoCD name&gt;-argocd-image-updater-controller** (Deployment): Argocd-image-updater-controller. Optional component consisting of a single **argocd-image-updater** container and intended for automatic container image updates in Argo CD applications when new versions appear in image registries. The component tracks image tag changes and, when a new version is found, updates corresponding Application resources in Argo CD (for example, image tags in manifests or `Helm values`) via a pull request to a Git repository or directly, depending on the selected workflow.

   Argocd-image-updater-controller performs the following functions:

   - manages the [ImageUpdater](/modules/operator-argo/cr.html#imageupdater) custom resource that defines settings for automatic updates of application container images;
   - periodically checks application container images in supported registries (Docker Hub, Quay.io, Harbor, and others);
   - supports filtering image tags by patterns and update strategies (`semver`, `latest`, and others);
   - when a new image version is found, automatically performs write-back (writes the new image tag value) to Argo CD Application or a Git repository depending on the configured method.

   For correct operation, the component requires access to Git repositories and, if needed, private image registries. Credentials for accessing image registries can be stored in Kubernetes Secrets.

   To enable the component, set [`.spec.imageUpdater.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-imageupdater-enabled) to `true` in the ArgoCD custom resource.

   For more details about the component, refer to [the argocd-image-updater documentation](https://argocd-image-updater.readthedocs.io/).

1. **&lt;ArgoCD name&gt;-notifications-controller** (Deployment): Optional controller consisting of a single **argocd-notifications-controller** container that sends notifications about Argo CD events (for example, successful application sync, deployment failures, status changes, and others) to external notification systems, including email, Slack, Microsoft Teams, Telegram, OpsGenie, Webhook, and others.

   The main module operator argocd-operator-controller-manager generates notification settings based on [NotificationsConfiguration](/modules/operator-argo/cr.html#notificationsconfiguration) custom resources and stores them in the `argocd-notifications-cm` ConfigMap and `argocd-notifications-secret` Secret used by the controller to generate and send notifications.

   To enable the component, set [`.spec.notifications.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-notifications-enabled) to `true` in the ArgoCD custom resource.

   For more details on operation, refer to [the Argo CD Notifications documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/notifications/).

1. **&lt;ArgoCD name&gt;-dex-server** (Deployment): Argocd-dex-server. Optional component for user authentication in Argo CD, acting as an OIDC provider (OpenID Connect) based on [Dex](https://github.com/dexidp/dex). The component enables user login through various external authentication providers (LDAP, GitHub, GitLab, SAML, Azure AD, and others) and supports static users defined in Dex configuration.

   It consists of the following containers:

   - **copyutil**: Init container that copies executables for use by the main container.
   - **dex**: Main container.

   To enable the component, define parameters in [`.spec.sso.dex`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-sso-dex) section of the ArgoCD custom resource.

   {% alert level="warning" %}
   For Argo CD user authentication in DKP, the `operator-argo` module supports integration with the [`user-authn`](/modules/user-authn/) module (built-in DKP authentication). Other external providers via Dex are not used in this configuration.

   For more details about module usage examples, refer to [the corresponding documentation section](/modules/operator-argo/examples.html#authentication).
   {% endalert %}

1. **&lt;ArgoCD name&gt;-redis** (Deployment): Argocd-regis. Mandatory component consisting of a single **redis** container and responsible for storing task queue data and session state in Argo CD. Argocd-regis provides a dedicated [Redis](https://github.com/redis/redis) database instance.

   Argocd-operator-controller-manager deploys this component if [`.spec.ha.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-enabled) in the ArgoCD custom resource is `false`.

1. **&lt;ArgoCD name&gt;-redis-ha-server** (StatefulSet): Argocd-redis-ha-server. Mandatory component for deploying Redis in high availability (HA) mode in Argo CD. It provides a fault-tolerant Redis cluster with replication and automatic failover using [Redis Sentinel](https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/).

   It consists of the following containers:

   - **config-init**: Init container that prepares configuration for Redis and Sentinel before main containers start.
   - **redis**: Main container implementing a Redis server instance.
   - **sentinel**: Auxiliary container running [Redis Sentinel](https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/) to monitor Redis instance health and automatically switch to a replica if the primary instance fails.

   Argocd-operator-controller-manager deploys this component if [`.spec.ha.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-enabled) in the ArgoCD custom resource is `true`.

1. **&lt;ArgoCD name&gt;-redis-ha-haproxy** (Deployment): Argocd-redis-ha-proxy. Additional component for load balancing and traffic distribution to Redis cluster instances (`redis-ha-server`).

   It consists of the following containers:

   - **config-init**: Init container that prepares HAProxy configuration before the main container starts.
   - **haproxy**: Container acting as a proxy server and providing transparent routing of client requests to available Redis master/replica instances, as well as automatic switching between them on failover.

   Argocd-operator-controller-manager deploys this component if [`.spec.ha.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-enabled) in the ArgoCD custom resource is `true`.

   {% alert level="info" %}
   The following components are used to implement a secure and scalable multicluster setup in Argo CD:
   - Principal: The central control point. It stores state and distributes tasks.
   - Agent: The executor in each target cluster. It applies manifests and reports back.

   This approach allows you to manage multiple clusters without direct access from the central Argo CD to the Kubernetes API of each cluster, which reduces the number of required open inbound connections, provides isolation, and ensures fault tolerance.
   {% endalert %}

1. **&lt;ArgoCD name&gt;-agent-agent** (Deployment): Argocd-agent-agent. Optional component consisting of a single **&lt;ArgoCD name&gt;-agent-agent** container and responsible for executing operations on managed Kubernetes cluster resources based on requests from Argo CD. The component establishes connection to Argo CD Principal, synchronizes applications, and manages their state based on commands received from Argo CD Principal.

   For details on Argo CD multicluster architecture, refer to [the Argo CD documentation](https://argocd-agent.readthedocs.io/stable/concepts/architecture/#architectural-diagram).

   Argocd-operator-controller-manager deploys this component if [`.spec.argoCDAgent.agent.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-argocdagent-agent-enabled) in the ArgoCD custom resource is `true`. Argo CD Agent and Argo CD Principal cannot be enabled simultaneously in a single ArgoCD resource.

1. **&lt;ArgoCD name&gt;-agent-principal** (Deployment): Argocd-agent-principal. Optional component consisting of a single **&lt;ArgoCD name&gt;-agent-principal** container and enabling Argo CD operation in a [multicluster setup](https://argocd-agent.readthedocs.io/stable/concepts/architecture/#architectural-diagram).

   When this component is enabled, argocd-operator-controller-manager reconfigures all components that use Redis connection to use Redis proxy instead. The Argocd-agent-principal component provides this Redis proxy and routes database requests by analyzing Redis keys: depending on key values, a request is sent either to a local Redis instance or to one of remote Argo CD Agents.

   Argocd-operator-controller-manager deploys this component if [`.spec.argoCDAgent.principal.enabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-argocdagent-principal-enabled) in the ArgoCD custom resource is `true`. Argo CD Agent and Argo CD Principal cannot be enabled simultaneously in a single ArgoCD resource.

1. **&lt;Export name&gt;** (Job/CronJob): Argocd-export. Optional component implemented as Job or CronJob that creates a pod with a single **argocd-export** container. The component creates backup of Argo CD instance configuration and state.

## Module interactions

The module interacts with the following components:

1. **External image registries**: Receives image lists.
1. **External code repositories**:
    - Receives application deployment manifests from repositories.
    - Updates `image` in Helm chart source code.
1. **External Argo CD Principal**:
    - Connects to Argo CD principal cluster.
    - Receives processing requests.
    - Sends back processing results.
1. **Kube-apiserver**:
    - Manages Application, ApplicationSet, AppProject, ArgoCD, ArgoCDExport, ImageUpdater, NamespaceManagement, NotificationsConfiguration custom resources, as well as Secret and ConfigMap.
    - Manages resources created during deployment of user applications described in the Application custom resource.
    - Authorizes requests for metrics retrieval.
1. **[`user-authn`](/modules/user-authn/) module**: Redirects user for authentication.

The following external components interact with the module:

1. **Prometheus-main**: Collects metrics provided by the operator and Argo CD instances.
1. **External Argo CD Agent**:
    - connects to Argo CD principal cluster;
    - receives processing requests;
    - sends back processing results.
