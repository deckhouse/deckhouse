# Webhook-operator

## Description
**Webhook-operator** is a Kubernetes operator that allows you to define and manage shell-operator webhooks as Kubernetes resources.  
It simplifies webhook creation by providing a declarative interface for describing ValidationWebhook and ConversionWebhook Custom Resources (CRDs).

The operator processes CRD resources (`ValidationWebhook`, `ConversionWebhook`) and dynamically generates corresponding webhooks for shell-operator.  
Validation or conversion logic is defined directly inside the manifest using code.

### Resource Examples
#### ValidationWebhook
Validation of Services: deny creation of objects containing the word `test` in the name.
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ValidationWebhook
metadata:
  name: validationwebhook-sample
validationObject:
  name: service.apps.kubernetes.io
  group: main
  rules:
  - apiGroups:   ["*"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["services"]
    scope:       "*"
context:
  - name: services
    kubernetes:
      apiVersion: v1
      kind: Service
handler:
  python: |
    def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
        resource = ctx.review.request.name
        if "test" in resource:
            return "TEST: service with \"test\" in .metadata.name", False
        return None, True
```

#### ConversionWebhook
Automatic conversion of CRDs between v1alpha1 and v1.
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConversionWebhook
metadata:
  labels:
    app.kubernetes.io/name: operator
    app.kubernetes.io/managed-by: kustomize
  name: example.deckhouse.io
conversions:
  - from: v1alpha1
    to: v1
    handler:
      python: |
        def v1alpha1_to_v1(self, o: dict) -> typing.Tuple[None, dict]:
            obj = DotMap(o)

            obj.apiVersion = "deckhouse.io/v1"

            obj.spec.host=obj.spec.hostPort
            obj.spec.port=obj.spec.hostPort
            del obj.spec.hostPort

            return None, obj.toDict()
  - from: v1
    to: v1alpha1
    handler:
      python: |
        def v1_to_v1alpha1(self, o: dict) -> typing.Tuple[None, dict]:
            obj = DotMap(o)

            obj.apiVersion = "deckhouse.io/v1alpha1"
            if not obj.spec.host:
              return None, obj.toDict()

            hostPort = obj.spec.host+":"+obj.spec.port
            del obj.spec
            if hostPort:
              obj.spec.hostPort=hostPort

            return None, obj.toDict()
```

## Getting Started

### How to setup locally for development
```bash
cd modules/002-deckhouse/images/webhook-handler/operator
minikube start
```

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create sample webhook resources**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## License
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
