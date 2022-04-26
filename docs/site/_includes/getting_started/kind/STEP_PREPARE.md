Install [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation), if it is not already installed.

> This guide provides a minimal `kind` cluster configuration required for a Deckhouse installation. The configuration includes installing a single-node Kubernetes cluster and exporting two ports required for a cluster Ingress controller. You can use your cluster configuration in `kind`, [increase](https://kind.sigs.k8s.io/docs/user/configuration/#nodes) the number of nodes or [configure](https://kind.sigs.k8s.io/docs/user/local-registry/) a local container registry. 

Create a cluster configuration file for kind:
{% snippetcut selector="create-kind-cfg" %}
```shell
cat <<EOF > kind.cfg
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    listenAddress: "127.0.0.1"
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    listenAddress: "127.0.0.1"
    protocol: TCP
EOF
```
{% endsnippetcut %}

> Before creating a cluster, make sure that ports 80 and 443 are not used.

Create cluster with kind:
{% snippetcut selector="create-kind-cluster" %}
```shell
kind create cluster --image "kindest/node:v1.22.7@sha256:c195c17f2a9f6ad5bbddc9eb8bad68fa21709162aabf2b84e4a3896db05c0808" --config kind.cfg
```
{% endsnippetcut %}

Example of creation command output:
```shell
$ kind create cluster --image "kindest/node:v1.22.7@sha256:c195c17f2a9f6ad5bbddc9eb8bad68fa21709162aabf2b84e4a3896db05c0808" --config kind.cfg
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.22.7) 🖼
 ✓ Preparing nodes 📦  
 ✓ Writing configuration 📜 
 ✓ Starting control-plane 🕹️ 
 ✓ Installing CNI 🔌 
 ✓ Installing StorageClass 💾 
Set kubectl context to "kind-kind"
You can now use your cluster with:

kubectl cluster-info --context kind-kind

Have a nice day! 👋
```
