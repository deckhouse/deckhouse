You can verify the status of the Kubernetes cluster right after (or even during) the Deckhouse Platform installation.
{%- if include.mode == "baremetal" or include.mode == "cloud" %} By default, the `.kube/config` file used to communicate with Kubernetes is generated on the cluster's host. Thus, you can connect to the host via SSH and use regular k8s tools (such as `kubectl`) to interact with Kubernetes.
{%- endif %}

For example, you can use the following command to view the cluster status:

```shell
kubectl -n d8-system get deployments/deckhouse
```

In the command's output, the `deckhouse` deployment should be `READY 1/1`. Such status indicates that modules are installed successfully, and the cluster is ready for use.

For more convenient control over the cluster, a [module](/en/documentation/v1/modules/500-dashboard/) with the official Kubernetes dashboard is provided. It gets enabled by default after installation is complete and is available at `https://dashboard<your-publicDomainTemplate-value>` with the *User* access level. (The [user-authz module](/en/documentation/v1/modules/140-user-authz/) documentation provides a detailed overview of access levels.)

Logs are stored in JSON format, so you might want to use the [jq](https://stedolan.github.io/jq/download/) utility to view them. Here are a few options that may be useful to you:

- A short command for viewing events:
  ```yaml
  kubectl logs -n d8-system deployments/deckhouse -f --tail=10 | jq -rc .msg
  ```
- Viewing events with colored output, including the time and the name of the module::
  ```shell
kubectl -n d8-system logs deploy/deckhouse -f | jq -r 'select(.module != null) | .color |= (if .level == "error" then 1 else 4 end) |
"\(.time) \u001B[1;3\(.color)m[\(.level)]\u001B[0m\u001B[1;35m[\(.module)]\u001B[0m - \u001B[1;33m\(.msg)\u001B[0m"'
```
- Viewing events of a specific module. Example for the `node-manager` module:
  ```shell
kubectl -n d8-system logs deploy/deckhouse -f | jq -r --arg mod node-manager 'select(.module == $mod) |
"\(.time) [\(.level)][\(.module)] - \(.binding) - \(.msg)"'
```

There is also a pack of [special modules based on Prometheus](/en/documentation/v1/modules/300-prometheus/) to implement full-fledged and detailed monitoring of the cluster.
