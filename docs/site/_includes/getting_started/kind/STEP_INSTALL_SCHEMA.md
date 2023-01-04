[kind](https://kind.sigs.k8s.io/) is a tool for running local Kubernetes clusters using container “nodes” and  was primarily designed for testing Kubernetes itself, but may be used for local development or CI.

Installing Deckhouse on kind, will allow you to get a Kubernetes cluster with Deckhouse installed in less than 15 minutes. It will allow you to get acquainted with Deckhouse main features quickly.

Deckhouse will be installed in a **minimal** configuration, with Grafana based [monitoring](/documentation/v1/modules/300-prometheus/) enabled. Some features, such as [node management](/documentation/v1/modules/040-node-manager/) and [control plane management](/documentation/v1/modules/040-control-plane-manager/) will not work. To simplify, the [sslip.io](https://sslip.io ) service is used for working with DNS.

> **Note!** Some providers are blocking work sslip.io and similar services. If you encounter such a problem, put the necessary domain names in the `hosts` file locally, or use a real domain and fix [DNS names template](../../documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate).
>
> **Note!** When using kind on Windows, monitoring (Grafana, Prometheus) may not be available or work incorrectly due to the need to use a special kernel for WSL. Read about the solution in [the kind documentation](https://kind.sigs.k8s.io/docs/user/using-wsl2/#kubernetes-service-with-session-affinity).

{% offtopic title="Computer minimal requirements..." %}
- Operating system: macOS, Windows or Linux.
- At least 4GB of RAM.
- Installed container runtime (docker, containerd) and docker client.
- HTTPS access to the `registry.deckhouse.io` container image registry.
{% endofftopic %}

## Installing

A Kubernetes cluster will be deployed and Deckhouse will be installed into a cluster using [the Shell script](https://github.com/deckhouse/deckhouse/blob/main/tools/kind-d8.sh):
- Run the following command for installing Deckhouse **Community Edition**:
  {% snippetcut selector="kind-install" %}
```shell
bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/kind-d8.sh)"
```
  {% endsnippetcut %}
- Or run the following command for installing Deckhouse **Enterprise Edition** by providing a license key:
  {% snippetcut selector="kind-install" %}
```shell
bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/kind-d8.sh)" -- --key <LICENSE_KEY>
```
  {% endsnippetcut %}

After installation is complete, you will get the `admin` user password for accessing Grafana. Grafana will be available at the URL [http://grafana.127.0.0.1.sslip.io](http://grafana.127.0.0.1.sslip.io).

{% offtopic title="Example of the output..." %}
```text
Waiting for the Ingress controller to be ready.........................................
Ingress controller is running.

You have installed Deckhouse Platform in kind!

Don't forget that the default kubectl context has been changed to 'kind-d8'.

Run 'kubectl --context kind-d8 cluster-info' to see cluster info.
Run 'kind delete cluster --name d8' to remove cluster.

Provide following credentials to access Grafana at http://grafana.127.0.0.1.sslip.io/ :

    Username: admin
    Password: LlF7X67BvgRO74LNWXHi

The information above is saved to /home/user/.kind-d8/info.txt file.

Good luck!
```
{% endofftopic %}

The user `admin` password for Grafana can also be found by running the command:
{% snippetcut selector="kind-get-password" %}
```shell
kubectl -n d8-system exec deploy/deckhouse -- sh -c "deckhouse-controller module values prometheus -o json | jq -r '.prometheus.internal.auth.password'"
```
{% endsnippetcut %}
