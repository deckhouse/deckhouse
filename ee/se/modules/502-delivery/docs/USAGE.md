---
title: "The delivery module: usage"
---

## Before you begin

This section describes how Argo CD works when bundled with Deckhouse and assumes you have basic understanding or prior knowledge of Argo CD.

The following details are used in the examples below:
- The `argocd` domain has been allocated according to the name template defined in the [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) parameter. It allows you to access the Argo CD web interface and the API. The examples below use `argocd.example.com`.
- `myapp` is the name of the application.
- `mychart` is the name of the Helm chart and werf bundle. They must match each other in the scheme below. For the sake of clarity, the name of the Helm chart and the werf bundle is different from the app name.
- `cr.example.com` is the OCI registry's hostname.
- `cr.example.com/myproject` is the bundle repository in the OCI registry.

## The concept

The module implements a method of deploying applications using a combination of [werf bundles](https://werf.io/documentation/v1.2/advanced/bundles.html#deploy-with-werf-bundle-apply) and [OCI-based registries](https://helm.sh/docs/topics/registries/).

The advantage of this approach is that there is a single place to deliver the artifact — the container registry. The artifact contains both container images and the Helm chart. It is used both for the initial deployment of the application and for pull model auto-updates.

The following components are used:

- Argo CD;
- Argo CD Image Updater with [OCI repository support patch](https://github.com/argoproj-labs/argocd-image-updater/pull/405);
- werf-argocd-cmp-sidecar to keep werf annotations during manifest rendering.

To use the OCI registry as a repository, you have to enable the `enableOCI=true` flag in the Argo CD repository settings. The `delivery` module does it automatically.

Argo CD Image Updater automatically updates applications in the cluster once the artifact has been delivered. Argo CD Image Updater has been [modified](https://github.com/argoproj-labs/argocd-image-updater/pull/405) to work with werf bundles.

The examples below use the [«Application of
Applications»](https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/#app-of-apps-pattern) pattern, which implies that there are two git repositories — a dedicated repository for the application, and a dedicated repository for the infrastructure:
![flow](../../images/502-delivery/werf-bundle-and-argocd.svg)

The use of the Application of Applications pattern and a separate infrastructure repository is not necessary if you are allowed to create Application resources manually. For the sake of simplicity, in the examples below we will manage Application resources manually.

## WerfSource CRD-based configuration

All you need to do to use Argo CD and Argo CD Image Updater is to configure the Application object and access to the registry. Access to the registry is required for the Argo CD repository and Argo CD Image Updater. Thus, you have to configure:

1. Secret to access the registry.
2. Application object containing the application configuration.
3. Registry for Image Updater in its configMap (there will be a link to the registry Secret listed above as the first item).
4. Secret for the Argo CD repository (it will contain a copy of the access credentials from the Secret listed as the first item).

The `delivery` module makes it easy to configure Deckhouse to work with werf bundles and Argo CD. Specifically, it streamlines the configuration of the Argo CD repository and the configuration of the Image Updater registry. All you have to do is define a single resource, the *WerfBundle*. Therefore, you have to configure three objects in the module:

1. Secret in `dockerconfigjson` format to access the registry.
2. Application object containing the application configuration.
3. WerfSource object, which contains information about the registry and a link to the Secret (the one listed as the first item) used for access.

Thus, three objects need to be created for deploying from the OCI repository. Note that all namespaced objects must be created in the `d8-delivery` namespace.

Example:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: WerfSource
metadata:
  name: example
spec:
  imageRepo: cr.example.io/myproject  # bundle and image repository
  pullSecretName: example-registry    # Secret with the credentials required for access
---
apiVersion: v1
kind: Secret
metadata:
  namespace: d8-delivery              # namespace of the module
  name: example-registry
type: kubernetes.io/dockerconfigjson  # only this Secret type is supported
data:
  .dockerconfigjson: ...
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 0.0
  name: myapp
  namespace: d8-delivery  # namespace of the module
spec:
  destination:
    namespace: myapp
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: mychart                    # bundle — cr.example.com/myproject/mychart
    helm: {}
    repoURL: cr.example.com/myproject # Argo CD repository from WerfBundle
    targetRevision: 1.0.0
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```

## Publishing an artifact to the registry

The Helm OCI chart requires that the chart name in `Chart.yaml` be the same as the last element in the path to the OCI registry. This is why you have to include the chart name in the bundle name:

```sh
werf bundle publish --repo cr.example.com/myproject/mychart --tag 1.0.0
```

For more information about bundles, see the [werf documentation](https://werf.io/documentation/v1.2/advanced/ci_cd/werf_with_argocd/configure_ci_cd.html).

## Bundle auto-updating

As part of the pull model, Argo CD Image Updater automatically updates the Application using a published werf bundle. Image Updater scans the OCI repository at set intervals and updates `targetRevision` in the Application. This causes the entire application to be updated based on the updated artifact. We use a [modified Image Updater](https://github.com/argoproj-labs/argocd-image-updater/pull/405) that supports OCI registries and werf bundles.

### The rules for updating images

You have to add an annotation to the Application object containing the rules governing image updates (see the [werf documentation](https://werf.io/documentation/v1.2/advanced/ci_cd/werf_with_argocd/configure_ci_cd.html#continuous-deployment) for details).

Here is an example of a rule that updates a patch version of an application (`1.0.*`):

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
```

### Access settings for the registry

The `argocd-image-updater` service account can only work with resources in the `d8-delivery` namespace. Therefore, you must create a Secret in this exact namespace containing parameters to access the registry referenced by the `credentials` field.

#### Per-Application configuration

You can refer access credentials individually in each Application using the `argocd-image-updater.argoproj.io/pull-secret` annotation.

Example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
    argocd-image-updater.argoproj.io/pull-secret: pullsecret:d8-delivery/example-registry
```

## How to authenticate using the `argocd` CLI

### Argo CD user

Set `username` and `password` in the Argo CD configuration or use the `admin` user. The admin user is disabled by default, so you have to enable it first.

To do so:

1. Open the `delivery` module config:

   ```sh
   kubectl edit mc delivery
   ```

1. Set `spec.settings.argocd.admin.enabled` to `true`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: delivery
     # (...)
   spec:
     enabled: true
     settings:
       argocd:
         admin:
           enabled: true
     version: 1
   ```

### kubectl

If [external access to Kubernetes API](../../modules/150-user-authn/configuration.html#parameters-publishapi) is configured, `argocd` can use requests to kube-apiserver:

```sh
argocd login argocd.example.com --core
```

The `argocd` CLI [does not allow namespace](https://github.com/argoproj/argo-cd/issues/9123) to be set at invocation time and relies on the value defined in `kubectl`. The `delivery` module is in the `d8-delivery` namespace, so you must set the `d8-delivery` namespace to be the default while working with argocd.

Use the following command to set `d8-delivery` as the default namespace:

```sh
kubectl config set-context --current --namespace=d8-delivery
```

### Dex

The Dex-based authorization **does not work for the CLI**, but it does in the web interface.

That is, you **cannot** authorize via SSO because Dex Client in Deckhouse does not support public
clients, in this case Argo CD:

```sh
argocd login argocd.example.com --sso
```

## Partial WerfSource CRD usage scenarios

### No Argo CD repository

WerfSource creates a repository on the Argo CD and adds registry information to the Image Updater configuration. However, it also allows you to skip the repository creation. To do so, set the `spec.argocdRepoEnabled` parameter to `false` (`spec.argocdRepoEnabled=false`). This comes in handy when using a repository type other than OCI, e.g., Chart Museum or git:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: WerfSource
metadata:
  name: example
spec:
  # ...
  argocdRepoEnabled: false
```

#### How to manually create an Argo CD repository for the OCI registry

The registry acts as a repository for the bundles. To use it, you will have to enable OCI mode in the repository. Unfortunately, the web interface does not allow you to set the `enableOCI` flag, so you must add it by other means.

##### Using the Argo CD CLI

The `argocd` CLI tool supports the `--enable-oci` flag:

```sh
$ argocd repo add cr.example.com/myproject \
  --enable-oci \
  --type helm \
  --name REPO_NAME \
  --username USERNAME \
  --password PASSWORD
```

##### Using the web interface and kubectl

The missing flag can be added manually to an existing repository:

```sh
kubectl -n d8-delivery edit secret repo-....
```

```yaml
apiVersion: v1
kind: Secret
stringData:           # <----- add
  enableOCI: "true"   # <----- and save
data:
  # (...)
metadata:
  # (...)
  name: repo-....
  namespace: d8-delivery
type: Opaque
```

### No registry for Image Updater

The registries in `configmap/argocd-image-updater-config` can only be configured via WerfSource because Deckhouse generates this ConfigMap using the WerfSource objects. If this approach is not suitable for some reason, Image Updater can be configured using annotations in each Application individually:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
    argocd-image-updater.argoproj.io/pull-secret: pullsecret:d8-delivery/example-registry
```
