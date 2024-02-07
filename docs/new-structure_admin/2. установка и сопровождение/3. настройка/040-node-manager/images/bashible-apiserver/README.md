# Bashible apiserver

## What is it for

Bashible apiserver serves the bashible script and its step scripts through Kubernetes API.

## Usage

Call bashible script or a bundle for a pair of OS and node group:

```shell
kubectl get -o json  bootstrap         ubuntu-lts.master    # <os>.<nodegroup>
kubectl get -o json  bashibles         ubuntu-lts.master    # <os>.<nodegroup>
kubectl get -o json  nodegroupbundles  ubuntu-lts.master    # <os>.<nodegroup>
```

or

```
GET /apis/bashible.deckhouse.io/v1alpha1/bootstrap/ubuntu-lts.master
GET /apis/bashible.deckhouse.io/v1alpha1/bashibles/ubuntu-lts.master
GET /apis/bashible.deckhouse.io/v1alpha1/nodegroupbundles/ubuntu-lts.master
```

Example:

```shell
kubectl get -o json bashibles ubuntu-lts.master
{
  "kind": "Bashible",
  "metadata": {
    "creationTimestamp": "2021-02-08T07:59:25Z",
    "name": "ubuntu-lts.master"
  },
  "data": {
    "bashible.sh": "#!/usr/bin/env bash\n\nset -Eeo pipefail\n\nfunction kubectl_exec() {\n ..."
  }
}
```

## How it works

Bashible apiserver generates bash scripts on the fly for a requested bundle. Templates of bashible steps are located in
the container of the server. The context for these templates is mounted from `bashible-apiserver-context` secret. All
objects returned by the apiserver contain a map where keys are script file names, and values are rendered bash scripts.

`metadata.creationTimestamp` is generated in the request time.

## Motivation

The number of combinations of bashible steps is the product of four factors:

* \# of supported Kubernetes versions
* \# of supported Linux distributions on nodes
* \# of steps in a bundle for a specific Linux distribution
* \# of node groups

All the scripts cannot be stored pre-rendered because of Etcd limitations. Helm release secret would have to keep them
all at once. Generating steps with an extension apiserver scales better.

## Code

The code is based on [sample-apiserver](https://https://github.com/kubernetes/sample-apiserver).

Code generation is provided by the `code-generator` package. The generated code is commited to the repo. To re-generate
code, run in the project root

```shell
./hack/update-codegen.sh
```

### Templates

Templates of bashible and bundle steps are added to the container from `candi/bashible` on building.

The file structure in the container:

```shell
tree /bashible
/bashible
├── context.yaml   # mounted from `bashible-apiserver-context` secret
└── templates      # added on container building
    ├── bashible
    │   ├── bashible.sh.tpl
    │   ├── bundles
    │   │   ├── <os>
    │   │   │   ├── all
    │   │   │   │   └── <>.sh.tpl
    │   │   │   └── node-group
    │   │   │       └── <>.sh.tpl
    │   └── common-steps
    │       ├── all
    │       │   └── <>.sh.tpl
    │       └── node-group
    │           └── <>.sh.tpl
    └── cloud-providers
        └── <provider>
            └── bashible
                ├── bundles
                │   └── <os>
                │       └── ...
                └── common-steps
                    └── ...
```
