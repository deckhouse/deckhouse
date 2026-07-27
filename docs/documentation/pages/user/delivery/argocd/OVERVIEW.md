---
title: "Application delivery with Argo CD"
permalink: en/user/delivery/argocd/
description: "Application delivery with Argo CD in Deckhouse Kubernetes Platform."
lang: en
search: argocd, application delivery
relatedLinks:
  - title: "Official Argo CD website"
    url: "https://argo-cd.readthedocs.io"
  - title: "Official Argo CD Operator website"
    url: "https://argocd-operator.readthedocs.io"
---

This section describes how to organize application delivery with Argo CD in Deckhouse Kubernetes Platform (DKP).

Argo CD lets you describe applications declaratively and synchronize their state with the contents of a Git repository.
The user specifies the manifest source, target cluster, namespace, and synchronization parameters,
after which Argo CD deploys the application and keeps it in the target state.

In DKP, Argo CD instances are deployed with the [operator-argo](/modules/operator-argo/) module.
Typical user work with Argo CD includes:

- creating or using an existing [AppProject](/modules/operator-argo/cr.html#appproject) object;
- preparing the target namespace for the application;
- creating an [Application](/modules/operator-argo/cr.html#application) object that describes the application source and synchronization rules;
- creating an application through the Argo CD web interface;
- creating an application with the `argocd` CLI utility.

## Prerequisites

Before you start, the following conditions must be met:

- the cluster administrator has enabled the [operator-argo](/modules/operator-argo/) module;
- the administrator has deployed at least one Argo CD instance;
- the user has access to the required Argo CD instance and target namespaces.

If an Argo CD instance is not deployed yet, contact the administrator or follow the instructions in the [Running Argo CD](/products/kubernetes-platform/documentation/v1/admin/configuration/delivery/argocd/setup/) section.

## AppProject projects

[AppProject](/modules/operator-argo/cr.html#appproject) is an Argo CD custom resource that defines the logical boundaries of a project. With it, you can define:

- which Git repositories are allowed as application sources;
- which clusters and namespaces applications may be deployed to;
- which cluster-wide and namespaced resources are allowed;
- which roles and access policies apply within the project.

Every [Application](/modules/operator-argo/cr.html#application) object must reference a project through the [`spec.project`](/modules/operator-argo/cr.html#application-v1alpha1-spec-project) parameter.

By default, Argo CD includes the `default` project. You can use it for first experiments and simple scenarios.
For production environments, create separate AppProject objects to restrict access to repositories, clusters, and namespaces.

The manifest of the `default` AppProject object is shown below:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: default
  namespace: argocd
spec:
  clusterResourceWhitelist:
  - group: '*'
    kind: '*'
  destinations:
  - namespace: '*'
    server: '*'
  sourceRepos:
  - '*'
```

## Preparing a namespace

Before deploying an application, create the target namespace and add the `argocd.argoproj.io/managed-by` label
that indicates which Argo CD instance manages resources in this namespace.

Example:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
  labels:
    argocd.argoproj.io/managed-by: argocd
```

In this example, the `demo` namespace will be managed by the Argo CD instance deployed in the `argocd` namespace.

## Deploying an application

You can create an application in Argo CD in several ways:

- declaratively — with an [Application](/modules/operator-argo/cr.html#application) object;
- interactively — through the Argo CD web interface;
- with the `argocd` CLI utility.

### Creating an application with an Application object

An [Application](/modules/operator-argo/cr.html#application) object is used to describe an application. It specifies:

- the Argo CD project ([`spec.project`](/modules/operator-argo/cr.html#application-v1alpha1-spec-project));
- the manifest or chart source ([`spec.source`](/modules/operator-argo/cr.html#application-v1alpha1-spec-source));
- the target cluster and namespace ([`spec.destination`](/modules/operator-argo/cr.html#application-v1alpha1-spec-destination));
- the synchronization policy ([`spec.syncPolicy`](/modules/operator-argo/cr.html#application-v1alpha1-spec-syncpolicy)).

Example of an Application object:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: demo
  namespace: argocd
spec:
  destination:
    namespace: demo
    server: https://kubernetes.default.svc
  project: default
  source:
    path: helm-guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps
    targetRevision: HEAD
  syncPolicy:
    # Enable automatic synchronization.
    automated:
      # Delete outdated resources.
      prune: true
      # Enable self-healing if third-party changes occur.
      selfHeal: true
```

After you create the Application object, Argo CD starts tracking the application state and synchronizing it with the repository contents.
As a result, resources related to the `demo` application should appear in the `demo` namespace:

```console
d8 k -n demo get deployment,svc,pod
NAME                                  READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/demo-helm-guestbook   1/1     1            1           15s

NAME                          TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
service/demo-helm-guestbook   ClusterIP   10.222.177.84   <none>        80/TCP    15s

NAME                                       READY   STATUS    RESTARTS   AGE
pod/demo-helm-guestbook-66d5d69ccd-xfkb7   1/1     Running   0          15s
```

### Creating an application through the Argo CD web interface

You can create an application through the Argo CD web interface. To do this:

1. Open the web interface of the required Argo CD instance.
1. Go to the "Applications" section.
1. Click "New App".
1. Specify the application name, project, repository, revision, path to manifests or chart, and the target cluster and namespace.
1. If needed, configure automatic synchronization and additional parameters.
1. Click "Create".

The form fields in the web interface correspond to the main parameters of the Application object:
project, application source, target cluster, namespace, and synchronization policy.

### Creating an application with the argocd CLI utility

The `argocd` CLI utility lets you create and maintain applications from the command line.
You can download the `argocd` binary from the "Documentation" section of the Argo CD web interface.

Before creating an application, authenticate with the following command:

```bash
argocd login <ARGOCD_DOMAIN>:443
```

{% alert level="info" %}
When using SSO authentication, add the `--sso` flag to the login command.
{% endalert %}

Example of creating an application:

```bash
argocd app create guestbook \
  --repo https://github.com/argoproj/argocd-example-apps.git \
  --path guestbook \
  --dest-namespace demo \
  --dest-server https://kubernetes.default.svc \
  --directory-recurse \
  --sync-policy automated \
  --self-heal \
  --auto-prune
```

As with creating an Application object, the command specifies the application source, target cluster, namespace, and synchronization policy.

To view the status of a deployed application, use the `argocd app get <APP_NAME>` command, for example:

```console
argocd app get guestbook
Name:               argocd/guestbook
Project:            default
Server:             https://kubernetes.default.svc
Namespace:          demo
URL:                https://argocd.192.168.0.235.sslip.io/applications/guestbook
Source:
- Repo:             https://github.com/argoproj/argocd-example-apps.git
  Target:
  Path:             guestbook
SyncWindow:         Sync Allowed
Sync Policy:        Automated (Prune)
Sync Status:        Synced to  (8088f4c)
Health Status:      Healthy

GROUP  KIND        NAMESPACE  NAME          STATUS  HEALTH   HOOK  MESSAGE
apps   Deployment  demo       guestbook-ui  Synced  Healthy        deployment.apps/guestbook-ui unchanged
       Service     demo       guestbook-ui  Synced  Healthy
```
